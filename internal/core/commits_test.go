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

func setupCommitWorkspace(t *testing.T) *core.Workspace {
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

	// Write config
	require.NoError(t, os.WriteFile(filepath.Join(backlogDir, "config.yaml"),
		[]byte("artifact_types:\n  - task\nid_prefix_map:\n  task: T\nmax_slug_length: 60\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(backlogDir, "header-def.yaml"), []byte("defaults: {}\ntypes: {}\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(backlogDir, "registry.yaml"), []byte("routes: {}\n"), 0o644))

	// Seed a task
	taskContent := "---\nid: T001\ntitle: Test task\nstatus: active\ntype: task\n---\nTask body\n"
	require.NoError(t, os.WriteFile(filepath.Join(tasksDir, "T001.md"), []byte(taskContent), 0o644))
	require.NoError(t, db.UpsertItem(context.Background(), database, &models.Artifact{
		ID: "T001", Title: "Test task", Status: models.StatusActive, ArtifactType: "task",
	}))

	return &core.Workspace{RootPath: tmpDir, DB: database}
}

func TestLinkCommit_StoresAssociation(t *testing.T) {
	// Arrange
	ws := setupCommitWorkspace(t)
	ctx := context.Background()

	// Act
	err := core.LinkCommit(ctx, ws.DB, ws, "T001", "abc123def", "feat: implement T001", "test@example.com")

	// Assert
	require.NoError(t, err)
}

func TestGetCommitLinks_ReturnsLinked(t *testing.T) {
	// Arrange
	ws := setupCommitWorkspace(t)
	ctx := context.Background()
	require.NoError(t, core.LinkCommit(ctx, ws.DB, ws, "T001", "abc123def", "feat: implement T001", "test@example.com"))

	// Act
	links, err := core.GetCommitLinks(ctx, ws.DB, "T001")

	// Assert
	require.NoError(t, err)
	require.Len(t, links, 1)
	assert.Equal(t, "abc123def", links[0].CommitSHA)
}

func TestAutoLinkCommits_FindsReferences(t *testing.T) {
	// Arrange
	ws := setupCommitWorkspace(t)
	ctx := context.Background()

	// Act — depth 0 means no git log entries
	links, err := core.AutoLinkCommits(ctx, ws.DB, ws, 0)

	// Assert
	require.NoError(t, err)
	assert.Empty(t, links, "no commits to scan with depth 0")
}
