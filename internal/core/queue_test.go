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

func TestMoveInQueue_ReordersItems(t *testing.T) {
	// Arrange
	ws := setupQueueWorkspace(t)
	ctx := context.Background()

	// Act — move T003 to position 0
	err := core.MoveInQueue(ctx, ws.DB, "T003", 0)

	// Assert
	require.NoError(t, err)
}

func TestBulkUpdateStatus(t *testing.T) {
	// Arrange
	ws := setupQueueWorkspace(t)
	ctx := context.Background()

	// Act
	count, err := core.BulkUpdateStatus(ctx, ws.DB, ws, []string{"T001", "B001"}, "active")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 2, count, "should update both queued items")
}
