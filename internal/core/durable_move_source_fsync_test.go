package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/models"
)

// recordDirSyncs swaps the mkdirDirSyncEnabled/mkdirDirSyncFn seams to model POSIX
// on a Windows host and record every directory fsynced. failOn, when non-empty,
// makes the fsync of that exact dir fail so the failure-classification behavior is
// testable in-process (a process kill cannot simulate power loss).
//
// Must not run with t.Parallel: this swaps package-global seams read on the
// production write path.
func recordDirSyncs(t *testing.T, failOn string) *[]string {
	t.Helper()
	origEnabled := mkdirDirSyncEnabled
	origFn := mkdirDirSyncFn
	mkdirDirSyncEnabled = true
	synced := &[]string{}
	mkdirDirSyncFn = func(p string) error {
		*synced = append(*synced, p)
		if failOn != "" && p == failOn {
			return errors.New("simulated dir fsync failure")
		}
		return nil
	}
	t.Cleanup(func() { mkdirDirSyncEnabled = origEnabled; mkdirDirSyncFn = origFn })
	return synced
}

// --- Site 1: persistArtifact (surfaces ErrWriteIndeterminate) ----------------

// TestPersistArtifact_DurableCrossDirMoveFsyncsSourceParent asserts a durable
// cross-directory relocation fsyncs the SOURCE parent (so the removed old dirent
// is durable) in addition to the destination the artifact write already flushed.
func TestPersistArtifact_DurableCrossDirMoveFsyncsSourceParent(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	feat, err := CreateArtifact(ctx, ws, "Feature", "feature")
	require.NoError(t, err)
	task, err := CreateArtifact(ctx, ws, "Task", "task", WithParent(feat.ID))
	require.NoError(t, err)
	srcDir := filepath.Dir(findArtifactPathDirect(ws, task.ID))

	ws.Config.DurableWrites = true
	synced := recordDirSyncs(t, "")

	task.Status = models.StatusDone // done routes to the archive dir (cross-dir)
	require.NoError(t, persistArtifact(ctx, ws, task, true))

	assert.Contains(t, *synced, srcDir, "durable cross-dir move must fsync the SOURCE parent dir")
}

// TestPersistArtifact_SourceFsyncFailureIsIndeterminate asserts a source-parent
// fsync failure surfaces ErrWriteIndeterminate (it runs before the sole DB
// upsert, so surfacing is safe and honors the durable_writes contract).
func TestPersistArtifact_SourceFsyncFailureIsIndeterminate(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	feat, err := CreateArtifact(ctx, ws, "Feature", "feature")
	require.NoError(t, err)
	task, err := CreateArtifact(ctx, ws, "Task", "task", WithParent(feat.ID))
	require.NoError(t, err)
	srcDir := filepath.Dir(findArtifactPathDirect(ws, task.ID))

	ws.Config.DurableWrites = true
	recordDirSyncs(t, srcDir) // fail only the source-parent fsync

	task.Status = models.StatusDone
	err = persistArtifact(ctx, ws, task, true)
	require.Error(t, err)
	assert.True(t, blerrors.IsWriteIndeterminate(err),
		"a source-parent fsync failure before the DB upsert must be indeterminate")
}

// TestPersistArtifact_DurableOffSkipsSourceFsync asserts the durable-off fast path
// performs no source-parent fsync (byte-for-byte prior behavior).
func TestPersistArtifact_DurableOffSkipsSourceFsync(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	feat, err := CreateArtifact(ctx, ws, "Feature", "feature")
	require.NoError(t, err)
	task, err := CreateArtifact(ctx, ws, "Task", "task", WithParent(feat.ID))
	require.NoError(t, err)

	// durable stays off; seam still swapped to prove it is never invoked.
	synced := recordDirSyncs(t, "")
	task.Status = models.StatusDone
	require.NoError(t, persistArtifact(ctx, ws, task, true))
	assert.Empty(t, *synced, "durable-off relocation must not fsync any directory")
}

// TestPersistArtifact_WindowsSkipsSourceFsync asserts the Windows best-effort
// dirent behavior: with dir syncing disabled no source-parent fsync is attempted.
func TestPersistArtifact_WindowsSkipsSourceFsync(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	feat, err := CreateArtifact(ctx, ws, "Feature", "feature")
	require.NoError(t, err)
	task, err := CreateArtifact(ctx, ws, "Task", "task", WithParent(feat.ID))
	require.NoError(t, err)

	origEnabled := mkdirDirSyncEnabled
	origFn := mkdirDirSyncFn
	mkdirDirSyncEnabled = false // model Windows
	called := false
	mkdirDirSyncFn = func(string) error { called = true; return nil }
	t.Cleanup(func() { mkdirDirSyncEnabled = origEnabled; mkdirDirSyncFn = origFn })

	ws.Config.DurableWrites = true
	task.Status = models.StatusDone
	require.NoError(t, persistArtifact(ctx, ws, task, true))
	assert.False(t, called, "Windows dirent durability is best-effort; no dir fsync attempted")
}

// --- Site 3: ArchiveItem (best-effort warn) ----------------------------------

// TestArchiveItem_DurableFsyncsSourceParent asserts archiving a queued item
// (queue -> archive cross-dir move) fsyncs the SOURCE (queue) parent.
func TestArchiveItem_DurableFsyncsSourceParent(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	feat, err := CreateArtifact(ctx, ws, "Feature", "feature")
	require.NoError(t, err)
	task, err := CreateArtifact(ctx, ws, "Task", "task", WithParent(feat.ID))
	require.NoError(t, err)
	srcDir := filepath.Dir(findArtifactPathDirect(ws, task.ID))

	ws.Config.DurableWrites = true
	synced := recordDirSyncs(t, "")

	_, err = ArchiveItem(ctx, ws.DB, ws, task.ID)
	require.NoError(t, err)
	assert.Contains(t, *synced, srcDir, "durable archive must fsync the SOURCE (queue) parent dir")
}

// TestArchiveItem_SourceFsyncFailureDoesNotRollBack asserts the best-effort
// contract: a source-parent fsync failure is logged, not surfaced, so the
// completed archive move is NOT rolled back (FS/DB stay consistent).
func TestArchiveItem_SourceFsyncFailureDoesNotRollBack(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	feat, err := CreateArtifact(ctx, ws, "Feature", "feature")
	require.NoError(t, err)
	task, err := CreateArtifact(ctx, ws, "Task", "task", WithParent(feat.ID))
	require.NoError(t, err)
	srcDir := filepath.Dir(findArtifactPathDirect(ws, task.ID))

	ws.Config.DurableWrites = true
	recordDirSyncs(t, srcDir) // fail the source-parent fsync

	rec, err := ArchiveItem(ctx, ws.DB, ws, task.ID)
	require.NoError(t, err, "a best-effort source fsync failure must not fail the archive")
	require.NotNil(t, rec)
	assert.FileExists(t, findArtifactPathDirect(ws, task.ID),
		"the archive move completed and was not rolled back")
}

// --- Site 4: UnarchiveItem (best-effort warn) --------------------------------

// TestUnarchiveItem_DurableFsyncsSourceParent asserts restoring an archived item
// (archive -> queue cross-dir move) fsyncs the SOURCE (archive) parent.
func TestUnarchiveItem_DurableFsyncsSourceParent(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	feat, err := CreateArtifact(ctx, ws, "Feature", "feature")
	require.NoError(t, err)
	task, err := CreateArtifact(ctx, ws, "Task", "task", WithParent(feat.ID))
	require.NoError(t, err)

	// Archive first with durable OFF so the setup does not record syncs.
	_, err = ArchiveItem(ctx, ws.DB, ws, task.ID)
	require.NoError(t, err)
	archiveDir := filepath.Dir(findArtifactPathDirect(ws, task.ID))

	ws.Config.DurableWrites = true
	synced := recordDirSyncs(t, "")

	require.NoError(t, UnarchiveItem(ctx, ws.DB, ws, task.ID))
	assert.Contains(t, *synced, archiveDir, "durable unarchive must fsync the SOURCE (archive) parent dir")
}

// --- Site 2: AdoptItem (best-effort warn, same-directory rename) --------------

// TestAdoptItem_DurableFsyncsRenameDir asserts a durable adopt ID-rename fsyncs
// the directory holding the renamed .md so the removed old-ID dirent is durable.
func TestAdoptItem_DurableFsyncsRenameDir(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	feat1, err := CreateArtifact(ctx, ws, "Feature one", "feature")
	require.NoError(t, err)
	feat2, err := CreateArtifact(ctx, ws, "Feature two", "feature")
	require.NoError(t, err)
	task, err := CreateArtifact(ctx, ws, "Task", "task", WithParent(feat1.ID))
	require.NoError(t, err)
	renameDir := filepath.Dir(findArtifactPathDirect(ws, task.ID))

	ws.Config.DurableWrites = true
	synced := recordDirSyncs(t, "")

	res, err := AdoptItem(ctx, ws, task.ID, feat2.ID)
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Contains(t, *synced, renameDir, "durable adopt rename must fsync the .md directory")
}

// TestAdoptItem_RenameFsyncFailureIsIndeterminateButApplied asserts the
// commit-then-surface contract (Finding C2): a post-mutation rename-dir fsync
// failure is surfaced as ErrWriteIndeterminate, but the adopt is NOT rolled back
// (the rename/tx already committed, so FS and DB must stay in agreement). The
// caller still receives the fully-built result alongside the honest signal.
func TestAdoptItem_RenameFsyncFailureIsIndeterminateButApplied(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	feat1, err := CreateArtifact(ctx, ws, "Feature one", "feature")
	require.NoError(t, err)
	feat2, err := CreateArtifact(ctx, ws, "Feature two", "feature")
	require.NoError(t, err)
	task, err := CreateArtifact(ctx, ws, "Task", "task", WithParent(feat1.ID))
	require.NoError(t, err)
	oldID := task.ID
	renameDir := filepath.Dir(findArtifactPathDirect(ws, oldID))

	ws.Config.DurableWrites = true
	recordDirSyncs(t, renameDir) // fail the rename-dir fsync

	res, err := AdoptItem(ctx, ws, oldID, feat2.ID)
	// (a) The durability failure is surfaced as indeterminate.
	require.Error(t, err)
	assert.True(t, blerrors.IsWriteIndeterminate(err),
		"a post-mutation adopt fsync failure must surface ErrWriteIndeterminate")
	// (b) The adopt APPLIED and was not rolled back: the result is fully built,
	// the new-ID file is discoverable, and the old-ID entry is gone.
	require.NotNil(t, res)
	assert.NotEmpty(t, res.NewID)
	assert.FileExists(t, findArtifactPathDirect(ws, res.NewID),
		"the renamed new-ID artifact must remain on disk (not rolled back)")
	assert.Empty(t, findArtifactPathDirect(ws, oldID),
		"the old-ID artifact must be gone (the rename completed)")
}

// TestAdoptItem_DurableSeamSuccessReturnsNil asserts the symmetric off-case: with
// a durable adopt whose dir fsyncs SUCCEED, AdoptItem returns a nil error (no
// false indeterminate). durableSyncDirDetailed is a POSIX/durable-gated no-op, so
// durable-off and Windows likewise never produce a spurious indeterminate signal.
func TestAdoptItem_DurableSeamSuccessReturnsNil(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	feat1, err := CreateArtifact(ctx, ws, "Feature one", "feature")
	require.NoError(t, err)
	feat2, err := CreateArtifact(ctx, ws, "Feature two", "feature")
	require.NoError(t, err)
	task, err := CreateArtifact(ctx, ws, "Task", "task", WithParent(feat1.ID))
	require.NoError(t, err)

	ws.Config.DurableWrites = true
	recordDirSyncs(t, "") // all fsyncs succeed

	res, err := AdoptItem(ctx, ws, task.ID, feat2.ID)
	require.NoError(t, err, "a successful durable adopt must not report indeterminate")
	require.NotNil(t, res)
}

// --- Finding CN: durable creation of a fresh archive/restore directory --------

// TestArchiveItem_DurableFsyncsNewArchiveDirParent asserts that when durable and
// the archive directory does not yet exist, ArchiveItem creates it durably: the
// parent (.backlogit) dirent of the newly created "archive" dir is fsynced so a
// power loss cannot lose the brand-new directory.
func TestArchiveItem_DurableFsyncsNewArchiveDirParent(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	feat, err := CreateArtifact(ctx, ws, "Feature", "feature")
	require.NoError(t, err)
	task, err := CreateArtifact(ctx, ws, "Task", "task", WithParent(feat.ID))
	require.NoError(t, err)

	backlogDir := WorkspaceStorageRoot(ws.RootPath)
	archiveDir := filepath.Join(backlogDir, "archive")
	require.NoDirExists(t, archiveDir, "precondition: archive dir must be freshly created")

	ws.Config.DurableWrites = true
	synced := recordDirSyncs(t, "")

	_, err = ArchiveItem(ctx, ws.DB, ws, task.ID)
	require.NoError(t, err)
	assert.Contains(t, *synced, backlogDir,
		"creating the fresh archive dir must fsync its parent dirent (.backlogit)")
}

// TestArchiveItem_DurableOffSkipsNewDirFsync asserts the durable-off fast path
// creates the archive dir with no added fsync (byte-for-byte prior behavior).
func TestArchiveItem_DurableOffSkipsNewDirFsync(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	feat, err := CreateArtifact(ctx, ws, "Feature", "feature")
	require.NoError(t, err)
	task, err := CreateArtifact(ctx, ws, "Task", "task", WithParent(feat.ID))
	require.NoError(t, err)

	// durable stays off; seam swapped only to prove it is never invoked.
	synced := recordDirSyncs(t, "")
	_, err = ArchiveItem(ctx, ws.DB, ws, task.ID)
	require.NoError(t, err)
	assert.Empty(t, *synced, "durable-off archive must not fsync any directory")
}

// TestUnarchiveItem_DurableFsyncsNewRestoreDirParent asserts that when durable and
// the restore target directory does not yet exist, UnarchiveItem creates it
// durably: the parent dirent of the newly created restore dir is fsynced.
func TestUnarchiveItem_DurableFsyncsNewRestoreDirParent(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	feat, err := CreateArtifact(ctx, ws, "Feature", "feature")
	require.NoError(t, err)
	task, err := CreateArtifact(ctx, ws, "Task", "task", WithParent(feat.ID))
	require.NoError(t, err)
	queueDir := filepath.Dir(findArtifactPathDirect(ws, task.ID))

	// Archive with durable OFF so setup records nothing.
	_, err = ArchiveItem(ctx, ws.DB, ws, task.ID)
	require.NoError(t, err)
	// Remove the restore-target dir so the restore must recreate it durably.
	require.NoError(t, os.RemoveAll(queueDir))
	require.NoDirExists(t, queueDir)

	ws.Config.DurableWrites = true
	synced := recordDirSyncs(t, "")

	require.NoError(t, UnarchiveItem(ctx, ws.DB, ws, task.ID))
	assert.Contains(t, *synced, filepath.Dir(queueDir),
		"recreating the fresh restore dir must fsync its parent dirent")
}
