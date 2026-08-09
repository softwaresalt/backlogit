package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	blErrors "github.com/softwaresalt/backlogit/internal/errors"
)

// mergeSyncFallbackThreshold is the maximum number of changed files before
// MergeSync falls back to a full Rehydrate. Matches the plan specification.
const mergeSyncFallbackThreshold = 50

var syncLog = slog.With("component", "merge_sync")

// SyncEntry records a single artifact touched by a MergeSync operation,
// pairing the artifact ID with its workspace-relative file path.
type SyncEntry struct {
	// ID is the backlogit artifact ID (e.g., "037-F").
	ID string `json:"id"`
	// Path is the workspace-relative file path (e.g., "queue/037-F.md").
	Path string `json:"path"`
}

// MergeSyncResult describes the complete outcome of a MergeSync call.
type MergeSyncResult struct {
	// Added holds artifacts indexed for the first time by this sync.
	Added []SyncEntry `json:"added"`
	// Changed holds artifacts whose index entry was updated.
	Changed []SyncEntry `json:"changed"`
	// Deleted holds artifacts removed from the index because their file was deleted.
	Deleted []SyncEntry `json:"deleted"`
	// Relocated holds artifacts whose file path changed (same ID, new path).
	Relocated []SyncEntry `json:"relocated"`
	// StashRefreshed reports whether the stash table was fully rebuilt
	// because stash.jsonl appeared in the diff.
	StashRefreshed bool `json:"stash_refreshed"`
	// LogsRefreshed reports whether the item_log_entries table was fully
	// rebuilt because one or more logs/*.jsonl files appeared in the diff.
	LogsRefreshed bool `json:"logs_refreshed"`
	// FallbackUsed reports that the incremental path was skipped and a full
	// Rehydrate was performed instead.
	FallbackUsed bool `json:"fallback_used"`
	// DryRun reports that the call was made with dryRun=true, meaning no DB
	// changes were applied.
	DryRun bool `json:"dry_run"`
	// FallbackReason explains why the fallback was triggered when FallbackUsed is true.
	FallbackReason string `json:"fallback_reason,omitempty"`
}

// MergeSync performs an incremental sync of the .backlogit workspace cache.
//
// The call sequence is:
//  1. Build a current manifest from the filesystem.
//  2. Compute a diff against the provided manifest snapshot.
//  3. If ShouldFallback returns true, delegate to Rehydrate and return with
//     FallbackUsed: true.
//  4. If dryRun is true, return the diff result without modifying the database.
//  5. Apply targeted upserts for added and changed artifacts, deletes for removed
//     artifacts, and upserts for relocated artifacts within a RetryWrite-wrapped
//     transaction.
//  6. When stash.jsonl appears in the diff, run rehydrateStash.
//  7. When any logs/*.jsonl file appears in the diff, run rehydrateItemLogs.
//
// Returns the sync result, the updated manifest (suitable for storage in the
// caller's in-memory cache), and any error.
func MergeSync(
	ctx context.Context,
	workspacePath string,
	database *sql.DB,
	manifest map[string]FileEntry,
	dryRun bool,
) (MergeSyncResult, map[string]FileEntry, error) {
	log := syncLog.With("workspace", workspacePath)

	var result MergeSyncResult
	result.DryRun = dryRun

	// Step 1: Build current manifest from the filesystem.
	current, err := BuildManifest(workspacePath)
	if err != nil {
		return result, manifest, fmt.Errorf("merge sync: build manifest: %w", err)
	}

	// Step 2: Compute diff against the prior manifest.
	diff := ComputeDiff(manifest, current)

	// Step 3: Fall back to full rehydrate when the delta exceeds the threshold.
	fallback, reason := ShouldFallback(diff, len(manifest), mergeSyncFallbackThreshold)
	if fallback {
		// A dry run must never write to the database. Report that a fallback
		// would be used, along with the computed diff, without performing the
		// rehydrate (which clears and repopulates every table).
		if dryRun {
			result.FallbackUsed = true
			result.FallbackReason = reason
			result.Added = artifactSyncEntries(diff.Added)
			result.Changed = artifactSyncEntries(diff.Changed)
			result.Deleted = artifactSyncEntries(diff.Deleted)
			result.Relocated = relocationSyncEntries(diff.Relocated)
			return result, current, nil
		}
		log.Info("falling back to full rehydrate", "reason", reason)
		_, newManifest, rehydrateErr := RehydrateWithManifest(ctx, workspacePath, database)
		if rehydrateErr != nil {
			return result, manifest, fmt.Errorf("merge sync: fallback rehydrate: %w", rehydrateErr)
		}
		result.FallbackUsed = true
		result.FallbackReason = reason
		return result, newManifest, nil
	}

	// Step 4: Dry run — report the diff without writing to the database.
	if dryRun {
		result.Added = artifactSyncEntries(diff.Added)
		result.Changed = artifactSyncEntries(diff.Changed)
		result.Deleted = artifactSyncEntries(diff.Deleted)
		result.Relocated = relocationSyncEntries(diff.Relocated)
		return result, current, nil
	}

	// Step 5: Apply targeted upserts (added, changed, relocated).
	var upsertTargets []FileEntry
	upsertTargets = append(upsertTargets, diff.Added...)
	upsertTargets = append(upsertTargets, diff.Changed...)
	for _, r := range diff.Relocated {
		upsertTargets = append(upsertTargets, r.Entry)
	}

	// failedPaths accumulates RelPath values for artifacts that failed to parse
	// or upsert. After the transaction, their prior manifest entries are restored
	// so they remain visible in the next diff and will be retried.
	var failedPaths []string
	if len(upsertTargets) > 0 {
		if err := RetryWrite(ctx, func() error {
			failedPaths = nil // reset on each retry attempt
			tx, txErr := database.BeginTx(ctx, nil)
			if txErr != nil {
				return fmt.Errorf("begin upsert transaction: %w", txErr)
			}
			defer tx.Rollback() //nolint:errcheck

			for _, entry := range upsertTargets {
				if entry.Kind != FileKindArtifact || entry.ItemID == "" {
					continue
				}
				absPath := filepath.Join(workspacePath, filepath.FromSlash(entry.RelPath))
				artifact, parseErr := parseMarkdownArtifact(absPath)
				if parseErr != nil {
					log.Warn("skipping artifact: parse error", "path", entry.RelPath, "error", parseErr)
					failedPaths = append(failedPaths, entry.RelPath)
					continue
				}
				if upsertErr := UpsertItemTx(ctx, tx, artifact); upsertErr != nil {
					log.Warn("skipping artifact: upsert error", "id", artifact.ID, "error", upsertErr)
					failedPaths = append(failedPaths, entry.RelPath)
					continue
				}

				// Mirror the full rehydration logic: update derived hierarchy
				// fields and refresh dep/link edges for the changed artifact so
				// the incremental path stays in sync with what a full Rehydrate
				// would produce.
				if artifact.Level == 0 && isHierarchicalID(artifact.ID) {
					level := strings.Count(artifact.ID, ".") + 1
					hierarchyPath := hierarchyPathFromID(artifact.ID)
					if _, execErr := tx.ExecContext(ctx,
						`UPDATE items SET level = ?, hierarchy_path = ? WHERE id = ?`,
						level, hierarchyPath, artifact.ID,
					); execErr != nil {
						log.Warn("failed to set level/hierarchy_path", "id", artifact.ID, "error", execErr)
					}
				}

				// Refresh dependency edges: delete stale rows then re-insert.
				if _, delErr := tx.ExecContext(ctx, `DELETE FROM item_deps WHERE item_id = ?`, artifact.ID); delErr != nil {
					log.Warn("failed to clear deps", "id", artifact.ID, "error", delErr)
				}
				for _, dep := range artifact.Dependencies {
					if dep.ID == "" {
						continue
					}
					depType := dep.Type
					if depType == "" {
						depType = "blocks"
					}
					if depErr := upsertDependencyTx(ctx, tx, artifact.ID, dep.ID, depType); depErr != nil {
						log.Warn("failed to upsert dependency", "item_id", artifact.ID, "dep_id", dep.ID, "error", depErr)
					}
				}

				// Refresh link edges: delete stale rows then re-insert.
				if _, delErr := tx.ExecContext(ctx, `DELETE FROM item_links WHERE source_id = ?`, artifact.ID); delErr != nil {
					log.Warn("failed to clear links", "id", artifact.ID, "error", delErr)
				}
				for _, link := range artifact.Links {
					if strings.TrimSpace(link.TargetID) == "" || strings.TrimSpace(link.LinkType) == "" {
						continue
					}
					if !isValidLinkType(link.LinkType) {
						log.Warn("merge_sync: skipping invalid link_type",
							"source_id", artifact.ID, "target_id", link.TargetID, "link_type", link.LinkType)
						continue
					}
					if _, execErr := tx.ExecContext(ctx,
						`INSERT OR IGNORE INTO item_links (source_id, target_id, link_type) VALUES (?, ?, ?)`,
						artifact.ID, link.TargetID, link.LinkType,
					); execErr != nil {
						log.Warn("failed to upsert link", "source_id", artifact.ID, "target_id", link.TargetID, "link_type", link.LinkType, "error", execErr)
					}
				}
			}

			return tx.Commit()
		}); err != nil {
			return result, manifest, fmt.Errorf("merge sync: upsert batch: %w", err)
		}

		// Restore prior manifest entries for artifacts that could not be
		// indexed. This keeps them in the diff on the next MergeSync call so
		// they are automatically retried.
		for _, relPath := range failedPaths {
			if old, ok := manifest[relPath]; ok {
				current[relPath] = old
			} else {
				delete(current, relPath)
			}
		}
	}

	// Delete removed artifacts (one cascade transaction per item to reuse existing tested logic).
	// Only successfully deleted entries (or already-absent ones) advance the manifest and appear
	// in result.Deleted. Failed deletes restore the prior manifest entry so the deletion is
	// retried on the next MergeSync call.
	var successfulDeletes []FileEntry
	for _, entry := range diff.Deleted {
		if entry.Kind != FileKindArtifact || entry.ItemID == "" {
			continue
		}
		if delErr := DeleteItemCascade(ctx, database, entry.ItemID); delErr != nil {
			// Tolerate ErrNotFound — item may already be absent from the index.
			if !errors.Is(delErr, blErrors.ErrNotFound) {
				log.Warn("failed to cascade-delete artifact", "id", entry.ItemID, "error", delErr)
				// Restore the prior manifest entry so the deletion is retried next call.
				if old, ok := manifest[entry.RelPath]; ok {
					current[entry.RelPath] = old
				}
				continue
			}
		}
		successfulDeletes = append(successfulDeletes, entry)
	}

	// Populate result with artifact-level sync entries.
	result.Added = artifactSyncEntries(diff.Added)
	result.Changed = artifactSyncEntries(diff.Changed)
	result.Deleted = artifactSyncEntries(successfulDeletes)
	result.Relocated = relocationSyncEntries(diff.Relocated)

	// Step 6: Refresh stash when stash.jsonl appears in the diff.
	if diffContainsKind(diff, FileKindStash) {
		harvestedStash := make(map[string]StashRecord)
		if _, stashErr := rehydrateStash(ctx, workspacePath, database, harvestedStash); stashErr != nil {
			log.Warn("stash refresh failed after merge sync", "error", stashErr)
		} else {
			result.StashRefreshed = true
		}
	}

	// Step 7: Refresh item logs when any logs/*.jsonl appears in the diff.
	if diffContainsKind(diff, FileKindLog) {
		if _, logErr := rehydrateItemLogs(ctx, workspacePath, database); logErr != nil {
			log.Warn("log refresh failed after merge sync", "error", logErr)
		} else {
			result.LogsRefreshed = true
		}
	}

	return result, current, nil
}

// artifactSyncEntries converts a slice of FileEntry values to SyncEntry values,
// filtering out entries without an ItemID (non-artifact files).
func artifactSyncEntries(entries []FileEntry) []SyncEntry {
	out := make([]SyncEntry, 0, len(entries))
	for _, e := range entries {
		if e.ItemID == "" {
			continue
		}
		out = append(out, SyncEntry{ID: e.ItemID, Path: e.RelPath})
	}
	return out
}

// relocationSyncEntries converts a slice of RelocationEntry values to SyncEntry values.
func relocationSyncEntries(relocations []RelocationEntry) []SyncEntry {
	out := make([]SyncEntry, 0, len(relocations))
	for _, r := range relocations {
		out = append(out, SyncEntry{ID: r.ItemID, Path: r.NewPath})
	}
	return out
}

// diffContainsKind reports whether any entry in the diff has the given FileKind.
func diffContainsKind(diff DiffResult, kind FileKind) bool {
	for _, e := range diff.Added {
		if e.Kind == kind {
			return true
		}
	}
	for _, e := range diff.Changed {
		if e.Kind == kind {
			return true
		}
	}
	for _, e := range diff.Deleted {
		if e.Kind == kind {
			return true
		}
	}
	for _, r := range diff.Relocated {
		if r.Entry.Kind == kind {
			return true
		}
	}
	return false
}
