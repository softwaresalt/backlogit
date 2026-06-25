package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/models"
	"github.com/softwaresalt/backlogit/internal/stash"
)

// rehydrateBatchSize is the number of artifacts inserted per transaction during
// the batch-rebuild phase. Smaller batches hold the write lock for less time,
// reducing contention under concurrent MCP workloads.
const rehydrateBatchSize = 100

// collectedArtifact holds a parsed artifact ready for batch insertion.
type collectedArtifact struct {
	artifact *models.Artifact
}

// warnOnDuplicateSourceIDs emits exactly one warning per artifact ID that is
// declared by two or more source files (066.004-T). Duplicate source files --
// most acutely the same root ID present in both the queue and the archive (the
// 066-F bug) -- are otherwise masked by the PK-keyed upsert, which silently
// collapses them to a single indexed row. The warning is observational only:
// it does not modify the rebuild transaction or the collapse result. IDs are
// reported in sorted order for deterministic output.
func warnOnDuplicateSourceIDs(idToPaths map[string][]string) {
	ids := make([]string, 0, len(idToPaths))
	for id, paths := range idToPaths {
		if len(paths) >= 2 {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	for _, id := range ids {
		paths := append([]string(nil), idToPaths[id]...)
		sort.Strings(paths)
		slog.Warn("rehydrate: duplicate source id detected; sources collapse to a single indexed row",
			"id", id, "paths", paths)
	}
}

// Rehydrate walks the workspace directory tree and rebuilds the SQLite index
// from the Markdown source files. Files that fail to parse are skipped with a
// debug log entry. Returns the number of artifacts successfully indexed.
//
// The rebuild is split into three phases to reduce write-lock hold time:
//
//  1. Collect: walk the filesystem and parse all Markdown files into memory.
//     No database interaction occurs during this phase.
//  2. Clear: a single IMMEDIATE transaction deletes all existing index rows so
//     that removed or renamed Markdown files do not leave ghost entries.
//  3. Batch-insert: parsed artifacts are inserted in batches of
//     rehydrateBatchSize per transaction. Each batch acquires and releases the
//     write lock independently, allowing concurrent readers to make progress
//     between batches.
//
// Note: between the clear commit and the final batch commit the index is empty
// or partially populated. This is acceptable because backlogit.db is an
// ephemeral cache that can be rebuilt at any time.
func Rehydrate(ctx context.Context, workspacePath string, db *sql.DB) (int, error) {
	harvestedStash := make(map[string]StashRecord)

	// ── Phase 1: Collect ──────────────────────────────────────────────────────
	var collected []collectedArtifact
	// 066.004-T: Track every source file per ID so duplicate source files (e.g.
	// the same root ID present in both queue and archive -- the 066-F bug) can be
	// surfaced as a warning. This is purely observational: it does not alter the
	// atomic clear+rebuild transaction below, and the PK-keyed upsert still
	// collapses duplicates to a single indexed row.
	idToPaths := make(map[string][]string)
	if walkErr := filepath.WalkDir(workspacePath, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			slog.Debug("walk error, skipping", "path", path, "error", walkErr)
			return nil
		}
		if d.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}
		if strings.EqualFold(filepath.Base(path), ".stash.md") {
			return nil
		}

		artifact, parseErr := parseMarkdownArtifact(path)
		if parseErr != nil {
			slog.Debug("skipping unparseable file", "path", path, "error", parseErr)
			return nil
		}
		if artifact == nil {
			return nil
		}

		collected = append(collected, collectedArtifact{artifact: artifact})
		idToPaths[artifact.ID] = append(idToPaths[artifact.ID], path)
		if record, ok := stashRecordFromArtifact(artifact); ok {
			harvestedStash[record.ID] = record
		}
		return nil
	}); walkErr != nil {
		return 0, fmt.Errorf("rehydrate walk: %w", walkErr)
	}

	// 066.004-T: Emit exactly one warning per duplicated source ID before the
	// transaction begins. The atomic clear+rebuild below is left untouched so the
	// SQLite rebuild keeps its single-transaction integrity guarantee.
	warnOnDuplicateSourceIDs(idToPaths)

	// ── Phase 2: Clear ────────────────────────────────────────────────────────
	if clearErr := RetryWrite(ctx, func() error {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin clear transaction: %w", err)
		}
		defer tx.Rollback() //nolint:errcheck

		if err := deleteAllItemLogs(ctx, tx); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM items`); err != nil {
			return fmt.Errorf("clear items: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM item_deps`); err != nil {
			return fmt.Errorf("clear item_deps: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM item_links`); err != nil {
			return fmt.Errorf("clear item_links: %w", err)
		}
		return tx.Commit()
	}); clearErr != nil {
		return 0, clearErr
	}

	// ── Phase 3: Batch-insert ─────────────────────────────────────────────────
	count := 0
	for i := 0; i < len(collected); i += rehydrateBatchSize {
		end := i + rehydrateBatchSize
		if end > len(collected) {
			end = len(collected)
		}
		batch := collected[i:end]

		batchCount := 0
		batchErr := RetryWrite(ctx, func() error {
			// Reset batchCount at the start of each attempt so that a retried
			// transaction does not double-count rows inserted in the failed attempt.
			batchCount = 0
			tx, err := db.BeginTx(ctx, nil)
			if err != nil {
				return fmt.Errorf("begin batch transaction at offset %d: %w", i, err)
			}
			defer tx.Rollback() //nolint:errcheck

			for _, ca := range batch {
				artifact := ca.artifact
				if upsertErr := upsertItemTx(ctx, tx, artifact); upsertErr != nil {
					slog.Warn("failed to upsert artifact", "id", artifact.ID, "error", upsertErr)
					continue
				}

				if artifact.Level == 0 && isHierarchicalID(artifact.ID) {
					level := strings.Count(artifact.ID, ".") + 1
					hierarchyPath := hierarchyPathFromID(artifact.ID)
					if _, execErr := tx.ExecContext(ctx,
						`UPDATE items SET level = ?, hierarchy_path = ? WHERE id = ?`,
						level, hierarchyPath, artifact.ID,
					); execErr != nil {
						slog.Warn("failed to set level/hierarchy_path", "id", artifact.ID, "error", execErr)
					}
				}

				for _, depID := range artifact.Dependencies {
					if depID == "" {
						continue
					}
					if depErr := upsertDependencyTx(ctx, tx, artifact.ID, depID); depErr != nil {
						slog.Warn("failed to upsert dependency", "item_id", artifact.ID, "dep_id", depID, "error", depErr)
					}
				}
				for _, link := range artifact.Links {
					if strings.TrimSpace(link.TargetID) == "" || strings.TrimSpace(link.LinkType) == "" {
						continue
					}
					if !isValidLinkType(link.LinkType) {
						slog.Warn("rehydration: skipping invalid link_type",
							"source_id", artifact.ID, "target_id", link.TargetID, "link_type", link.LinkType)
						continue
					}
					if _, execErr := tx.ExecContext(ctx,
						`INSERT OR IGNORE INTO item_links (source_id, target_id, link_type) VALUES (?, ?, ?)`,
						artifact.ID, link.TargetID, link.LinkType,
					); execErr != nil {
						slog.Warn("failed to upsert link", "source_id", artifact.ID, "target_id", link.TargetID, "link_type", link.LinkType, "error", execErr)
					}
				}

				batchCount++
			}

			return tx.Commit()
		})
		if batchErr != nil {
			return count, fmt.Errorf("rehydration batch at offset %d: %w", i, batchErr)
		}
		count += batchCount
	}

	stashCount, stashErr := rehydrateStash(ctx, workspacePath, db, harvestedStash)
	if stashErr != nil {
		return count, stashErr
	}
	count += stashCount

	if err := rehydrateItemLogs(ctx, workspacePath, db); err != nil {
		return count, err
	}

	return count, nil
}

// parseMarkdownArtifact reads a Markdown file and extracts the artifact using
// the models layer directly, avoiding an import of the parser package (which
// would create an import cycle through core).
func parseMarkdownArtifact(path string) (*models.Artifact, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	fm, body, err := models.ParseFrontmatter(string(data))
	if err != nil {
		return nil, fmt.Errorf("parse frontmatter %s: %w", path, err)
	}
	if fm == nil {
		return nil, nil
	}

	artifact, err := models.ArtifactFromFrontmatter(fm, body)
	if err != nil {
		return nil, fmt.Errorf("artifact from frontmatter %s: %w", path, err)
	}

	return artifact, nil
}

func upsertDependencyTx(ctx context.Context, tx *sql.Tx, itemID, dependsOn string) error {
	_, err := tx.ExecContext(ctx,
		`INSERT OR IGNORE INTO item_deps (item_id, depends_on, dep_type) VALUES (?, ?, 'blocks')`,
		itemID, dependsOn,
	)
	return err
}

// isHierarchicalID reports whether an ID uses the dot-separated numeric hierarchy
// format (e.g., "001", "001.002", "001.002.003"). Non-hierarchical IDs such as
// "T001" or "BUG-3" are excluded.
func isHierarchicalID(id string) bool {
	if id == "" {
		return false
	}
	for _, seg := range strings.Split(id, ".") {
		if seg == "" {
			return false
		}
		numeric := leadingDigits(seg)
		if numeric == "" {
			return false
		}
		suffix := strings.TrimPrefix(seg, numeric)
		if suffix == "" {
			continue
		}
		if !strings.HasPrefix(suffix, "-") {
			return false
		}
		for _, ch := range strings.TrimPrefix(suffix, "-") {
			if (ch < 'A' || ch > 'Z') && (ch < 'a' || ch > 'z') {
				return false
			}
		}
	}
	return true
}

// hierarchyPathFromID builds the ancestor path for a hierarchical ID.
// "001.002.003" → "001/001.002/001.002.003".
func hierarchyPathFromID(id string) string {
	parts := strings.Split(id, ".")
	numericParts := make([]string, len(parts))
	for i := range parts {
		numericParts[i] = leadingDigits(parts[i])
	}
	segments := make([]string, len(parts))
	for i := range numericParts {
		segments[i] = strings.Join(numericParts[:i+1], ".")
	}
	return strings.Join(segments, "/")
}

func leadingDigits(value string) string {
	var digits strings.Builder
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			break
		}
		digits.WriteRune(ch)
	}
	return digits.String()
}

func rehydrateItemLogs(ctx context.Context, workspacePath string, database *sql.DB) error {
	logsDir := filepath.Join(workspacePath, "logs")
	if _, err := os.Stat(logsDir); os.IsNotExist(err) {
		return nil
	}

	// Collect all log events first, then index them in a single transaction.
	// Without batching, each IndexEvent call auto-commits with a disk sync,
	// causing O(n) fsyncs that make rehydration extremely slow on workspaces
	// with hundreds of log entries.
	type logEvent struct {
		event events.Event
		path  string
	}
	var allEvents []logEvent

	if walkErr := filepath.WalkDir(logsDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			slog.Debug("log walk error, skipping", "path", path, "error", walkErr)
			return nil
		}
		if d.IsDir() || filepath.Ext(path) != ".jsonl" {
			return nil
		}

		itemID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		eventsForItem, err := parseItemLogFile(path, itemID)
		if err != nil {
			slog.Warn("failed to parse item log", "path", path, "error", err)
			return nil
		}
		for _, event := range eventsForItem {
			allEvents = append(allEvents, logEvent{event: event, path: path})
		}
		return nil
	}); walkErr != nil {
		return walkErr
	}

	if len(allEvents) == 0 {
		return nil
	}

	return RetryWrite(ctx, func() error {
		tx, err := database.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin log rehydration transaction: %w", err)
		}
		defer tx.Rollback() //nolint:errcheck

		for _, le := range allEvents {
			if err := indexEventTx(ctx, tx, logsDir, le.event); err != nil {
				slog.Warn("failed to index item log event", "item_id", le.event.ItemID, "path", le.path, "error", err)
			}
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit log rehydration: %w", err)
		}
		return nil
	})
}

func parseItemLogFile(path, itemID string) ([]events.Event, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read item log %s: %w", path, err)
	}
	lines := strings.Split(string(data), "\n")
	result := make([]events.Event, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var event events.Event
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return nil, fmt.Errorf("parse item log line: %w", err)
		}
		if event.ItemID == "" {
			event.ItemID = itemID
		}
		result = append(result, event)
	}
	return result, nil
}

func rehydrateStash(ctx context.Context, workspacePath string, database *sql.DB, harvested map[string]StashRecord) (int, error) {
	stashPath := filepath.Join(workspacePath, "queue", stash.FileName)
	jsonlPath := filepath.Join(workspacePath, stash.JSONLFileName)

	activeEntries := []StashRecord{}
	activeIndex := make(map[string]int)
	appendActive := func(entry StashRecord) {
		if idx, exists := activeIndex[entry.ID]; exists {
			activeEntries[idx] = entry
			return
		}
		activeIndex[entry.ID] = len(activeEntries)
		activeEntries = append(activeEntries, entry)
	}
	now := time.Now().UTC()

	// Attempt to read stash.jsonl. When it contains at least one entry the
	// workspace is considered migrated and the file is used exclusively.
	// An empty stash.jsonl (e.g. created by backlogit init on a legacy
	// workspace) is treated as absent so that .stash.md entries are not
	// silently lost.
	var jsonlEntries []stash.Entry
	if _, statErr := os.Stat(jsonlPath); statErr == nil {
		f, openErr := os.Open(jsonlPath)
		if openErr != nil {
			return 0, fmt.Errorf("open stash jsonl: %w", openErr)
		}
		var readErr error
		jsonlEntries, readErr = stash.ReadJSONL(f)
		closeErr := f.Close()
		if readErr != nil {
			return 0, fmt.Errorf("read stash jsonl: %w", readErr)
		}
		if closeErr != nil {
			return 0, fmt.Errorf("close stash jsonl: %w", closeErr)
		}
	} else if !os.IsNotExist(statErr) {
		return 0, fmt.Errorf("stat stash jsonl: %w", statErr)
	}

	if len(jsonlEntries) > 0 {
		// Migrated workspace: stash.jsonl has entries; use it exclusively.
		if _, err := os.Stat(stashPath); err == nil {
			slog.Debug("skipping legacy stash.md: stash.jsonl has entries", "workspace", workspacePath)
		}
		for _, e := range jsonlEntries {
			appendActive(activeStashRecord(e, filepath.ToSlash(stash.JSONLFileName), now))
		}
		slog.Debug("indexed stash from jsonl", "count", len(jsonlEntries))
	} else {
		// Legacy or fresh workspace: stash.jsonl absent or empty — read .stash.md.
		if _, err := os.Stat(stashPath); err == nil {
			_, entries, parseErr := stash.ParseFile(stashPath)
			if parseErr != nil {
				return 0, fmt.Errorf("parse stash file: %w", parseErr)
			}
			legacySource := filepath.ToSlash(filepath.Join("queue", stash.FileName))
			for _, entry := range entries {
				appendActive(activeStashRecord(entry, legacySource, now))
			}
		} else if !os.IsNotExist(err) {
			return 0, fmt.Errorf("stat stash file: %w", err)
		}
	}

	if err := RehydrateStashIndex(ctx, database, activeEntries, harvested); err != nil {
		return 0, err
	}
	return len(activeEntries), nil
}

func activeStashRecord(entry stash.Entry, sourcePath string, updatedAt time.Time) StashRecord {
	return StashRecord{
		ID:             entry.ID,
		Priority:       entry.Priority,
		Kind:           entry.Kind,
		Text:           entry.Text,
		DeliberationID: entry.DeliberationID,
		State:          "active",
		SourcePath:     sourcePath,
		UpdatedAt:      updatedAt,
	}
}

func stashRecordFromArtifact(artifact *models.Artifact) (StashRecord, bool) {
	if artifact == nil || artifact.CustomFields == nil {
		return StashRecord{}, false
	}
	stashID, _ := artifact.CustomFields["source_stash_id"].(string)
	if stashID == "" {
		return StashRecord{}, false
	}
	kind, _ := artifact.CustomFields["source_stash_kind"].(string)
	text, _ := artifact.CustomFields["source_stash_text"].(string)
	record := StashRecord{
		ID:             stashID,
		Priority:       stash.DefaultPriority,
		Kind:           kind,
		Text:           text,
		State:          "harvested",
		SourcePath:     filepath.ToSlash(stash.JSONLFileName),
		ItemID:         artifact.ID,
		UpdatedAt:      artifact.UpdatedAt,
		DeliberationID: "",
	}
	if priority, _ := artifact.CustomFields["source_stash_priority"].(string); priority != "" {
		record.Priority = priority
	}
	if sourcePath, _ := artifact.CustomFields["source_stash_path"].(string); strings.TrimSpace(sourcePath) != "" {
		record.SourcePath = filepath.ToSlash(strings.TrimSpace(sourcePath))
	}
	if deliberationID, _ := artifact.CustomFields["source_deliberation_id"].(string); deliberationID != "" {
		record.DeliberationID = deliberationID
	}
	linkedAt := artifact.UpdatedAt
	record.LinkedAt = &linkedAt
	return record, true
}
