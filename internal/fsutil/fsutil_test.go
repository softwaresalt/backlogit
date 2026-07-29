package fsutil_test

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/fsutil"
)

// --- FsyncDir tests (131.001-T) ---

// TestFsyncDir_SuccessOnRealDir verifies that FsyncDir returns nil for an
// existing directory. Skipped on Windows because directory-handle Sync() is
// unsupported there; production gates on dirSyncEnabled = runtime.GOOS != "windows".
func TestFsyncDir_SuccessOnRealDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory fsync unsupported on Windows")
	}
	dir := t.TempDir()
	require.NoError(t, fsutil.FsyncDir(dir))
}

// TestFsyncDir_ErrorOnNonExistentPath verifies that FsyncDir returns an error
// that names the path when the directory does not exist (open-error path;
// reliable cross-platform).
func TestFsyncDir_ErrorOnNonExistentPath(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "no-such-dir")
	err := fsutil.FsyncDir(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), dir, "error must name the failing path")
}

// --- MkdirAllDurable creation-path tests (131.004-T) ---

// TestMkdirAllDurable_NonDurableIsExactlyMkdirAll verifies that when durable is
// false, the call is equivalent to os.MkdirAll with no syncDir invocation.
func TestMkdirAllDurable_NonDurableIsExactlyMkdirAll(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	dir := filepath.Join(base, "a", "b", "c")
	syncCalled := false
	err := fsutil.MkdirAllDurable(dir, false, false, func(string) error {
		syncCalled = true
		return nil
	})
	require.NoError(t, err)
	assert.DirExists(t, dir)
	assert.False(t, syncCalled, "non-durable path must not call syncDir")
}

// TestMkdirAllDurable_DurablePOSIXFsyncsAncestorParents verifies that in durable
// mode with dirSyncEnabled=true, each newly created ancestor's parent is fsynced.
func TestMkdirAllDurable_DurablePOSIXFsyncsAncestorParents(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	dir := filepath.Join(base, "a", "b")
	var synced []string
	err := fsutil.MkdirAllDurable(dir, true, true, func(p string) error {
		synced = append(synced, p)
		return nil
	})
	require.NoError(t, err)
	assert.DirExists(t, dir)
	// Parent of "a" (=base) and parent of "b" (=base/a) must be fsynced.
	assert.Contains(t, synced, base, "parent of first new dir must be fsynced")
	assert.Contains(t, synced, filepath.Join(base, "a"), "parent of second new dir must be fsynced")
}

// TestMkdirAllDurable_DirSyncDisabledSkipsFsync verifies that when dirSyncEnabled
// is false (Windows-equivalent), the tree is created but syncDir is never called.
func TestMkdirAllDurable_DirSyncDisabledSkipsFsync(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	dir := filepath.Join(base, "x", "y")
	syncCalled := false
	err := fsutil.MkdirAllDurable(dir, true, false, func(string) error {
		syncCalled = true
		return nil
	})
	require.NoError(t, err)
	assert.DirExists(t, dir)
	assert.False(t, syncCalled, "dirSyncEnabled=false must not call syncDir")
}

// TestMkdirAllDurable_FsyncErrorPropagates verifies that a syncDir failure during
// ancestor creation propagates as a non-nil neutral error containing "fsync".
func TestMkdirAllDurable_FsyncErrorPropagates(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	dir := filepath.Join(base, "newdir")
	fsyncErr := errors.New("simulated fsync failure")
	err := fsutil.MkdirAllDurable(dir, true, true, func(string) error {
		return fsyncErr
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fsync", "fsync error must be named in the returned error")
}

// --- MkdirAllDurable retry/re-confirm tests (131.005-T) ---

// TestMkdirAllDurable_ExistingDirReSyncsParent verifies that when dir already
// exists in durable mode, the parent is re-fsynced (U4) and nil is returned.
func TestMkdirAllDurable_ExistingDirReSyncsParent(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	dir := filepath.Join(base, "existing")
	require.NoError(t, os.Mkdir(dir, 0o755))
	var synced []string
	err := fsutil.MkdirAllDurable(dir, true, true, func(p string) error {
		synced = append(synced, p)
		return nil
	})
	require.NoError(t, err)
	assert.Contains(t, synced, base, "parent of existing dir must be re-fsynced (U4)")
}

// TestMkdirAllDurable_ExistingDirFsyncFails verifies that when the parent fsync
// fails for an existing dir, a non-nil neutral error is returned (caller maps to
// ErrWriteNotApplied).
func TestMkdirAllDurable_ExistingDirFsyncFails(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	dir := filepath.Join(base, "existing")
	require.NoError(t, os.Mkdir(dir, 0o755))
	wantErr := errors.New("simulated fsync failure")
	err := fsutil.MkdirAllDurable(dir, true, true, func(string) error { return wantErr })
	require.Error(t, err)
	assert.True(t, errors.Is(err, wantErr), "returned error must wrap the syncDir failure")
}

// TestMkdirAllDurable_RetryAfterFsyncFail verifies U4 semantics: on the second
// call after dir was created, the parent is re-fsynced (retried), not skipped.
func TestMkdirAllDurable_RetryAfterFsyncFail(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	dir := filepath.Join(base, "newdir")
	// Only count fsyncs of base (direct parent of dir); Finding-2 may also fire
	// for filepath.Dir(base) on the first call, which we ignore.
	baseCalls := 0
	syncDir := func(p string) error {
		if p == base {
			baseCalls++
			if baseCalls == 1 {
				return errors.New("first attempt fails")
			}
		}
		return nil
	}
	// First call: dir doesn't exist, created, parent fsync fails.
	err1 := fsutil.MkdirAllDurable(dir, true, true, syncDir)
	require.Error(t, err1)
	assert.DirExists(t, dir)
	assert.Equal(t, 1, baseCalls, "parent fsynced once on first call")

	// Second call: dir now exists, U4 re-fsyncs parent.
	err2 := fsutil.MkdirAllDurable(dir, true, true, syncDir)
	require.NoError(t, err2)
	assert.Equal(t, 2, baseCalls, "parent must be re-fsynced on retry (U4)")
}

// TestMkdirAllDurable_NestedPartialCreateReconfirmsAncestor verifies Finding-2:
// when some ancestors exist and some are missing, the first existing ancestor's
// parent is re-confirmed (fsynced) before the missing dirs are created.
func TestMkdirAllDurable_NestedPartialCreateReconfirmsAncestor(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	// base/a exists (first existing ancestor), but base/a/b does not.
	existingAncestor := filepath.Join(base, "a")
	require.NoError(t, os.Mkdir(existingAncestor, 0o755))
	dir := filepath.Join(existingAncestor, "b")
	var synced []string
	err := fsutil.MkdirAllDurable(dir, true, true, func(p string) error {
		synced = append(synced, p)
		return nil
	})
	require.NoError(t, err)
	assert.DirExists(t, dir)
	// Finding-2: parent of first existing ancestor (base) must be re-confirmed.
	assert.Contains(t, synced, base,
		"parent of first existing ancestor must be re-confirmed (Finding-2)")
}
