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

func setupArchiveWorkspace(t *testing.T) *core.Workspace {
	t.Helper()
	tmpDir := t.TempDir()
	backlogDir := filepath.Join(tmpDir, ".backlogit")
	tasksDir := filepath.Join(backlogDir, "tasks")
	archiveDir := filepath.Join(backlogDir, "archive")
	require.NoError(t, os.MkdirAll(tasksDir, 0o755))
	require.NoError(t, os.MkdirAll(archiveDir, 0o755))

	dbPath := filepath.Join(backlogDir, "backlogit.db")
	database, err := db.Open(dbPath)
	require.NoError(t, err)
	require.NoError(t, db.EnsureSchema(database))
	t.Cleanup(func() { database.Close() })

	// Write config.yaml
	configData := []byte("artifact_types:\n  - task\n  - bug\nid_prefix_map:\n  task: T\n  bug: B\nmax_slug_length: 60\n")
	require.NoError(t, os.WriteFile(filepath.Join(backlogDir, "config.yaml"), configData, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(backlogDir, "header-def.yaml"), []byte("defaults: {}\ntypes: {}\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(backlogDir, "registry.yaml"), []byte("routes: {}\n"), 0o644))

	// Seed a completed task file
	taskContent := "---\nid: T001\ntitle: Completed task\nstatus: done\nartifact_type: task\n---\nDone task body\n"
	require.NoError(t, os.WriteFile(filepath.Join(tasksDir, "T001.md"), []byte(taskContent), 0o644))
	require.NoError(t, db.UpsertItem(context.Background(), database, &models.Artifact{
		ID: "T001", Title: "Completed task", Status: models.StatusDone, ArtifactType: "task",
	}))

	return &core.Workspace{RootPath: tmpDir, DB: database}
}

func TestArchiveItem_MovesToArchive(t *testing.T) {
	// Arrange
	ws := setupArchiveWorkspace(t)
	ctx := context.Background()

	// Act
	record, err := core.ArchiveItem(ctx, ws.DB, ws, "T001")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "T001", record.ID)
	assert.NotEmpty(t, record.ArchivePath)
	assert.FileExists(t, record.ArchivePath)
}

func TestUnarchiveItem_RestoresFromArchive(t *testing.T) {
	// Arrange
	ws := setupArchiveWorkspace(t)
	ctx := context.Background()
	_, err := core.ArchiveItem(ctx, ws.DB, ws, "T001")
	require.NoError(t, err)

	// Act
	err = core.UnarchiveItem(ctx, ws.DB, ws, "T001")

	// Assert
	require.NoError(t, err)
	originalPath := filepath.Join(ws.RootPath, ".backlogit", "tasks", "T001.md")
	assert.FileExists(t, originalPath)
}

func TestArchiveItem_ExcludedFromDefaultList(t *testing.T) {
	// GIVEN an archived item in the workspace
	ws := setupArchiveWorkspace(t)
	ctx := context.Background()
	_, err := core.ArchiveItem(ctx, ws.DB, ws, "T001")
	require.NoError(t, err)

	// WHEN querying with the default (empty) filters
	items, err := db.QueryItems(ctx, ws.DB, db.QueryFilters{})

	// THEN the archived item must not appear
	require.NoError(t, err)
	for _, item := range items {
		assert.NotEqual(t, "T001", item.ID, "archived item T001 must be excluded from default query")
	}
}

func TestArchiveItem_IncludedWhenExplicitlyRequested(t *testing.T) {
	// GIVEN an archived item
	ws := setupArchiveWorkspace(t)
	ctx := context.Background()
	_, err := core.ArchiveItem(ctx, ws.DB, ws, "T001")
	require.NoError(t, err)

	// WHEN querying with IncludeArchived: true
	items, err := db.QueryItems(ctx, ws.DB, db.QueryFilters{IncludeArchived: true})

	// THEN the archived item must be present
	require.NoError(t, err)
	found := false
	for _, item := range items {
		if item.ID == "T001" {
			found = true
			assert.Equal(t, models.StatusArchived, item.Status)
		}
	}
	assert.True(t, found, "archived item T001 must appear when IncludeArchived is true")
}

func TestAutoArchive_ProcessesExpiredItems(t *testing.T) {
	// Arrange
	ws := setupArchiveWorkspace(t)
	ctx := context.Background()
	policy := &core.ArchivePolicy{
		TerminalStatuses: []string{"done", "accepted", "rejected"},
		RetentionDays:    0, // archive immediately
		ArchiveDir:       "archive",
	}

	// Act
	count, err := core.AutoArchive(ctx, ws.DB, ws, policy)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 1, count, "should archive the one completed task")
}
