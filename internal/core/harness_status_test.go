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

func defaultStatusConfig() *core.StatusConfig {
	return &core.StatusConfig{
		ValidStatuses: []string{"queued", "active", "blocked", "review", "done", "accepted", "rejected"},
		Transitions: []core.StatusTransitionRule{
			{From: "queued", To: "active"},
			{From: "active", To: "review"},
			{From: "active", To: "blocked"},
			{From: "review", To: "done"},
			{From: "blocked", To: "active"},
			{From: "done", To: "accepted"},
			{From: "done", To: "rejected"},
		},
	}
}

func TestValidateTransition(t *testing.T) {
	tests := []struct {
		name    string
		from    string
		to      string
		wantErr bool
	}{
		{name: "valid queued to active", from: "queued", to: "active", wantErr: false},
		{name: "valid active to review", from: "active", to: "review", wantErr: false},
		{name: "invalid queued to done", from: "queued", to: "done", wantErr: true},
		{name: "invalid done to queued", from: "done", to: "queued", wantErr: true},
		{name: "same status noop", from: "active", to: "active", wantErr: false},
	}

	cfg := defaultStatusConfig()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := core.ValidateTransition(cfg, "task", tt.from, tt.to)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func setupStatusWorkspace(t *testing.T) *core.Workspace {
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
		[]byte("artifact_types:\n  - task\nid_prefix_map:\n  task: T\nmax_slug_length: 60\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(backlogDir, "header-def.yaml"), []byte("defaults: {}\ntypes: {}\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(backlogDir, "registry.yaml"), []byte("routes: {}\n"), 0o644))

	ctx := context.Background()
	// Parent with two children
	require.NoError(t, db.UpsertItem(ctx, database, &models.Artifact{
		ID: "T001", Title: "Parent", Status: models.StatusActive, ArtifactType: "task",
	}))
	require.NoError(t, db.UpsertItem(ctx, database, &models.Artifact{
		ID: "T002", Title: "Child A", Status: models.StatusDone, ArtifactType: "task", ParentID: "T001",
	}))
	require.NoError(t, db.UpsertItem(ctx, database, &models.Artifact{
		ID: "T003", Title: "Child B", Status: models.StatusDone, ArtifactType: "task", ParentID: "T001",
	}))

	return &core.Workspace{RootPath: tmpDir, DB: database}
}

func TestComputeParentStatus_AllChildrenDone(t *testing.T) {
	// Arrange
	ws := setupStatusWorkspace(t)
	ctx := context.Background()

	// Act
	status, err := core.ComputeParentStatus(ctx, ws.DB, "T001")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, models.StatusDone, status, "all children done → parent done")
}

func TestCascadeStatusUpdate(t *testing.T) {
	// Arrange
	ws := setupStatusWorkspace(t)
	ctx := context.Background()

	// Act — cascade from child T002 upward
	err := core.CascadeStatusUpdate(ctx, ws.DB, ws, "T002")

	// Assert
	require.NoError(t, err)
}
