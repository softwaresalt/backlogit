package core

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	bldb "github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/models"
)

// crossRefUpdate carries the data needed to atomically rewrite one referencing
// artifact's Markdown file and corresponding DB rows inside the adoption
// transaction. snapshotRaw holds the original file bytes for rollback.
type crossRefUpdate struct {
	artifact    *models.Artifact
	filePath    string
	snapshotRaw []byte
}

// findCrossArtifactReferences walks all artifact Markdown files in the
// workspace, collecting a crossRefUpdate for every artifact whose frontmatter
// references oldID in parent_id, dependencies, or links. The adopted artifact
// itself (identified by oldID and by newID when they differ) is excluded from
// results. Each returned Artifact struct already has oldID replaced by newID so
// callers can pass the slice directly to applyCrossArtifactRewrites.
//
// This function performs no writes and opens no transactions.
func findCrossArtifactReferences(
	_ context.Context,
	ws *Workspace,
	oldID, newID string,
) ([]crossRefUpdate, error) {
	if oldID == newID {
		return nil, nil
	}

	searchDirs, err := artifactSearchDirs(ws)
	if err != nil {
		return nil, fmt.Errorf("find cross-artifact references: %w", err)
	}

	var updates []crossRefUpdate

	for _, dir := range searchDirs {
		if _, statErr := os.Stat(dir); os.IsNotExist(statErr) {
			continue
		}
		walkErr := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || filepath.Ext(path) != ".md" {
				return err
			}

			raw, readErr := os.ReadFile(path)
			if readErr != nil {
				slog.Debug("find cross-refs: skipping unreadable file", "path", path, "error", readErr)
				return nil
			}

			a, _, parseErr := parseFile(path)
			if parseErr != nil {
				slog.Debug("find cross-refs: skipping unparseable file", "path", path, "error", parseErr)
				return nil
			}

			// Exclude the adopted artifact itself (old and new IDs).
			if a.ID == oldID || a.ID == newID {
				return nil
			}

			refersParent := a.ParentID == oldID
			var refersAnyDep bool
			for _, dep := range a.Dependencies {
				if dep == oldID {
					refersAnyDep = true
					break
				}
			}
			var refersAnyLink bool
			for _, link := range a.Links {
				if link.TargetID == oldID {
					refersAnyLink = true
					break
				}
			}

			if !refersParent && !refersAnyDep && !refersAnyLink {
				return nil
			}

			// Deep-copy and rewrite.
			updated := *a
			updated.UpdatedAt = time.Now()

			if refersParent {
				updated.ParentID = newID
			}

			if refersAnyDep {
				newDeps := make([]string, len(a.Dependencies))
				for i, dep := range a.Dependencies {
					if dep == oldID {
						newDeps[i] = newID
					} else {
						newDeps[i] = dep
					}
				}
				updated.Dependencies = newDeps
			}

			if refersAnyLink {
				newLinks := make([]models.ArtifactLink, len(a.Links))
				for i, link := range a.Links {
					if link.TargetID == oldID {
						newLinks[i] = models.ArtifactLink{TargetID: newID, LinkType: link.LinkType}
					} else {
						newLinks[i] = link
					}
				}
				updated.Links = newLinks
			}

			updates = append(updates, crossRefUpdate{
				artifact:    &updated,
				filePath:    path,
				snapshotRaw: raw,
			})
			return nil
		})
		if walkErr != nil {
			return nil, fmt.Errorf("find cross-artifact references walk %s: %w", dir, walkErr)
		}
	}

	return updates, nil
}

// applyCrossArtifactRewrites applies a batch of cross-artifact reference
// updates inside the provided transaction. For each update it:
//
//  1. Writes the updated Markdown file via atomic tmp+rename.
//  2. Upserts the artifact's items row via UpsertItemTx.
//  3. Deletes then reinserts the artifact's item_deps and item_links rows so
//     they reflect the updated in-memory state.
//
// On any write failure the function restores all previously written files from
// their snapshotRaw bytes before returning the error. The caller remains
// responsible for rolling back or committing the transaction.
func applyCrossArtifactRewrites(
	ctx context.Context,
	tx *sql.Tx,
	_ *Workspace,
	updates []crossRefUpdate,
) error {
	if len(updates) == 0 {
		return nil
	}

	written := make([]crossRefUpdate, 0, len(updates))

	restoreWritten := func() {
		for _, u := range written {
			tmp := u.filePath + ".restore-tmp"
			if writeErr := os.WriteFile(tmp, u.snapshotRaw, 0o644); writeErr != nil {
				slog.Warn("apply cross-refs: failed to write restore tmp",
					"path", u.filePath, "error", writeErr)
				continue
			}
			if renameErr := os.Rename(tmp, u.filePath); renameErr != nil {
				slog.Warn("apply cross-refs: failed to restore file",
					"path", u.filePath, "error", renameErr)
				os.Remove(tmp) //nolint:errcheck
			}
		}
	}

	for _, u := range updates {
		if writeErr := WriteArtifactFile(u.artifact, u.filePath); writeErr != nil {
			restoreWritten()
			return fmt.Errorf("apply cross-artifact rewrite for %s: %w", u.artifact.ID, writeErr)
		}
		written = append(written, u)

		if upsertErr := bldb.UpsertItemTx(ctx, tx, u.artifact); upsertErr != nil {
			restoreWritten()
			return fmt.Errorf("apply cross-artifact rewrite for %s: upsert item: %w", u.artifact.ID, upsertErr)
		}

		// Refresh dep rows: delete existing, reinsert from updated struct.
		if _, delErr := tx.ExecContext(ctx,
			`DELETE FROM item_deps WHERE item_id = ?`, u.artifact.ID,
		); delErr != nil {
			restoreWritten()
			return fmt.Errorf("apply cross-artifact rewrite for %s: delete deps: %w", u.artifact.ID, delErr)
		}
		for _, dep := range u.artifact.Dependencies {
			if strings.TrimSpace(dep) == "" {
				continue
			}
			if _, insErr := tx.ExecContext(ctx,
				`INSERT OR REPLACE INTO item_deps (item_id, depends_on, dep_type) VALUES (?, ?, 'blocks')`,
				u.artifact.ID, dep,
			); insErr != nil {
				restoreWritten()
				return fmt.Errorf("apply cross-artifact rewrite for %s: insert dep %s: %w",
					u.artifact.ID, dep, insErr)
			}
		}

		// Refresh link rows: delete existing, reinsert from updated struct.
		if _, delErr := tx.ExecContext(ctx,
			`DELETE FROM item_links WHERE source_id = ?`, u.artifact.ID,
		); delErr != nil {
			restoreWritten()
			return fmt.Errorf("apply cross-artifact rewrite for %s: delete links: %w", u.artifact.ID, delErr)
		}
		for _, link := range u.artifact.Links {
			if strings.TrimSpace(link.TargetID) == "" || strings.TrimSpace(link.LinkType) == "" {
				continue
			}
			if _, insErr := tx.ExecContext(ctx,
				`INSERT OR IGNORE INTO item_links (source_id, target_id, link_type) VALUES (?, ?, ?)`,
				u.artifact.ID, link.TargetID, link.LinkType,
			); insErr != nil {
				restoreWritten()
				return fmt.Errorf("apply cross-artifact rewrite for %s: insert link %s: %w",
					u.artifact.ID, link.TargetID, insErr)
			}
		}
	}

	return nil
}
