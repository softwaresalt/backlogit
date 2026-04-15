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

func setupArchiveWorkspace(t *testing.T) *core.Workspace {
	t.Helper()
	tmpDir := t.TempDir()
	backlogDir := filepath.Join(tmpDir, ".backlogit")
	queueDir := filepath.Join(backlogDir, "queue")
	archiveDir := filepath.Join(backlogDir, "archive")
	require.NoError(t, os.MkdirAll(queueDir, 0o755))
	require.NoError(t, os.MkdirAll(archiveDir, 0o755))

	dbPath := filepath.Join(backlogDir, "backlogit.db")
	database, err := db.Open(dbPath)
	require.NoError(t, err)
	require.NoError(t, db.EnsureSchema(database))
	t.Cleanup(func() { database.Close() })

	// Write config.yaml
	configData := []byte("artifact_types:\n  task:\n    prefix: T\n    suffix: \"-T\"\n    name_format: \"{NNN}{suffix}\"\n  bug:\n    prefix: B\n    suffix: \"-B\"\n    name_format: \"{NNN}{suffix}\"\nmax_slug_length: 60\n")
	require.NoError(t, os.WriteFile(filepath.Join(backlogDir, "config.yaml"), configData, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(backlogDir, "header-def.yaml"), []byte("defaults: {}\ntypes: {}\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(backlogDir, "registry.yaml"), []byte("routes: {}\n"), 0o644))

	// Seed a completed task file
	taskContent := "---\nid: 001-T\ntitle: Completed task\nstatus: done\nartifact_type: task\n---\nDone task body\n"
	require.NoError(t, os.WriteFile(filepath.Join(queueDir, "001-T.md"), []byte(taskContent), 0o644))
	require.NoError(t, db.UpsertItem(context.Background(), database, &models.Artifact{
		ID: "001-T", Title: "Completed task", Status: models.StatusDone, ArtifactType: "task",
	}))

	return &core.Workspace{RootPath: tmpDir, DB: database}
}

func TestArchiveItem_MovesToArchive(t *testing.T) {
	// Arrange
	ws := setupArchiveWorkspace(t)
	ctx := context.Background()

	// Act
	record, err := core.ArchiveItem(ctx, ws.DB, ws, "001-T")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "001-T", record.ID)
	assert.NotEmpty(t, record.ArchivePath)
	assert.FileExists(t, record.ArchivePath)

	raw, readErr := os.ReadFile(record.ArchivePath)
	require.NoError(t, readErr)
	fm, _, parseErr := models.ParseFrontmatter(string(raw))
	require.NoError(t, parseErr)
	assert.Equal(t, ".backlogit/queue/001-T.md", fm["archived_from"])
}

func TestUnarchiveItem_RestoresFromArchive(t *testing.T) {
	// Arrange
	ws := setupArchiveWorkspace(t)
	ctx := context.Background()
	_, err := core.ArchiveItem(ctx, ws.DB, ws, "001-T")
	require.NoError(t, err)

	// Act
	err = core.UnarchiveItem(ctx, ws.DB, ws, "001-T")

	// Assert
	require.NoError(t, err)
	originalPath := filepath.Join(ws.RootPath, ".backlogit", "queue", "001-T.md")
	assert.FileExists(t, originalPath)
}

func TestArchiveItem_ExcludedFromDefaultList(t *testing.T) {
	// GIVEN an archived item in the workspace
	ws := setupArchiveWorkspace(t)
	ctx := context.Background()
	_, err := core.ArchiveItem(ctx, ws.DB, ws, "001-T")
	require.NoError(t, err)

	// WHEN querying with the default (empty) filters
	items, err := db.QueryItems(ctx, ws.DB, db.QueryFilters{})

	// THEN the archived item must not appear
	require.NoError(t, err)
	for _, item := range items {
		assert.NotEqual(t, "001-T", item.ID, "archived item 001-T must be excluded from default query")
	}
}

func TestArchiveItem_IncludedWhenExplicitlyRequested(t *testing.T) {
	// GIVEN an archived item
	ws := setupArchiveWorkspace(t)
	ctx := context.Background()
	_, err := core.ArchiveItem(ctx, ws.DB, ws, "001-T")
	require.NoError(t, err)

	// WHEN querying with IncludeArchived: true
	items, err := db.QueryItems(ctx, ws.DB, db.QueryFilters{IncludeArchived: true})

	// THEN the archived item must be present
	require.NoError(t, err)
	found := false
	for _, item := range items {
		if item.ID == "001-T" {
			found = true
			assert.Equal(t, models.StatusArchived, item.Status)
		}
	}
	assert.True(t, found, "archived item 001-T must appear when IncludeArchived is true")
}

func TestUnarchiveItem_RestoresSuffixedFilenameByFrontmatterID(t *testing.T) {
	ws := setupArchiveWorkspace(t)
	ctx := context.Background()

	originalPath := filepath.Join(ws.RootPath, ".backlogit", "queue", "001-T.md")
	suffixedPath := filepath.Join(ws.RootPath, ".backlogit", "queue", "001-T-completed-task.md")
	require.NoError(t, os.Rename(originalPath, suffixedPath))

	record, err := core.ArchiveItem(ctx, ws.DB, ws, "001-T")
	require.NoError(t, err)
	assert.FileExists(t, record.ArchivePath)

	err = core.UnarchiveItem(ctx, ws.DB, ws, "001-T")

	require.NoError(t, err)
	assert.FileExists(t, suffixedPath)
}

func TestUnarchiveItem_RejectsActiveArtifact(t *testing.T) {
	ws := setupArchiveWorkspace(t)
	ctx := context.Background()

	err := core.UnarchiveItem(ctx, ws.DB, ws, "001-T")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "is not archived")
}

// TestArchiveItem_ItemAlreadyInArchiveDir verifies that ArchiveItem does not
// delete the archive file when the item's current path is already inside the
// archive directory. This can happen when a registry routes a terminal status
// (e.g. "done") to the archive/ directory: completeReleaseScope moves the file
// there first, then archiveItems calls ArchiveItem a second time on the same
// item. Without the same-path guard, os.Remove(currentPath) would delete the
// file that was just written.
func TestArchiveItem_ItemAlreadyInArchiveDir(t *testing.T) {
	ws := setupArchiveWorkspace(t)
	ctx := context.Background()

	backlogDir := filepath.Join(ws.RootPath, ".backlogit")
	archiveDir := filepath.Join(backlogDir, "archive")
	queueDir := filepath.Join(backlogDir, "queue")

	// Seed item directly in archive/ (simulating status-routing move)
	alreadyArchivedContent := "---\nid: 002-T\ntitle: Already archived task\nstatus: done\nartifact_type: task\n---\nBody\n"
	archiveFilePath := filepath.Join(archiveDir, "002-T.md")
	require.NoError(t, os.WriteFile(archiveFilePath, []byte(alreadyArchivedContent), 0o644))
	require.NoError(t, db.UpsertItem(ctx, ws.DB, &models.Artifact{
		ID: "002-T", Title: "Already archived task", Status: models.StatusDone, ArtifactType: "task",
	}))

	// Act: ArchiveItem on an item already in archive/
	record, err := core.ArchiveItem(ctx, ws.DB, ws, "002-T")

	// Assert: no error, archive file still exists, queue file not created
	require.NoError(t, err)
	assert.FileExists(t, archiveFilePath, "archive file must survive ArchiveItem when already in archive/")
	assert.Equal(t, archiveFilePath, record.ArchivePath)
	assert.NoFileExists(t, filepath.Join(queueDir, "002-T.md"), "no queue file should be created")
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
