package core_test

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

func TestUpdateArtifact_MissingMarkdownPath_ReturnsErrorAndDoesNotUpsert(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	feature, err := core.CreateArtifact(ctx, ws, "Parent feature", "feature")
	require.NoError(t, err)
	task, err := core.CreateArtifact(ctx, ws, "Task", "task", core.WithParent(feature.ID))
	require.NoError(t, err)

	filePath, err := core.FindArtifactPath(ctx, ws, task.ID)
	require.NoError(t, err)
	require.NoError(t, os.Remove(filePath))

	_, err = core.UpdateArtifact(ctx, ws, task.ID, map[string]any{"status": "active"})
	require.Error(t, err)

	refreshed, getErr := db.GetItem(ctx, ws.DB, task.ID)
	require.NoError(t, getErr)
	assert.Equal(t, models.StatusQueued, refreshed.Status)
}

func TestBulkUpdateStatus_MissingMarkdownPath_ReturnsErrorWithoutDBDrift(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	feature, err := core.CreateArtifact(ctx, ws, "Parent feature", "feature")
	require.NoError(t, err)
	task, err := core.CreateArtifact(ctx, ws, "Task", "task", core.WithParent(feature.ID))
	require.NoError(t, err)

	filePath, err := core.FindArtifactPath(ctx, ws, task.ID)
	require.NoError(t, err)
	require.NoError(t, os.Remove(filePath))

	result, err := core.BulkUpdateStatus(ctx, ws.DB, ws, []string{task.ID}, "active")
	require.NoError(t, err)
	assert.Contains(t, result.Failed, task.ID, "missing-file item must appear in Failed slice")
	assert.Equal(t, 0, result.Succeeded, "nothing should succeed when the only item is missing")

	refreshed, getErr := db.GetItem(ctx, ws.DB, task.ID)
	require.NoError(t, getErr)
	assert.Equal(t, models.StatusQueued, refreshed.Status, "DB must not drift when Markdown write was skipped")
}

func TestUpdateArtifact_StatusChange_RelocatesToRegistryDirectory(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	feature, err := core.CreateArtifact(ctx, ws, "Feature to archive", "feature")
	require.NoError(t, err)

	updated, err := core.UpdateArtifact(ctx, ws, feature.ID, map[string]any{"status": "done"})
	require.NoError(t, err)
	assert.Equal(t, models.StatusDone, updated.Status)

	filePath, err := core.FindArtifactPath(ctx, ws, feature.ID)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(core.WorkspaceStorageRoot(ws.RootPath), "archive", filepath.Base(filePath)), filePath)
}
