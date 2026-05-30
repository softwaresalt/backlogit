package core_test

// 060.002-T: Resolve duplicate archive source paths
// 060.003-T: Restore status during unarchive
//
// 060.002-T contracts:
//   - When an item exists in both the queue and archive directories,
//     ArchiveItem must prefer the queue copy as the canonical source,
//     complete the archive successfully, and leave no stale queue copy.
//
// 060.003-T contracts:
//   - ArchiveItem must persist the pre-archive status as "archived_status"
//     in frontmatter so UnarchiveItem can restore it.
//   - UnarchiveItem must restore "status" to the pre-archive value (not leave
//     it as "archived"). If no archived_status is present, default to "queued".
//   - After unarchive the item must appear in normal active list flows.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/models"
)

// setupDuplicateArchiveWorkspace creates a workspace where an item exists in
// both the queue directory (canonical) and the archive directory (stale copy
// from a previously half-completed archive operation).
func setupDuplicateArchiveWorkspace(t *testing.T) (*core.Workspace, string, string) {
	return setupDuplicateArchiveWorkspaceWithQueueRoot(t, "queue")
}

func setupDuplicateArchiveWorkspaceWithQueueRoot(t *testing.T, queueRoot string) (*core.Workspace, string, string) {
	t.Helper()
	tmpDir := t.TempDir()
	backlogDir := filepath.Join(tmpDir, ".backlogit")
	queueDir := filepath.Join(backlogDir, filepath.FromSlash(queueRoot))
	archiveDir := filepath.Join(backlogDir, "archive")
	require.NoError(t, os.MkdirAll(queueDir, 0o755))
	require.NoError(t, os.MkdirAll(archiveDir, 0o755))

	dbPath := filepath.Join(backlogDir, "backlogit.db")
	database, err := db.Open(dbPath)
	require.NoError(t, err)
	require.NoError(t, db.EnsureSchema(database))
	t.Cleanup(func() { database.Close() })

	configYAML := fmt.Sprintf("artifact_types:\n  task:\n    prefix: T\n    suffix: \"-T\"\n    name_format: \"{NNN}{suffix}\"\nmax_slug_length: 60\nqueue_layout:\n  root_dir: %s\n", queueRoot)
	require.NoError(t, os.WriteFile(filepath.Join(backlogDir, "config.yaml"), []byte(configYAML), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(backlogDir, "header-def.yaml"), []byte("defaults: {}\ntypes: {}\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(backlogDir, "registry.yaml"), []byte("directories:\n  - path: archive\n    condition:\n      status: [archived]\n"), 0o644))

	const itemID = "077-T"
	// Queue copy — the canonical unarchived state.
	queueContent := "---\nid: 077-T\ntitle: Duplicate source test\nstatus: done\nartifact_type: task\n---\nBody\n"
	queuePath := filepath.Join(queueDir, itemID+".md")
	require.NoError(t, os.WriteFile(queuePath, []byte(queueContent), 0o644))

	// Stale archive copy — left behind by a previously interrupted archive.
	archiveContent := fmt.Sprintf("---\nid: 077-T\ntitle: Duplicate source test\nstatus: archived\narchived_from: %s/077-T.md\nartifact_type: task\n---\nBody\n", filepath.ToSlash(queueRoot))
	archivePath := filepath.Join(archiveDir, itemID+".md")
	require.NoError(t, os.WriteFile(archivePath, []byte(archiveContent), 0o644))

	require.NoError(t, db.UpsertItem(context.Background(), database, &models.Artifact{
		ID: itemID, Title: "Duplicate source test", Status: models.StatusDone, ArtifactType: "task",
	}))

	ws := &core.Workspace{
		RootPath: tmpDir,
		DB:       database,
		Config: &config.WorkspaceConfig{
			QueueLayout: &config.QueueLayoutConfig{RootDir: queueRoot},
		},
	}
	return ws, queuePath, archivePath
}

// TestArchiveItem_DuplicateSourcePaths_PreservesSuccess verifies that ArchiveItem
// succeeds when the item exists in both the queue and archive directories.
func TestArchiveItem_DuplicateSourcePaths_PreservesSuccess(t *testing.T) {
	ws, _, _ := setupDuplicateArchiveWorkspace(t)
	ctx := context.Background()

	record, err := core.ArchiveItem(ctx, ws.DB, ws, "077-T")

	require.NoError(t, err, "ArchiveItem must succeed when both queue and archive copies exist")
	assert.NotNil(t, record)
}

// TestArchiveItem_DuplicateSourcePaths_NoStaleQueueCopy verifies that after
// a successful archive, no copy remains in the queue directory.
func TestArchiveItem_DuplicateSourcePaths_NoStaleQueueCopy(t *testing.T) {
	ws, queuePath, _ := setupDuplicateArchiveWorkspace(t)
	ctx := context.Background()

	_, err := core.ArchiveItem(ctx, ws.DB, ws, "077-T")
	require.NoError(t, err)

	assert.NoFileExists(t, queuePath,
		"stale queue copy must be removed after successful archive")
}

// TestArchiveItem_DuplicateSourcePaths_ArchiveFileCorrect verifies that the
// archive file is in the archive directory with status=archived after the call.
func TestArchiveItem_DuplicateSourcePaths_ArchiveFileCorrect(t *testing.T) {
	ws, _, archivePath := setupDuplicateArchiveWorkspace(t)
	ctx := context.Background()

	record, err := core.ArchiveItem(ctx, ws.DB, ws, "077-T")
	require.NoError(t, err)

	assert.Equal(t, archivePath, record.ArchivePath,
		"archive record must point to the archive directory")
	assert.FileExists(t, archivePath, "archive file must exist after archive")
}

// TestArchiveItem_DuplicateSourcePaths_UsesConfiguredQueueRoot verifies that
// ArchiveItem respects the configured queue root when resolving the canonical
// source file during duplicate-source recovery.
func TestArchiveItem_DuplicateSourcePaths_UsesConfiguredQueueRoot(t *testing.T) {
	ws, queuePath, archivePath := setupDuplicateArchiveWorkspaceWithQueueRoot(t, "inbox")
	ctx := context.Background()

	record, err := core.ArchiveItem(ctx, ws.DB, ws, "077-T")

	require.NoError(t, err)
	assert.Equal(t, queuePath, record.OriginalPath,
		"configured queue-root copy must be chosen as the canonical source")
	assert.Equal(t, archivePath, record.ArchivePath)
	assert.NoFileExists(t, queuePath)
	assert.FileExists(t, archivePath)
}

// ---------------------------------------------------------------------------
// 060.003-T: Restore status during unarchive
// ---------------------------------------------------------------------------

// setupArchiveStatusWorkspace creates a workspace with one task in "done" status.
func setupArchiveStatusWorkspace(t *testing.T) (*core.Workspace, string) {
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

	require.NoError(t, os.WriteFile(filepath.Join(backlogDir, "config.yaml"),
		[]byte("artifact_types:\n  task:\n    prefix: T\n    suffix: \"-T\"\n    name_format: \"{NNN}{suffix}\"\nmax_slug_length: 60\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(backlogDir, "header-def.yaml"), []byte("defaults: {}\ntypes: {}\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(backlogDir, "registry.yaml"), []byte("routes: {}\n"), 0o644))

	const itemID = "076-T"
	content := "---\nid: 076-T\ntitle: Status restore test\nstatus: done\nartifact_type: task\n---\nBody\n"
	filePath := filepath.Join(queueDir, itemID+".md")
	require.NoError(t, os.WriteFile(filePath, []byte(content), 0o644))
	require.NoError(t, db.UpsertItem(context.Background(), database, &models.Artifact{
		ID: itemID, Title: "Status restore test", Status: models.StatusDone, ArtifactType: "task",
	}))

	ws := &core.Workspace{RootPath: tmpDir, DB: database}
	return ws, filePath
}

// TestUnarchiveItem_RestoresPreArchiveStatus verifies that after archive+unarchive
// the item's status is restored to the pre-archive value, not left as "archived".
func TestUnarchiveItem_RestoresPreArchiveStatus(t *testing.T) {
	ws, _ := setupArchiveStatusWorkspace(t)
	ctx := context.Background()

	// Step 1: archive the item.
	_, err := core.ArchiveItem(ctx, ws.DB, ws, "076-T")
	require.NoError(t, err, "ArchiveItem must succeed")

	// Step 2: unarchive it.
	err = core.UnarchiveItem(ctx, ws.DB, ws, "076-T")
	require.NoError(t, err, "UnarchiveItem must succeed")

	// Step 3: find the restored file and check its status field.
	restoredPath, findErr := core.FindArtifactPath(ctx, ws, "076-T")
	require.NoError(t, findErr, "restored item must be findable after unarchive")

	raw, readErr := os.ReadFile(restoredPath)
	require.NoError(t, readErr)
	content := string(raw)

	assert.True(t, strings.Contains(content, "status: done"),
		"restored file must have original status 'done', not 'archived'; got:\n%s", content)
	assert.False(t, strings.Contains(content, "status: archived"),
		"restored file must NOT have status 'archived' after unarchive; got:\n%s", content)
}

// TestUnarchiveItem_RestoredStatusInDB verifies that the DB record reflects
// the restored status (not "archived") after UnarchiveItem.
func TestUnarchiveItem_RestoredStatusInDB(t *testing.T) {
	ws, _ := setupArchiveStatusWorkspace(t)
	ctx := context.Background()

	_, archErr := core.ArchiveItem(ctx, ws.DB, ws, "076-T")
	require.NoError(t, archErr, "ArchiveItem must succeed")
	require.NoError(t, core.UnarchiveItem(ctx, ws.DB, ws, "076-T"), "UnarchiveItem must succeed")

	item, dbErr := db.GetItem(ctx, ws.DB, "076-T")
	require.NoError(t, dbErr)
	assert.NotEqual(t, models.StatusArchived, item.Status,
		"DB status must not be 'archived' after unarchive; got %q", item.Status)
}

// TestUnarchiveItem_NoArchivedStatusField verifies that after unarchive the
// frontmatter does NOT contain the internal "archived_status" helper field.
func TestUnarchiveItem_NoArchivedStatusField(t *testing.T) {
	ws, _ := setupArchiveStatusWorkspace(t)
	ctx := context.Background()

	_, archErr2 := core.ArchiveItem(ctx, ws.DB, ws, "076-T")
	require.NoError(t, archErr2)
	require.NoError(t, core.UnarchiveItem(ctx, ws.DB, ws, "076-T"))

	restoredPath, findErr := core.FindArtifactPath(ctx, ws, "076-T")
	require.NoError(t, findErr)

	raw, readErr := os.ReadFile(restoredPath)
	require.NoError(t, readErr)
	assert.False(t, strings.Contains(string(raw), "archived_status"),
		"restored file must not contain the 'archived_status' helper field")
}
