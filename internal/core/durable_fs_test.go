package core

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// TestMkdirAllDurable_FsyncsNewAncestorsPOSIX is the F7 regression: a durable
// relocation into a not-yet-existing nested status dir must create each missing
// ancestor and fsync its parent so a brand-new status dir + its artifact survive
// power loss (WriteArtifactFileWithOptions only fsyncs the immediate parent after
// rename, not newly created ancestors).
//
// Must not run with t.Parallel: this test swaps the mkdirDirSyncEnabled /
// mkdirDirSyncFn package-global seams read on the production write path.
func TestMkdirAllDurable_FsyncsNewAncestorsPOSIX(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, "a", "b", "status")

	origEnabled := mkdirDirSyncEnabled
	origFn := mkdirDirSyncFn
	mkdirDirSyncEnabled = true // model POSIX on a Windows host
	var synced []string
	mkdirDirSyncFn = func(p string) error { synced = append(synced, p); return nil }
	t.Cleanup(func() { mkdirDirSyncEnabled = origEnabled; mkdirDirSyncFn = origFn })

	require.NoError(t, mkdirAllDurable(target, true))

	assert.DirExists(t, target)
	// Every newly created ancestor's parent must be fsynced so its dirent is durable.
	assert.Contains(t, synced, base, "parent of new dir 'a' (base) must be fsynced")
	assert.Contains(t, synced, filepath.Join(base, "a"), "parent of new dir 'b' must be fsynced")
	assert.Contains(t, synced, filepath.Join(base, "a", "b"), "parent of new dir 'status' must be fsynced")
}

// TestMkdirAllDurable_WindowsSkipsDirFsync asserts the Windows best-effort dirent
// behavior: no per-ancestor fsync is attempted when directory syncing is disabled.
//
// Must not run with t.Parallel: this test swaps the mkdirDirSyncEnabled /
// mkdirDirSyncFn package-global seams read on the production write path.
func TestMkdirAllDurable_WindowsSkipsDirFsync(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, "a", "b", "status")

	origEnabled := mkdirDirSyncEnabled
	origFn := mkdirDirSyncFn
	mkdirDirSyncEnabled = false // model Windows (no directory-handle flush)
	called := false
	mkdirDirSyncFn = func(string) error { called = true; return nil }
	t.Cleanup(func() { mkdirDirSyncEnabled = origEnabled; mkdirDirSyncFn = origFn })

	require.NoError(t, mkdirAllDurable(target, true))
	assert.DirExists(t, target)
	assert.False(t, called, "Windows dirent durability is best-effort; no dir fsync attempted")
}

// TestMkdirAllDurable_NonDurableSkipsFsync asserts the durable-off path is exactly
// os.MkdirAll with no dir fsync.
//
// Must not run with t.Parallel: this test swaps the mkdirDirSyncEnabled /
// mkdirDirSyncFn package-global seams read on the production write path.
func TestMkdirAllDurable_NonDurableSkipsFsync(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, "a", "b", "status")

	origEnabled := mkdirDirSyncEnabled
	origFn := mkdirDirSyncFn
	mkdirDirSyncEnabled = true
	called := false
	mkdirDirSyncFn = func(string) error { called = true; return nil }
	t.Cleanup(func() { mkdirDirSyncEnabled = origEnabled; mkdirDirSyncFn = origFn })

	require.NoError(t, mkdirAllDurable(target, false))
	assert.DirExists(t, target)
	assert.False(t, called, "durable-off mkdir must not fsync any directory")
}

// TestMkdirAllDurable_PropagatesFsyncError asserts a per-ancestor dirent fsync
// failure is surfaced to the caller.
//
// Must not run with t.Parallel: this test swaps the mkdirDirSyncEnabled /
// mkdirDirSyncFn package-global seams read on the production write path.
func TestMkdirAllDurable_PropagatesFsyncError(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, "a", "b", "status")

	origEnabled := mkdirDirSyncEnabled
	origFn := mkdirDirSyncFn
	mkdirDirSyncEnabled = true
	mkdirDirSyncFn = func(string) error { return errors.New("simulated dir fsync failure") }
	t.Cleanup(func() { mkdirDirSyncEnabled = origEnabled; mkdirDirSyncFn = origFn })

	err := mkdirAllDurable(target, true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fsync")
	_ = os.RemoveAll(base)
}

// TestMkdirAllDurable_ExistingDurableDir_HappyPath_ReturnsNil is the U4 success-path
// guard: when the target dir already exists and durable is true, a successful parent
// fsync must return nil (no false ErrWriteIndeterminate for a confirmed dir).
//
// Must not run with t.Parallel: this test swaps the mkdirDirSyncEnabled /
// mkdirDirSyncFn package-global seams read on the production write path.
func TestMkdirAllDurable_ExistingDurableDir_HappyPath_ReturnsNil(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, "existing")
	require.NoError(t, os.Mkdir(target, 0o755))

	origEnabled := mkdirDirSyncEnabled
	origFn := mkdirDirSyncFn
	mkdirDirSyncEnabled = true
	var synced []string
	mkdirDirSyncFn = func(p string) error { synced = append(synced, p); return nil }
	t.Cleanup(func() { mkdirDirSyncEnabled = origEnabled; mkdirDirSyncFn = origFn })

	err := mkdirAllDurable(target, true)
	require.NoError(t, err, "existing durable dir with successful parent fsync must return nil")
	assert.Contains(t, synced, base, "parent of existing durable dir must be fsynced")
}

// TestMkdirAllDurable_ExistingDurableDir_FsyncFails_ReturnsErrWriteNotApplied is the
// U4 regression: when the target dir exists but the parent fsync fails, the error must
// be wrapped with ErrWriteNotApplied (pre-write, safe to retry) so callers classify it
// correctly.
//
// Must not run with t.Parallel: this test swaps the mkdirDirSyncEnabled /
// mkdirDirSyncFn package-global seams read on the production write path.
func TestMkdirAllDurable_ExistingDurableDir_FsyncFails_ReturnsErrWriteNotApplied(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, "existing")
	require.NoError(t, os.Mkdir(target, 0o755))

	origEnabled := mkdirDirSyncEnabled
	origFn := mkdirDirSyncFn
	mkdirDirSyncEnabled = true
	mkdirDirSyncFn = func(string) error { return errors.New("simulated dir fsync failure") }
	t.Cleanup(func() { mkdirDirSyncEnabled = origEnabled; mkdirDirSyncFn = origFn })

	err := mkdirAllDurable(target, true)
	require.Error(t, err)
	assert.True(t, errors.Is(err, blerrors.ErrWriteNotApplied),
		"existing-dir parent fsync failure must be wrapped with ErrWriteNotApplied")
}

// TestMkdirAllDurable_RetryAfterFsyncFail_ResyncesParent is the U4 core regression:
// first call creates the dir but the parent fsync fails (returns ErrWriteNotApplied);
// on retry the dir now exists and the parent fsync must be re-attempted, not skipped.
// This test asserts exactly-once event count: after the retry succeeds, a consumer
// dependent on mkdirAllDurable (e.g. UnarchiveItem) can safely proceed.
//
// Must not run with t.Parallel: this test swaps the mkdirDirSyncEnabled /
// mkdirDirSyncFn package-global seams read on the production write path.
func TestMkdirAllDurable_RetryAfterFsyncFail_ResyncesParent(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, "newdir")

	origEnabled := mkdirDirSyncEnabled
	origFn := mkdirDirSyncFn
	mkdirDirSyncEnabled = true
	fsyncCalls := 0
	mkdirDirSyncFn = func(p string) error {
		fsyncCalls++
		if fsyncCalls == 1 {
			return errors.New("first parent fsync failure")
		}
		return nil
	}
	t.Cleanup(func() { mkdirDirSyncEnabled = origEnabled; mkdirDirSyncFn = origFn })

	// First call: dir created but parent fsync fails → error wrapping ErrWriteNotApplied.
	err1 := mkdirAllDurable(target, true)
	require.Error(t, err1, "first call must fail when parent fsync fails")
	assert.True(t, errors.Is(err1, blerrors.ErrWriteNotApplied),
		"first-call error must wrap ErrWriteNotApplied (pre-write dir-fsync failure)")
	assert.DirExists(t, target, "dir must be created even though parent fsync failed")
	assert.Equal(t, 1, fsyncCalls, "parent fsync attempted once on first call")

	// Second call (retry): dir now exists → parent fsync must be re-attempted.
	err2 := mkdirAllDurable(target, true)
	require.NoError(t, err2, "retry must succeed when parent fsync succeeds")
	assert.Equal(t, 2, fsyncCalls, "parent fsync must be re-attempted on retry (not skipped)")
}
