package core_test

// 026.010-T: Write-path invariant integration tests.
//
// These tests verify the CQRS write-path invariant: the Markdown file is the
// source of truth. After each write operation, the SQLite index is deleted and
// rebuilt via Rehydrate. The resulting state must match the original write.
//
//   - TestWriteInvariant_UpdateThenRehydrate_StateConsistent
//   - TestWriteInvariant_BulkUpdateThenRehydrate_StateConsistent
//   - TestWriteInvariant_MoveThenRehydrate_FileInCorrectDirectory

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/core"
	"github.com/backlogit/backlogit/internal/db"
	"github.com/backlogit/backlogit/internal/models"
)

// TestWriteInvariant_UpdateThenRehydrate_StateConsistent creates an artifact,
// updates it via UpdateArtifact, wipes the SQLite index, rehydrates from disk,
// and verifies that the DB state matches the Markdown file.
func TestWriteInvariant_UpdateThenRehydrate_StateConsistent(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	feature, err := core.CreateArtifact(ctx, ws, "Invariant feature", "feature")
	require.NoError(t, err)

	// Update several fields to ensure they all survive a rehydration cycle.
	updated, err := core.UpdateArtifact(ctx, ws, feature.ID, map[string]any{
		"status":      "active",
		"assigned_to": "alice",
		"priority":    "high",
	})
	require.NoError(t, err)
	assert.Equal(t, models.StatusActive, updated.Status)

	// Wipe the DB index to force rehydration from Markdown.
	_, wipeErr := ws.DB.ExecContext(ctx, `DELETE FROM items`)
	require.NoError(t, wipeErr)

	count, rehydrateErr := db.Rehydrate(ctx, core.WorkspaceStorageRoot(ws.RootPath), ws.DB)
	require.NoError(t, rehydrateErr)
	assert.Positive(t, count, "rehydration must index at least one artifact")

	// Fetch from the rebuilt index and verify it matches the Markdown state.
	refreshed, getErr := db.GetItem(ctx, ws.DB, feature.ID)
	require.NoError(t, getErr, "artifact must be findable after rehydration")
	assert.Equal(t, models.StatusActive, refreshed.Status,
		"status must be consistent between Markdown and DB after rehydration")
	assert.Equal(t, "alice", refreshed.AssignedTo,
		"assigned_to must be consistent between Markdown and DB after rehydration")
	assert.Equal(t, "high", refreshed.Priority,
		"priority must be consistent between Markdown and DB after rehydration")
}

// TestWriteInvariant_BulkUpdateThenRehydrate_StateConsistent bulk-updates
// multiple artifacts, wipes the DB, and verifies that rehydration restores the
// correct statuses for all affected items.
func TestWriteInvariant_BulkUpdateThenRehydrate_StateConsistent(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	feature, err := core.CreateArtifact(ctx, ws, "Bulk invariant feature", "feature")
	require.NoError(t, err)
	task1, err := core.CreateArtifact(ctx, ws, "Bulk task one", "task", core.WithParent(feature.ID))
	require.NoError(t, err)
	task2, err := core.CreateArtifact(ctx, ws, "Bulk task two", "task", core.WithParent(feature.ID))
	require.NoError(t, err)

	result, err := core.BulkUpdateStatus(ctx, ws.DB, ws, []string{task1.ID, task2.ID}, "active")
	require.NoError(t, err)
	require.Equal(t, 2, result.Succeeded, "both tasks must be updated")
	require.Empty(t, result.Failed)

	// Wipe the DB and rebuild from Markdown.
	_, wipeErr := ws.DB.ExecContext(ctx, `DELETE FROM items`)
	require.NoError(t, wipeErr)

	_, rehydrateErr := db.Rehydrate(ctx, core.WorkspaceStorageRoot(ws.RootPath), ws.DB)
	require.NoError(t, rehydrateErr)

	for _, taskID := range []string{task1.ID, task2.ID} {
		item, getErr := db.GetItem(ctx, ws.DB, taskID)
		require.NoError(t, getErr, "task %s must be findable after rehydration", taskID)
		assert.Equal(t, models.StatusActive, item.Status,
			"task %s status must be active after bulk update and rehydration", taskID)
	}
}

// TestWriteInvariant_MoveThenRehydrate_FileInCorrectDirectory moves a feature
// to "done" (which triggers relocation to the archive directory), wipes the DB,
// rehydrates, and verifies the artifact is found in the archive directory with
// the correct status.
func TestWriteInvariant_MoveThenRehydrate_FileInCorrectDirectory(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	feature, err := core.CreateArtifact(ctx, ws, "Feature to archive", "feature")
	require.NoError(t, err)

	// Move the feature to "done" — this relocates the file to archive/.
	moved, err := core.UpdateArtifact(ctx, ws, feature.ID, map[string]any{"status": "done"})
	require.NoError(t, err)
	assert.Equal(t, models.StatusDone, moved.Status)

	// Verify the Markdown file is in the archive directory.
	filePath, pathErr := core.FindArtifactPath(ctx, ws, feature.ID)
	require.NoError(t, pathErr)
	storageRoot := core.WorkspaceStorageRoot(ws.RootPath)
	archiveDir := filepath.Join(storageRoot, "archive")
	assert.Equal(t, archiveDir, filepath.Dir(filePath),
		"file must reside in the archive directory after moving to done")

	// Wipe the DB and rebuild from Markdown — the archive directory must be scanned.
	_, wipeErr := ws.DB.ExecContext(ctx, `DELETE FROM items`)
	require.NoError(t, wipeErr)

	count, rehydrateErr := db.Rehydrate(ctx, storageRoot, ws.DB)
	require.NoError(t, rehydrateErr)
	assert.Positive(t, count, "rehydration must index archived artifact")

	refreshed, getErr := db.GetItem(ctx, ws.DB, feature.ID)
	require.NoError(t, getErr, "archived artifact must be findable after rehydration")
	assert.Equal(t, models.StatusDone, refreshed.Status,
		"status must remain done after relocation and rehydration")

	// Confirm the file is still physically in the archive directory.
	finalPath, finalPathErr := core.FindArtifactPath(ctx, ws, feature.ID)
	require.NoError(t, finalPathErr)
	assert.Equal(t, archiveDir, filepath.Dir(finalPath),
		"file must still be in archive after rehydration")

	// Guard against file duplication: no copy should exist in the queue dir.
	queueDir := filepath.Join(storageRoot, "queue")
	entries, readErr := os.ReadDir(queueDir)
	require.NoError(t, readErr)
	for _, e := range entries {
		assert.NotEqual(t, filepath.Base(filePath), e.Name(),
			"moved artifact must not exist in queue directory after relocation")
	}
}
