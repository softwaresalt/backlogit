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
	"strings"
	"time"

	"github.com/backlogit/backlogit/internal/events"
	"github.com/backlogit/backlogit/internal/models"
	"github.com/backlogit/backlogit/internal/stash"
)

// Rehydrate walks the workspace directory tree and rebuilds the SQLite index
// from the Markdown source files. Files that fail to parse are skipped with a
// debug log entry. Returns the number of artifacts successfully indexed.
func Rehydrate(ctx context.Context, workspacePath string, db *sql.DB) (int, error) {
	count := 0
	harvestedStash := make(map[string]StashRecord)
	if err := DeleteAllItemLogs(ctx, db); err != nil {
		return 0, err
	}

	err := filepath.WalkDir(workspacePath, func(path string, d fs.DirEntry, walkErr error) error {
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

		if upsertErr := UpsertItem(ctx, db, artifact); upsertErr != nil {
			slog.Warn("failed to upsert artifact", "path", path, "error", upsertErr)
			return nil
		}
		if record, ok := stashRecordFromArtifact(artifact); ok {
			harvestedStash[record.ID] = record
		}

		// Derive level and hierarchy_path from the ID for hierarchical IDs
		// (segments separated by "."), e.g. "001" → level=1, "001.002" → level=2.
		// Only update when the artifact does not already carry these values.
		if artifact.Level == 0 && isHierarchicalID(artifact.ID) {
			level := strings.Count(artifact.ID, ".") + 1
			hierarchyPath := hierarchyPathFromID(artifact.ID)
			if _, execErr := db.ExecContext(ctx,
				`UPDATE items SET level = ?, hierarchy_path = ? WHERE id = ?`,
				level, hierarchyPath, artifact.ID,
			); execErr != nil {
				slog.Warn("failed to set level/hierarchy_path", "id", artifact.ID, "error", execErr)
			}
		}

		// Upsert dependency edges from frontmatter.
		if len(artifact.Dependencies) > 0 {
			for _, depID := range artifact.Dependencies {
				if depID == "" {
					continue
				}
				// Best-effort: skip if target doesn't exist yet (will be linked on subsequent rehydration).
				if depErr := upsertDependencyBestEffort(ctx, db, artifact.ID, depID); depErr != nil {
					slog.Warn("failed to upsert dependency", "item_id", artifact.ID, "dep_id", depID, "error", depErr)
				}
			}
		}

		count++
		return nil
	})
	if err != nil {
		return count, fmt.Errorf("rehydrate walk: %w", err)
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

func upsertDependencyBestEffort(ctx context.Context, db *sql.DB, itemID, dependsOn string) error {
	_, err := db.ExecContext(ctx,
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
		for _, ch := range seg {
			if ch < '0' || ch > '9' {
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
	segments := make([]string, len(parts))
	for i := range parts {
		segments[i] = strings.Join(parts[:i+1], ".")
	}
	return strings.Join(segments, "/")
}

func rehydrateItemLogs(ctx context.Context, workspacePath string, database *sql.DB) error {
	logsDir := filepath.Join(workspacePath, "logs")
	if _, err := os.Stat(logsDir); os.IsNotExist(err) {
		return nil
	}

	return filepath.WalkDir(logsDir, func(path string, d fs.DirEntry, walkErr error) error {
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
		if len(eventsForItem) == 0 {
			return nil
		}
		for _, event := range eventsForItem {
			if err := IndexEvent(ctx, database, logsDir, event); err != nil {
				slog.Warn("failed to index item log event", "item_id", event.ItemID, "path", path, "error", err)
			}
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
	// Read entries from legacy .stash.md if present.
	stashPath := filepath.Join(workspacePath, "queue", stash.FileName)
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

	// Also read entries from stash.jsonl if present (new format).
	jsonlPath := filepath.Join(workspacePath, stash.JSONLFileName)
	if _, err := os.Stat(jsonlPath); err == nil {
		f, openErr := os.Open(jsonlPath)
		if openErr != nil {
			return 0, fmt.Errorf("open stash jsonl: %w", openErr)
		}
		jsonlEntries, readErr := stash.ReadJSONL(f)
		_ = f.Close()
		if readErr != nil {
			return 0, fmt.Errorf("read stash jsonl: %w", readErr)
		}
		// Merge: deduplicate by ID, preferring JSONL entries when duplicates exist.
		for _, e := range jsonlEntries {
			appendActive(activeStashRecord(e, filepath.ToSlash(stash.JSONLFileName), now))
		}
		slog.Debug("merged stash jsonl entries", "count", len(jsonlEntries))
	} else if !os.IsNotExist(err) {
		return 0, fmt.Errorf("stat stash jsonl: %w", err)
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
		SourcePath:     filepath.ToSlash(filepath.Join("queue", stash.FileName)),
		ItemID:         artifact.ID,
		UpdatedAt:      artifact.UpdatedAt,
		DeliberationID: "",
	}
	if priority, _ := artifact.CustomFields["source_stash_priority"].(string); priority != "" {
		record.Priority = priority
	}
	if deliberationID, _ := artifact.CustomFields["source_deliberation_id"].(string); deliberationID != "" {
		record.DeliberationID = deliberationID
	}
	linkedAt := artifact.UpdatedAt
	record.LinkedAt = &linkedAt
	return record, true
}
