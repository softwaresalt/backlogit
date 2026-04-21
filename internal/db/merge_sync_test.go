package db_test

// 037.003-T: Incremental Sync Engine
//
// These harnesses verify the MergeSync function:
//   - Added artifact file is indexed after sync
//   - Modified artifact file is re-indexed after sync
//   - Deleted artifact file is removed from index after sync
//   - Relocated artifact file results in a single index update (not delete+add)
//   - stash.jsonl change triggers StashRefreshed
//   - log file change triggers LogsRefreshed
//   - Delta exceeding threshold triggers FallbackUsed
//   - dry_run=true returns diff without modifying the database

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/db"
)

const syncTestArtifactA = `---
id: 001-F
title: Feature A
artifact_type: feature
status: queued
---
Body.
`

const syncTestArtifactB = `---
id: 002-T
title: Task B
artifact_type: task
status: queued
parent_id: 001-F
---
Body.
`

// setupSyncWorkspace creates a temp workspace with one artifact and a baseline manifest.
func setupSyncWorkspace(t *testing.T) (ws string, database *sql.DB, manifest map[string]db.FileEntry) {
	t.Helper()
	ws = t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(ws, "queue"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(ws, "queue", "001-F.md"), []byte(syncTestArtifactA), 0o644))

	database = setupTestDB(t)
	ctx := context.Background()
	_, manifest, err := db.RehydrateWithManifest(ctx, ws, database)
	require.NoError(t, err)
	return ws, database, manifest
}

func TestMergeSync_DetectsAndIndexesAddedArtifact(t *testing.T) {
	ws, database, manifest := setupSyncWorkspace(t)
	ctx := context.Background()

	// Add a second artifact after the baseline manifest was captured.
	require.NoError(t, os.WriteFile(filepath.Join(ws, "queue", "002-T.md"), []byte(syncTestArtifactB), 0o644))

	result, _, err := db.MergeSync(ctx, ws, database, manifest, false)
	require.NoError(t, err)

	require.Len(t, result.Added, 1, "expected one added artifact")
	assert.Equal(t, "002-T", result.Added[0].ID)
	assert.Empty(t, result.Changed)
	assert.Empty(t, result.Deleted)
	assert.False(t, result.FallbackUsed)
	assert.False(t, result.DryRun)

	// Verify the item is now in the database.
	got, err := db.GetItem(ctx, database, "002-T")
	require.NoError(t, err)
	assert.Equal(t, "Task B", got.Title)
}

func TestMergeSync_DetectsAndReIndexesChangedArtifact(t *testing.T) {
	ws, database, manifest := setupSyncWorkspace(t)
	ctx := context.Background()

	// Overwrite the artifact with an updated title.
	updated := `---
id: 001-F
title: Feature A — Updated
artifact_type: feature
status: active
---
Updated body.
`
	require.NoError(t, os.WriteFile(filepath.Join(ws, "queue", "001-F.md"), []byte(updated), 0o644))

	result, _, err := db.MergeSync(ctx, ws, database, manifest, false)
	require.NoError(t, err)

	assert.Empty(t, result.Added)
	require.Len(t, result.Changed, 1)
	assert.Equal(t, "001-F", result.Changed[0].ID)
	assert.False(t, result.FallbackUsed)

	got, err := db.GetItem(ctx, database, "001-F")
	require.NoError(t, err)
	assert.Equal(t, "Feature A — Updated", got.Title)
}

func TestMergeSync_DetectsAndRemovesDeletedArtifact(t *testing.T) {
	ws, database, manifest := setupSyncWorkspace(t)
	ctx := context.Background()

	require.NoError(t, os.Remove(filepath.Join(ws, "queue", "001-F.md")))

	result, _, err := db.MergeSync(ctx, ws, database, manifest, false)
	require.NoError(t, err)

	assert.Empty(t, result.Added)
	assert.Empty(t, result.Changed)
	require.Len(t, result.Deleted, 1)
	assert.Equal(t, "001-F", result.Deleted[0].ID)
	assert.False(t, result.FallbackUsed)

	// Item must no longer be in the database.
	_, err = db.GetItem(ctx, database, "001-F")
	assert.Error(t, err, "deleted artifact must be absent from the index")
}

func TestMergeSync_DetectsRelocationAsUpdate(t *testing.T) {
	ws, database, manifest := setupSyncWorkspace(t)
	ctx := context.Background()

	// Simulate a status-driven relocation: remove from queue/, add to done/.
	require.NoError(t, os.MkdirAll(filepath.Join(ws, "done"), 0o755))
	require.NoError(t, os.Rename(
		filepath.Join(ws, "queue", "001-F.md"),
		filepath.Join(ws, "done", "001-F.md"),
	))

	result, _, err := db.MergeSync(ctx, ws, database, manifest, false)
	require.NoError(t, err)

	assert.Empty(t, result.Added, "relocation must not appear in Added")
	assert.Empty(t, result.Deleted, "relocation must not appear in Deleted")
	require.Len(t, result.Relocated, 1)
	assert.Equal(t, "001-F", result.Relocated[0].ID)
	assert.False(t, result.FallbackUsed)
}

func TestMergeSync_StashChangeTriggersStashRefresh(t *testing.T) {
	ws, database, manifest := setupSyncWorkspace(t)
	ctx := context.Background()

	// Write a stash file so it appears as a new entry in the diff.
	stashContent := `{"id":"AABBCCDD","text":"test stash","priority":"medium","kind":"task","state":"active"}` + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(ws, "stash.jsonl"), []byte(stashContent), 0o644))

	result, _, err := db.MergeSync(ctx, ws, database, manifest, false)
	require.NoError(t, err)

	assert.True(t, result.StashRefreshed, "stash.jsonl change must trigger stash refresh")
}

func TestMergeSync_LogChangeTriggersLogRefresh(t *testing.T) {
	ws, database, manifest := setupSyncWorkspace(t)
	ctx := context.Background()

	require.NoError(t, os.MkdirAll(filepath.Join(ws, "logs"), 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(ws, "logs", "001-F.jsonl"),
		[]byte(`{"timestamp":"2026-04-21T00:00:00Z","actor":"test","event_type":"comment","content":"hi"}`+"\n"),
		0o644,
	))

	result, _, err := db.MergeSync(ctx, ws, database, manifest, false)
	require.NoError(t, err)

	assert.True(t, result.LogsRefreshed, "logs/*.jsonl change must trigger log refresh")
}

func TestMergeSync_LargeDeltaTriggersFallback(t *testing.T) {
	ws, database, manifest := setupSyncWorkspace(t)
	ctx := context.Background()

	// Add enough new files to exceed the fallback threshold.
	require.NoError(t, os.MkdirAll(filepath.Join(ws, "queue"), 0o755))
	for i := 0; i < 60; i++ {
		content := syncTestArtifactA
		name := filepath.Join(ws, "queue", "extra-"+string(rune('a'+i%26))+".md")
		require.NoError(t, os.WriteFile(name, []byte(content), 0o644))
	}

	result, _, err := db.MergeSync(ctx, ws, database, manifest, false)
	require.NoError(t, err)

	assert.True(t, result.FallbackUsed, "large delta must trigger fallback to full rehydrate")
	assert.NotEmpty(t, result.FallbackReason)
}

func TestMergeSync_DryRunDoesNotModifyDatabase(t *testing.T) {
	ws, database, manifest := setupSyncWorkspace(t)
	ctx := context.Background()

	require.NoError(t, os.WriteFile(filepath.Join(ws, "queue", "002-T.md"), []byte(syncTestArtifactB), 0o644))

	result, _, err := db.MergeSync(ctx, ws, database, manifest, true)
	require.NoError(t, err)

	assert.True(t, result.DryRun)
	require.Len(t, result.Added, 1, "dry-run must still report the diff")

	// The added item must NOT be in the database because dry-run skips writes.
	_, err = db.GetItem(ctx, database, "002-T")
	assert.Error(t, err, "dry-run must not write to the database")
}

func TestMergeSync_ReturnsUpdatedManifest(t *testing.T) {
	ws, database, manifest := setupSyncWorkspace(t)
	ctx := context.Background()

	require.NoError(t, os.WriteFile(filepath.Join(ws, "queue", "002-T.md"), []byte(syncTestArtifactB), 0o644))

	_, newManifest, err := db.MergeSync(ctx, ws, database, manifest, false)
	require.NoError(t, err)

	_, ok := newManifest["queue/002-T.md"]
	assert.True(t, ok, "updated manifest must include the newly added file")
}
