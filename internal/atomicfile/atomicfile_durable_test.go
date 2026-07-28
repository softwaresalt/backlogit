package atomicfile

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// TestWriteFileAtomic_DurableOff_NoFsync asserts the durable-off fast path adds
// no fsync of the temp file or parent directory.
func TestWriteFileAtomic_DurableOff_NoFsync(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.md")
	seams := durableSeams{
		dirSyncEnabled: true,
		syncFile:       func(*os.File) error { t.Fatal("durable-off must not fsync temp"); return nil },
		syncDir:        func(*os.File) error { t.Fatal("durable-off must not fsync dir"); return nil },
	}
	require.NoError(t, writeFileAtomic(path, []byte("data"), false, seams))
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "data", string(got))
}

// TestWriteFileAtomic_DurableOn_FsyncsTempAndParentDir asserts the durable path
// fsyncs the temp file and, on POSIX, the parent directory (injected seams).
func TestWriteFileAtomic_DurableOn_FsyncsTempAndParentDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.md")
	tempSyncs, dirSyncs := 0, 0
	seams := durableSeams{
		dirSyncEnabled: true,
		syncFile:       func(f *os.File) error { tempSyncs++; return f.Sync() },
		syncDir:        func(*os.File) error { dirSyncs++; return nil },
	}
	require.NoError(t, writeFileAtomic(path, []byte("durable"), true, seams))
	assert.Equal(t, 1, tempSyncs, "durable write must fsync the temp file")
	assert.Equal(t, 1, dirSyncs, "durable write must fsync the parent dir on POSIX")
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "durable", string(got))
}

// TestWriteFileAtomic_DurableOn_WindowsDirFsyncSkip asserts no parent-dir fsync
// is attempted when directory syncing is disabled (Windows best-effort dirent),
// while the temp file is still fsynced.
func TestWriteFileAtomic_DurableOn_WindowsDirFsyncSkip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.md")
	tempSyncs := 0
	seams := durableSeams{
		dirSyncEnabled: false, // models Windows (no directory-handle flush)
		syncFile:       func(f *os.File) error { tempSyncs++; return f.Sync() },
		syncDir:        func(*os.File) error { t.Fatal("Windows must not fsync the dir"); return nil },
	}
	require.NoError(t, writeFileAtomic(path, []byte("data"), true, seams))
	assert.Equal(t, 1, tempSyncs, "the temp file is still fsynced on Windows")
}

// TestWriteFileAtomic_PostRenameDirFsyncFailureIndeterminate asserts a parent
// dir fsync failure AFTER the rename commits surfaces ErrWriteIndeterminate
// while the destination is already replaced with the new content.
func TestWriteFileAtomic_PostRenameDirFsyncFailureIndeterminate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.md")
	require.NoError(t, os.WriteFile(path, []byte("orig"), 0o644))
	seams := durableSeams{
		dirSyncEnabled: true,
		syncFile:       func(f *os.File) error { return f.Sync() },
		syncDir:        func(*os.File) error { return errors.New("simulated dir fsync failure") },
	}
	err := writeFileAtomic(path, []byte("replacement"), true, seams)
	require.Error(t, err)
	assert.True(t, blerrors.IsWriteIndeterminate(err),
		"a post-rename dir fsync failure must be indeterminate")
	got, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	assert.Equal(t, "replacement", string(got),
		"the file is already replaced even though the dir fsync failed")
}

// TestWriteFileAtomic_PreRenameFailureNotApplied asserts a failure before the
// rename commits (here a temp fsync failure) surfaces ErrWriteNotApplied and
// leaves the original destination intact.
func TestWriteFileAtomic_PreRenameFailureNotApplied(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.md")
	require.NoError(t, os.WriteFile(path, []byte("orig"), 0o644))
	seams := durableSeams{
		dirSyncEnabled: true,
		syncFile:       func(*os.File) error { return errors.New("simulated temp fsync failure") },
		syncDir:        func(*os.File) error { return nil },
	}
	err := writeFileAtomic(path, []byte("replacement"), true, seams)
	require.Error(t, err)
	assert.True(t, blerrors.IsWriteNotApplied(err),
		"a pre-rename failure must be not-applied")
	got, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	assert.Equal(t, "orig", string(got), "the destination must be untouched before the rename")
}

// TestWriteFileAtomicWithOptions_Wrapper asserts the public options entrypoint
// works end-to-end with real fsyncs.
func TestWriteFileAtomicWithOptions_Wrapper(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.md")
	require.NoError(t, WriteFileAtomicWithOptions(path, []byte("real durable"), Options{DurableWrites: true}))
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "real durable", string(got))
}
