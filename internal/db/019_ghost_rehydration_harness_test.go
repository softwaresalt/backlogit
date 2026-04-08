package db_test

// 019.004-T: Fix rehydration ghost index entries.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/db"
)

func TestRehydrate_ClearsGhostEntries(t *testing.T) {
	// 019.004-T: When a Markdown file is deleted, rehydration must remove the
	// corresponding index entry rather than leaving a ghost.
	ws := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(ws, "tasks"), 0o755))

	md := `---
id: 019G001
title: Ghost task
status: queued
artifact_type: task
---

This task will be deleted.`
	mdPath := filepath.Join(ws, "tasks", "019G001.md")
	require.NoError(t, os.WriteFile(mdPath, []byte(md), 0o644))

	database := setupTestDB(t)
	ctx := context.Background()

	// First rehydration: ghost task is indexed
	_, err := db.Rehydrate(ctx, ws, database)
	require.NoError(t, err)

	ghost, err := db.GetItem(ctx, database, "019G001")
	require.NoError(t, err)
	assert.Equal(t, "019G001", ghost.ID)

	// Delete the Markdown file
	require.NoError(t, os.Remove(mdPath))

	// Second rehydration: ghost entry must be purged
	_, err = db.Rehydrate(ctx, ws, database)
	require.NoError(t, err)

	_, err = db.GetItem(ctx, database, "019G001")
	assert.Error(t, err, "ghost entry must be removed after its markdown file is deleted")
}

func TestRehydrate_DoesNotGhostFromJSONLOnly(t *testing.T) {
	// 019.004-T: Rehydration must not index items that exist only in .jsonl logs
	// without a corresponding Markdown artifact file.
	ws := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(ws, "tasks"), 0o755))

	// Create a JSONL log file with no corresponding markdown
	jsonlLog := `{"event":"created","id":"019G002","title":"Phantom"}`
	require.NoError(t, os.WriteFile(filepath.Join(ws, "tasks", "019G002.jsonl"), []byte(jsonlLog), 0o644))

	database := setupTestDB(t)
	ctx := context.Background()

	_, err := db.Rehydrate(ctx, ws, database)
	require.NoError(t, err)

	_, err = db.GetItem(ctx, database, "019G002")
	assert.Error(t, err, "JSONL-only items must not appear in the index")
}
