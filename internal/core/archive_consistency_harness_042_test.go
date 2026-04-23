package core_test

// 041.003-T: Fix archive file/index consistency on partial failure
//
// Consistency contract after fix:
//   - If the DB status update fails after the file has been moved to the archive
//     directory, ArchiveItem must restore the file to its original path so the
//     filesystem and DB index remain consistent.
//   - A successful archive: file at archivePath, DB status = "archived",
//     frontmatter contains archived_from.
//   - After a simulated DB failure: file at originalPath (restored), DB status
//     unchanged (not "archived"), no stale archive file left behind.

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

// setupArchiveConsistencyWorkspace creates a workspace with one queued task.
func setupArchiveConsistencyWorkspace(t *testing.T) (*core.Workspace, string) {
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

	configData := []byte("artifact_types:\n  task:\n    prefix: T\n    suffix: \"-T\"\n    name_format: \"{NNN}{suffix}\"\nmax_slug_length: 60\n")
	require.NoError(t, os.WriteFile(filepath.Join(backlogDir, "config.yaml"), configData, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(backlogDir, "header-def.yaml"), []byte("defaults: {}\ntypes: {}\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(backlogDir, "registry.yaml"), []byte("routes: {}\n"), 0o644))

	content := "---\nid: 098-T\ntitle: Archive consistency task\nstatus: done\nartifact_type: task\n---\nBody\n"
	filePath := filepath.Join(queueDir, "098-T.md")
	require.NoError(t, os.WriteFile(filePath, []byte(content), 0o644))
	require.NoError(t, db.UpsertItem(context.Background(), database, &models.Artifact{
		ID: "098-T", Title: "Archive consistency task", Status: models.StatusDone, ArtifactType: "task",
	}))

	ws := &core.Workspace{RootPath: tmpDir, DB: database}
	return ws, filePath
}

// TestArchiveItem_Success_FileAtArchivePathDBUpdated verifies a clean archive
// path: file moves, DB reflects "archived" status.
func TestArchiveItem_Success_FileAtArchivePathDBUpdated(t *testing.T) {
	ws, originalPath := setupArchiveConsistencyWorkspace(t)
	ctx := context.Background()

	record, err := core.ArchiveItem(ctx, ws.DB, ws, "098-T")

	require.NoError(t, err)
	assert.NoFileExists(t, originalPath, "original file must be removed after archive")
	assert.FileExists(t, record.ArchivePath, "archive file must exist after archive")

	item, dbErr := db.GetItem(ctx, ws.DB, "098-T")
	require.NoError(t, dbErr)
	assert.Equal(t, models.StatusArchived, item.Status, "DB status must be archived")
}

// TestArchiveItem_DBFailureAfterFileMove_OriginalFileRestored verifies the key
// crash-safety contract: if the DB update fails after the file has moved,
// ArchiveItem restores the file to its original path so the workspace is
// consistent.
func TestArchiveItem_DBFailureAfterFileMove_OriginalFileRestored(t *testing.T) {
	ws, originalPath := setupArchiveConsistencyWorkspace(t)
	ctx := context.Background()

	// Close the DB so the UPDATE items SET status fails.
	ws.DB.Close()

	_, err := core.ArchiveItem(ctx, ws.DB, ws, "098-T")

	require.Error(t, err, "ArchiveItem must return an error when DB update fails")

	// The original file must be present at its original location.
	assert.FileExists(t, originalPath,
		"original file must be restored when DB update fails after file move")

	// No stale archive file should remain (it should have been cleaned up).
	backlogDir := core.WorkspaceStorageRoot(ws.RootPath)
	archivePath := filepath.Join(backlogDir, "archive", "098-T.md")
	assert.NoFileExists(t, archivePath,
		"stale archive file must be removed when DB update fails")
}

// TestArchiveItem_DBFailure_DBStatusUnchanged verifies that when DB update
// fails, the DB status remains in its pre-archive state (not "archived").
func TestArchiveItem_DBFailure_DBStatusUnchanged(t *testing.T) {
	// Use a new DB instance to check state after the failed archive.
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

	configData := []byte("artifact_types:\n  task:\n    prefix: T\n    suffix: \"-T\"\n    name_format: \"{NNN}{suffix}\"\nmax_slug_length: 60\n")
	require.NoError(t, os.WriteFile(filepath.Join(backlogDir, "config.yaml"), configData, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(backlogDir, "header-def.yaml"), []byte("defaults: {}\ntypes: {}\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(backlogDir, "registry.yaml"), []byte("routes: {}\n"), 0o644))

	content := "---\nid: 097-T\ntitle: DB failure status check\nstatus: done\nartifact_type: task\n---\nBody\n"
	require.NoError(t, os.WriteFile(filepath.Join(queueDir, "097-T.md"), []byte(content), 0o644))
	require.NoError(t, db.UpsertItem(context.Background(), database, &models.Artifact{
		ID: "097-T", Title: "DB failure status check", Status: models.StatusDone, ArtifactType: "task",
	}))

	ws := &core.Workspace{RootPath: tmpDir, DB: database}
	ctx := context.Background()

	// Close DB to force failure.
	database.Close()

	_, archErr := core.ArchiveItem(ctx, ws.DB, ws, "097-T")
	require.Error(t, archErr)

	// Re-open DB and check status is unchanged.
	database2, openErr := db.Open(dbPath)
	require.NoError(t, openErr)
	defer database2.Close()

	item, getErr := db.GetItem(ctx, database2, "097-T")
	require.NoError(t, getErr)
	assert.NotEqual(t, models.StatusArchived, item.Status,
		"DB status must not be 'archived' when the update failed")
}
