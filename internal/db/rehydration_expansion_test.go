package db_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/db"
	"github.com/backlogit/backlogit/internal/stash"
)

// TASK-002.01.04: Update rehydration engine for new fields.

func TestRehydrate_NewFieldsFlowThrough(t *testing.T) {
	// Arrange — write a Markdown file with all new fields
	ws := t.TempDir()
	md := `---
id: T001
title: Rehydration test
status: queued
artifact_type: task
assigned_to: alice
owner: bob
labels:
  - backend
  - urgent
dependencies:
  - T002
references:
  - docs/spec.md
commit: abc123
---

Description body`

	require.NoError(t, os.MkdirAll(filepath.Join(ws, "tasks"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(ws, "tasks", "T001.md"), []byte(md), 0o644))

	database := setupTestDB(t)
	ctx := context.Background()

	// Act
	count, err := db.Rehydrate(ctx, ws, database)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Assert — retrieve from SQLite and verify new fields
	got, err := db.GetItem(ctx, database, "T001")
	require.NoError(t, err)
	assert.Equal(t, "alice", got.AssignedTo)
	assert.Equal(t, "bob", got.Owner)
	assert.Equal(t, []string{"backend", "urgent"}, got.Labels)
	assert.Equal(t, []string{"T002"}, got.Dependencies)
	assert.Equal(t, []string{"docs/spec.md"}, got.References)
	assert.Equal(t, "abc123", got.Commit)
}

func TestRehydrate_EmptyNewFields(t *testing.T) {
	// Arrange — Markdown with no new fields
	ws := t.TempDir()
	md := `---
id: T002
title: Minimal task
status: queued
artifact_type: task
---

Body`

	require.NoError(t, os.MkdirAll(filepath.Join(ws, "tasks"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(ws, "tasks", "T002.md"), []byte(md), 0o644))

	database := setupTestDB(t)
	ctx := context.Background()

	// Act
	count, err := db.Rehydrate(ctx, ws, database)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Assert
	got, err := db.GetItem(ctx, database, "T002")
	require.NoError(t, err)
	assert.Empty(t, got.AssignedTo)
	assert.Empty(t, got.Labels)
}

func TestRehydrate_IndexesPerItemLogs(t *testing.T) {
	// Arrange
	ws := t.TempDir()
	md := `---
id: T003
title: Logged task
status: queued
artifact_type: task
---

Body`

	require.NoError(t, os.MkdirAll(filepath.Join(ws, "queue"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(ws, "queue", "T003.md"), []byte(md), 0o644))

	logsDir := filepath.Join(ws, "logs")
	require.NoError(t, os.MkdirAll(logsDir, 0o755))
	logLines := `{"timestamp":"2026-04-03T00:00:00Z","actor":"alice","item_id":"T003","event_type":"comment","delta":{"comment":"investigated issue"}}` + "\n" +
		`{"timestamp":"2026-04-03T00:01:00Z","actor":"bob","item_id":"T003","event_type":"worklog","delta":{"summary":"implemented fix"}}` + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(logsDir, "T003.jsonl"), []byte(logLines), 0o644))

	database := setupTestDB(t)
	ctx := context.Background()

	// Act
	count, err := db.Rehydrate(ctx, ws, database)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	entries, err := db.ListItemLogEntries(ctx, database, "T003", 10)
	require.NoError(t, err)

	searchResults, err := db.SearchItemLogEntries(ctx, database, "implemented fix", 10)
	require.NoError(t, err)

	// Assert
	require.Len(t, entries, 2)
	assert.Equal(t, "alice", entries[0].Actor)
	assert.Equal(t, "comment", entries[0].EventType)
	assert.Equal(t, "bob", entries[1].Actor)
	assert.Equal(t, "worklog", entries[1].EventType)
	assert.Contains(t, entries[0].LogPath, "logs/T003.jsonl")

	require.Len(t, searchResults, 1)
	assert.Equal(t, "T003", searchResults[0].ItemID)
	assert.Equal(t, "worklog", searchResults[0].EventType)
}

func TestRehydrate_StashJSONLOverridesLegacyAndTracksSourcePath(t *testing.T) {
	// Arrange
	ws := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(ws, "queue"), 0o755))
	legacyContent := stash.RenderContent(nil, []stash.Entry{
		{ID: "ABC12345", Priority: "low", Kind: "bug", Text: "legacy stash entry"},
		{ID: "LEGACY01", Priority: "medium", Kind: "task", Text: "legacy only entry"},
	})
	require.NoError(t, os.WriteFile(filepath.Join(ws, "queue", stash.FileName), []byte(legacyContent), 0o644))
	jsonlFile, err := os.Create(filepath.Join(ws, stash.JSONLFileName))
	require.NoError(t, err)
	require.NoError(t, stash.WriteJSONL(jsonlFile, []stash.Entry{
		{ID: "ABC12345", Priority: "critical", Kind: "bug", Text: "jsonl override entry"},
		{ID: "JSONL001", Priority: "high", Kind: "feature", Text: "jsonl only entry"},
	}))
	require.NoError(t, jsonlFile.Close())

	database := setupTestDB(t)
	ctx := context.Background()

	// Act
	count, err := db.Rehydrate(ctx, ws, database)
	require.NoError(t, err)
	// When stash.jsonl is present, .stash.md is skipped entirely.
	// Only the 2 JSONL entries are indexed; LEGACY01 is excluded.
	assert.Equal(t, 2, count)

	entries, err := db.ListStashEntries(ctx, database, false)
	require.NoError(t, err)

	// Assert
	require.Len(t, entries, 2)
	byID := make(map[string]db.StashRecord, len(entries))
	for _, entry := range entries {
		byID[entry.ID] = entry
	}
	assert.Equal(t, "jsonl override entry", byID["ABC12345"].Text)
	assert.Equal(t, "critical", byID["ABC12345"].Priority)
	assert.Equal(t, stash.JSONLFileName, byID["ABC12345"].SourcePath)
	// LEGACY01 must not appear — .stash.md is skipped when stash.jsonl exists.
	assert.NotContains(t, byID, "LEGACY01", "LEGACY01 must be excluded in a migrated workspace")
	assert.Equal(t, stash.JSONLFileName, byID["JSONL001"].SourcePath)
}

func TestRehydrate_HarvestedStashUsesCanonicalSourcePath(t *testing.T) {
	// Arrange
	ws := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(ws, "queue"), 0o755))
	artifact := `---
id: 001-F
title: Harvested feature
status: queued
artifact_type: feature
custom_fields:
  source_stash_id: ABC12345
  source_stash_priority: high
  source_stash_kind: feature
  source_stash_text: Canonical stash entry
  source_stash_path: stash.jsonl
---

Feature body`
	require.NoError(t, os.WriteFile(filepath.Join(ws, "queue", "001-F.md"), []byte(artifact), 0o644))

	database := setupTestDB(t)
	ctx := context.Background()

	// Act
	count, err := db.Rehydrate(ctx, ws, database)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	entries, err := db.ListStashEntries(ctx, database, true)
	require.NoError(t, err)

	// Assert
	require.Len(t, entries, 1)
	assert.Equal(t, "ABC12345", entries[0].ID)
	assert.Equal(t, "harvested", entries[0].State)
	assert.Equal(t, stash.JSONLFileName, entries[0].SourcePath)
	assert.Equal(t, "001-F", entries[0].ItemID)
	require.NotNil(t, entries[0].LinkedAt)
}

func TestRehydrate_SuffixHierarchicalIDs_PopulatesLevelAndNumericHierarchyPath(t *testing.T) {
	// Arrange
	ws := t.TempDir()
	queueDir := filepath.Join(ws, "queue")
	require.NoError(t, os.MkdirAll(queueDir, 0o755))

	feature := `---
id: 001-F
title: Feature root
status: queued
artifact_type: feature
---

Feature body`
	task := `---
id: 001.002-T
title: Child task
status: queued
artifact_type: task
parent_id: 001-F
---

Task body`

	require.NoError(t, os.WriteFile(filepath.Join(queueDir, "001-F.md"), []byte(feature), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(queueDir, "001.002-T.md"), []byte(task), 0o644))

	database := setupTestDB(t)
	ctx := context.Background()

	// Act
	count, err := db.Rehydrate(ctx, ws, database)
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	row := database.QueryRowContext(ctx, `SELECT level, hierarchy_path FROM items WHERE id = ?`, "001.002-T")
	var level sql.NullInt64
	var hierarchyPath sql.NullString
	require.NoError(t, row.Scan(&level, &hierarchyPath))

	// Assert
	assert.True(t, level.Valid)
	assert.EqualValues(t, 2, level.Int64)
	assert.True(t, hierarchyPath.Valid)
	assert.Equal(t, "001/001.002", hierarchyPath.String)
}

func TestEnsureSchema_CreatesItemLinksTable(t *testing.T) {
	// Arrange
	database := setupTestDB(t)

	// Act
	var count int
	err := database.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'item_links'`).Scan(&count)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}
