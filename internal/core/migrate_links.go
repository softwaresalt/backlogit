package core

import (
	"context"
	"errors"
	"fmt"

	"github.com/backlogit/backlogit/internal/db"
	blerrors "github.com/backlogit/backlogit/internal/errors"
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
					if addErr := db.AddLink(ctx, ws.DB, artifact.ID, targetID, "informs"); addErr != nil {
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
