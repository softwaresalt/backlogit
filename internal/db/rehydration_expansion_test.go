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
