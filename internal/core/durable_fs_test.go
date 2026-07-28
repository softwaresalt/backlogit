package core

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
