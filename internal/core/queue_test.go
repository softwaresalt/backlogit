package core_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/models"
)

func setupQueueWorkspace(t *testing.T) *core.Workspace {
	t.Helper()
	tmpDir := t.TempDir()
	backlogDir := filepath.Join(tmpDir, ".backlogit")
	tasksDir := filepath.Join(backlogDir, "tasks")
	require.NoError(t, os.MkdirAll(tasksDir, 0o755))

	dbPath := filepath.Join(backlogDir, "backlogit.db")
	database, err := db.Open(dbPath)
	require.NoError(t, err)
	require.NoError(t, db.EnsureSchema(database))
	t.Cleanup(func() { database.Close() })

	require.NoError(t, os.WriteFile(filepath.Join(backlogDir, "config.yaml"),
		[]byte("artifact_types:\n  - task\n  - bug\nid_prefix_map:\n  task: T\n  bug: B\nmax_slug_length: 60\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(backlogDir, "header-def.yaml"), []byte("defaults: {}\ntypes: {}\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(backlogDir, "registry.yaml"), []byte("routes: {}\n"), 0o644))

	ctx := context.Background()
	items := []*models.Artifact{
		{ID: "T001", Title: "Task A", Status: models.StatusQueued, ArtifactType: "task", Priority: "high"},
		{ID: "T002", Title: "Task B", Status: models.StatusActive, ArtifactType: "task", Priority: "medium"},
		{ID: "T003", Title: "Task C", Status: models.StatusDone, ArtifactType: "task", Priority: "low"},
		{ID: "B001", Title: "Bug A", Status: models.StatusQueued, ArtifactType: "bug", Priority: "high"},
	}
	for _, item := range items {
		require.NoError(t, db.UpsertItem(ctx, database, item))
	}

	return &core.Workspace{RootPath: tmpDir, DB: database}
}

func TestQueryQueue_FilterByStatus(t *testing.T) {
	// Arrange
	ws := setupQueueWorkspace(t)
	ctx := context.Background()
	filter := &core.QueueFilter{Statuses: []string{"queued"}}

	// Act
	view, err := core.QueryQueue(ctx, ws.DB, filter)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 2, view.TotalCount, "should return T001 and B001")
}

func TestQueryQueue_GroupByType(t *testing.T) {
	// Arrange
	ws := setupQueueWorkspace(t)
	ctx := context.Background()
	filter := &core.QueueFilter{GroupBy: "type"}

	// Act
	view, err := core.QueryQueue(ctx, ws.DB, filter)

	// Assert
	require.NoError(t, err)
	assert.NotEmpty(t, view.Groups, "should have groups when GroupBy is set")
}

func TestQueryQueue_IgnoresBlankTypeAndStatusFilters(t *testing.T) {
	// Arrange
	ws := setupQueueWorkspace(t)
	ctx := context.Background()
	filter := &core.QueueFilter{
		Types:    []string{""},
		Statuses: []string{""},
	}

	// Act
	view, err := core.QueryQueue(ctx, ws.DB, filter)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 4, view.TotalCount, "blank filters should behave like no filter")
}

func TestMoveInQueue_ReordersAndSurvivesRehydrate(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	low, err := core.CreateArtifact(ctx, ws, "Low priority task", "feature", core.WithPriority("low"))
	require.NoError(t, err)
	high, err := core.CreateArtifact(ctx, ws, "High priority task", "feature", core.WithPriority("high"))
	require.NoError(t, err)

	filter := &core.QueueFilter{Statuses: []string{"queued"}, SortBy: "priority"}

	initial, err := core.QueryQueue(ctx, ws.DB, filter)
	require.NoError(t, err)
	require.Len(t, initial.Items, 2)
	assert.Equal(t, high.ID, initial.Items[0].ID)
	assert.Equal(t, low.ID, initial.Items[1].ID)

	err = core.MoveInQueue(ctx, ws, low.ID, 1, filter)
	require.NoError(t, err)

	reordered, err := core.QueryQueue(ctx, ws.DB, filter)
	require.NoError(t, err)
	require.Len(t, reordered.Items, 2)
	assert.Equal(t, low.ID, reordered.Items[0].ID)
	assert.Equal(t, high.ID, reordered.Items[1].ID)

	_, err = db.Rehydrate(ctx, core.WorkspaceStorageRoot(ws.RootPath), ws.DB)
	require.NoError(t, err)

	rehydrated, err := core.QueryQueue(ctx, ws.DB, filter)
	require.NoError(t, err)
	require.Len(t, rehydrated.Items, 2)
	assert.Equal(t, low.ID, rehydrated.Items[0].ID)
	assert.Equal(t, high.ID, rehydrated.Items[1].ID)
}

func TestMoveInQueue_RejectsInvalidPosition(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	artifact, err := core.CreateArtifact(ctx, ws, "Queue item", "feature")
	require.NoError(t, err)

	err = core.MoveInQueue(ctx, ws, artifact.ID, 0, &core.QueueFilter{Statuses: []string{"queued"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "position must be >=")
}

func TestBulkUpdateStatus(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()
	feature, err := core.CreateArtifact(ctx, ws, "Bulk update feature", "feature")
	require.NoError(t, err)
	task, err := core.CreateArtifact(ctx, ws, "Bulk update task", "task", core.WithParent(feature.ID))
	require.NoError(t, err)

	// Act
	result, err := core.BulkUpdateStatus(ctx, ws.DB, ws, []string{task.ID}, "active")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 1, result.Succeeded, "should update the queued item")
	assert.Empty(t, result.Failed)

	updated, err := db.GetItem(ctx, ws.DB, task.ID)
	require.NoError(t, err)
	assert.Equal(t, models.StatusActive, updated.Status)
}

func TestBulkUpdateStatus_MarkdownFirst_WritesBeforeDB(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	feat, err := core.CreateArtifact(ctx, ws, "Feature to archive", "feature")
	require.NoError(t, err)

	result, err := core.BulkUpdateStatus(ctx, ws.DB, ws, []string{feat.ID}, "done")

	require.NoError(t, err)
	assert.Equal(t, 1, result.Succeeded)
	assert.Empty(t, result.Failed)

	// Markdown file must reflect the new status.
	filePath, pathErr := core.FindArtifactPath(ctx, ws, feat.ID)
	require.NoError(t, pathErr)
	data, readErr := os.ReadFile(filePath)
	require.NoError(t, readErr)
	assert.Contains(t, string(data), "status: done", "Markdown must be updated")

	// DB index must also reflect the new status.
	item, getErr := db.GetItem(ctx, ws.DB, feat.ID)
	require.NoError(t, getErr)
	assert.Equal(t, models.StatusDone, item.Status, "DB index must match Markdown after bulk update")
}

func TestBulkUpdateStatus_PartialFailure_ReportsAccurately(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	feat, err := core.CreateArtifact(ctx, ws, "Parent feature", "feature")
	require.NoError(t, err)
	task, err := core.CreateArtifact(ctx, ws, "Orphaned task", "task", core.WithParent(feat.ID))
	require.NoError(t, err)

	// Remove the task's Markdown file to simulate a missing artifact on disk.
	filePath, pathErr := core.FindArtifactPath(ctx, ws, task.ID)
	require.NoError(t, pathErr)
	require.NoError(t, os.Remove(filePath))

	result, err := core.BulkUpdateStatus(ctx, ws.DB, ws, []string{feat.ID, task.ID}, "done")

	require.NoError(t, err, "partial failure must not propagate as a function error")
	assert.Equal(t, 1, result.Succeeded, "feature without missing file should succeed")
	assert.Len(t, result.Failed, 1, "task with missing file should be reported as failed")
	assert.Contains(t, result.Failed, task.ID)
}
