package db_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/db"
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
