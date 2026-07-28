package core

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/softwaresalt/backlogit/internal/db"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/models"
)

// MigrateCustomFieldLinksResult summarises the outcome of a custom-field
// link migration run.
type MigrateCustomFieldLinksResult struct {
	// Scanned is the number of artifact files examined.
	Scanned int `json:"scanned"`
	// Migrated is the number of item_links rows inserted.
	Migrated int `json:"migrated"`
	// Skipped is the number of fields whose target was not a resolvable item ID.
	Skipped int `json:"skipped"`
}

// MigrateCustomFieldLinks scans all artifacts in the workspace and converts
// known relationship custom_fields into item_links rows:
//
//   - linked_deliberation_id → link_type "informs" (skipped when target is not an item)
//   - linked_stash_id        → always skipped (stash IDs are not item IDs)
//   - source_stash_id        → always skipped (stash IDs are not item IDs)
//
// Original custom_fields values are preserved as read-only provenance and are
// not removed by this migration. AddLink uses INSERT OR IGNORE so re-running
// is idempotent and does not create duplicate rows.
func MigrateCustomFieldLinks(ctx context.Context, ws *Workspace) (*MigrateCustomFieldLinksResult, error) {
	result := &MigrateCustomFieldLinksResult{}

	artifacts, err := db.QueryItems(ctx, ws.DB, db.QueryFilters{IncludeArchived: true})
	if err != nil {
		return nil, fmt.Errorf("migrate custom field links: query items: %w", err)
	}

	for _, artifact := range artifacts {
		result.Scanned++
		if artifact.CustomFields == nil {
			continue
		}

		// linked_deliberation_id → "informs" link when target is a real item.
		if rawID, ok := artifact.CustomFields["linked_deliberation_id"]; ok {
			targetID := fmt.Sprintf("%v", rawID)
			if targetID != "" && targetID != "<nil>" {
				if _, getErr := db.GetItem(ctx, ws.DB, targetID); getErr == nil {
					if addErr := AddArtifactLink(ctx, ws, artifact.ID, targetID, "informs"); addErr != nil {
						return nil, fmt.Errorf("migrate link %s→%s: %w", artifact.ID, targetID, addErr)
					}
					result.Migrated++
				} else if errors.Is(getErr, blerrors.ErrNotFound) {
					result.Skipped++
				} else {
					return nil, fmt.Errorf("resolve linked_deliberation_id %q: %w", targetID, getErr)
				}
			}
		}

		// linked_stash_id and source_stash_id are always skipped; stash IDs
		// are not artifact IDs and cannot be resolved against the items table.
		if _, ok := artifact.CustomFields["linked_stash_id"]; ok {
			result.Skipped++
		}
		if _, ok := artifact.CustomFields["source_stash_id"]; ok {
			result.Skipped++
		}
	}

	return result, nil
}

// MigrateDBOnlyLinksResult summarises the outcome of a DB-only links migration run.
type MigrateDBOnlyLinksResult struct {
	// Checked is the number of (source, target, type) triples examined.
	Checked int `json:"checked"`
	// Written is the number of link entries written to Markdown frontmatter.
	Written int `json:"written"`
	// Skipped is the number of links skipped because the source file was not
	// found or could not be written.
	Skipped int `json:"skipped"`
}

// MigrateDBOnlyLinks reads every row in item_links and writes any link that
// is not already reflected in the source artifact's Markdown frontmatter.
// This is a startup migration guard that must be called BEFORE the first
// db.Rehydrate invocation that clears item_links, ensuring that no links
// accumulated in the old SQLite-only model are silently dropped during the
// transition to Markdown-first link storage.
//
// The function is idempotent and best-effort: source artifacts that cannot be
// located on disk are logged at Debug level and counted as skipped. Links
// already present in Markdown frontmatter are never duplicated.
func MigrateDBOnlyLinks(ctx context.Context, ws *Workspace) (*MigrateDBOnlyLinksResult, error) {
	result := &MigrateDBOnlyLinksResult{}

	rows, err := ws.DB.QueryContext(ctx, `SELECT source_id, target_id, link_type FROM item_links`)
	if err != nil {
		return nil, fmt.Errorf("migrate db-only links: query item_links: %w", err)
	}
	defer rows.Close()

	type dbLink struct {
		targetID string
		linkType string
	}
	bySource := make(map[string][]dbLink)
	for rows.Next() {
		var sourceID, targetID, linkType string
		if scanErr := rows.Scan(&sourceID, &targetID, &linkType); scanErr != nil {
			return nil, fmt.Errorf("migrate db-only links: scan row: %w", scanErr)
		}
		bySource[sourceID] = append(bySource[sourceID], dbLink{targetID, linkType})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("migrate db-only links: iterate rows: %w", err)
	}

	for sourceID, dbLinks := range bySource {
		artifact, findErr := findArtifact(ctx, ws, sourceID)
		if findErr != nil {
			slog.DebugContext(ctx, "migrate db-only links: source artifact not on disk; skipping",
				"source_id", sourceID)
			result.Skipped += len(dbLinks)
			continue
		}

		// Build a lookup set from links already present in Markdown frontmatter.
		inFrontmatter := make(map[string]bool, len(artifact.Links))
		for _, l := range artifact.Links {
			inFrontmatter[l.TargetID+"\x00"+l.LinkType] = true
		}

		var toWrite bool
		for _, link := range dbLinks {
			result.Checked++
			key := link.targetID + "\x00" + link.linkType
			if inFrontmatter[key] {
				continue
			}
			artifact.Links = append(artifact.Links, models.ArtifactLink{
				TargetID: link.targetID,
				LinkType: link.linkType,
			})
			inFrontmatter[key] = true
			toWrite = true
			result.Written++
		}
		if !toWrite {
			continue
		}

		filePath, pathErr := FindArtifactPath(ctx, ws, sourceID)
		if pathErr != nil {
			slog.WarnContext(ctx, "migrate db-only links: cannot locate artifact file; skipping",
				"source_id", sourceID, "error", pathErr)
			result.Skipped++
			continue
		}
		artifact.UpdatedAt = models.NowUTC()
		if writeErr := WriteArtifactFileWithOptions(artifact, filePath, WorkspaceDurableWrites(ws)); writeErr != nil {
			slog.WarnContext(ctx, "migrate db-only links: file write failed; skipping",
				"source_id", sourceID, "error", writeErr)
			result.Skipped++
		}
	}
	return result, nil
}
