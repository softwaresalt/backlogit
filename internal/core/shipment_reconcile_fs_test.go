package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// 167.006-T behavior harness for writeShipmentReconcileArchiveFile (167.003-T
// panic-body declaration). The core orchestration (validation, fsync
// classification) is portable and injectable via shipmentReconcileFSSeams so
// the pre-rename/post-rename failure classification is deterministically
// testable on any host OS, mirroring internal/atomicfile's own
// seam-injection test pattern; the actual handle-relative create+rename
// mechanics are platform-specific (shipment_reconcile_fs_{unix,windows}.go).

func TestWriteShipmentReconcileArchiveFile_WritesAtomically(t *testing.T) {
	ws := setupShipmentWorkspace(t)

	err := writeShipmentReconcileArchiveFile(context.Background(), ws, "167.006-T-item", []byte("hello archive"))
	require.NoError(t, err)

	archivePath := filepath.Join(workspaceStorageRoot(ws), shipmentReconcileArchiveDirName, "167.006-T-item.md")
	got, readErr := os.ReadFile(archivePath)
	require.NoError(t, readErr)
	assert.Equal(t, "hello archive", string(got))

	// No temp file must be left behind.
	entries, dirErr := os.ReadDir(filepath.Dir(archivePath))
	require.NoError(t, dirErr)
	for _, entry := range entries {
		assert.NotContains(t, entry.Name(), ".tmp", "no temp file must survive a successful write")
	}
}

func TestWriteShipmentReconcileArchiveFile_OverwritesExistingAtomically(t *testing.T) {
	ws := setupShipmentWorkspace(t)

	require.NoError(t, writeShipmentReconcileArchiveFile(context.Background(), ws, "167.006-T-overwrite", []byte("first")))
	require.NoError(t, writeShipmentReconcileArchiveFile(context.Background(), ws, "167.006-T-overwrite", []byte("second, longer content")))

	archivePath := filepath.Join(workspaceStorageRoot(ws), shipmentReconcileArchiveDirName, "167.006-T-overwrite.md")
	got, err := os.ReadFile(archivePath)
	require.NoError(t, err)
	assert.Equal(t, "second, longer content", string(got))
}

func TestWriteShipmentReconcileArchiveFile_PreRenameFsyncFailureNotApplied(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	archivePath := filepath.Join(workspaceStorageRoot(ws), shipmentReconcileArchiveDirName, "167.006-T-prerename.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(archivePath), 0o755))
	require.NoError(t, os.WriteFile(archivePath, []byte("original"), 0o644))

	seams := shipmentReconcileFSSeams{
		dirSyncEnabled: true,
		syncFile:       func(*os.File) error { return errors.New("simulated temp fsync failure") },
		syncDir:        func(*os.File) error { return nil },
	}
	err := writeShipmentReconcileArchiveFileWithSeams(context.Background(), ws, "167.006-T-prerename", []byte("replacement"), seams)
	require.Error(t, err)
	assert.True(t, blerrors.IsWriteNotApplied(err), "a pre-rename failure must be not-applied, got: %v", err)

	got, readErr := os.ReadFile(archivePath)
	require.NoError(t, readErr)
	assert.Equal(t, "original", string(got), "the destination must be untouched before the rename commits")

	entries, dirErr := os.ReadDir(filepath.Dir(archivePath))
	require.NoError(t, dirErr)
	for _, entry := range entries {
		assert.NotContains(t, entry.Name(), ".tmp", "the temp file must be cleaned up on a pre-rename failure")
	}
}

func TestWriteShipmentReconcileArchiveFile_PostRenameDirFsyncFailureIndeterminate(t *testing.T) {
	ws := setupShipmentWorkspace(t)

	seams := shipmentReconcileFSSeams{
		dirSyncEnabled: true,
		syncFile:       func(f *os.File) error { return f.Sync() },
		syncDir:        func(*os.File) error { return errors.New("simulated dir fsync failure") },
	}
	err := writeShipmentReconcileArchiveFileWithSeams(context.Background(), ws, "167.006-T-postrename", []byte("committed content"), seams)
	require.Error(t, err)
	assert.True(t, blerrors.IsWriteIndeterminate(err), "a post-rename dir fsync failure must be indeterminate, got: %v", err)

	archivePath := filepath.Join(workspaceStorageRoot(ws), shipmentReconcileArchiveDirName, "167.006-T-postrename.md")
	got, readErr := os.ReadFile(archivePath)
	require.NoError(t, readErr)
	assert.Equal(t, "committed content", string(got), "the file is already replaced even though the dir fsync failed")
}

func TestWriteShipmentReconcileArchiveFile_RejectsUnsafeShipmentID(t *testing.T) {
	ws := setupShipmentWorkspace(t)

	ids := []string{"../escape", "a/b", ".."}
	if runtime.GOOS == "windows" {
		// Backslash is only a path separator (and therefore only rejected
		// as an unsafe multi-component filename) on Windows; on Unix-like
		// platforms filepath.Base treats a literal backslash as an ordinary
		// filename byte, so "a\b" is itself a safe single path component
		// there and must not be asserted as unsafe cross-platform.
		ids = append(ids, `a\b`)
	}
	for _, id := range ids {
		err := writeShipmentReconcileArchiveFile(context.Background(), ws, id, []byte("x"))
		assert.Error(t, err, "shipment id %q must be refused as an unsafe filename component", id)
	}
}

func TestWriteShipmentReconcileArchiveFile_ValidatesInputs(t *testing.T) {
	ws := setupShipmentWorkspace(t)

	err := writeShipmentReconcileArchiveFile(context.Background(), nil, "167.006-T", []byte("x"))
	assert.Error(t, err, "a nil workspace must be refused")

	err = writeShipmentReconcileArchiveFile(context.Background(), ws, "", []byte("x"))
	assert.Error(t, err, "an empty shipment id must be refused")
}

// TestWriteShipmentReconcileArchiveFile_RejectsSymlinkArchiveDir proves a
// symlinked archive directory is rejected rather than transparently
// followed. Skips gracefully on hosts where symlink creation is unsupported
// (for example, a non-elevated Windows account without SeCreateSymbolicLink).
func TestWriteShipmentReconcileArchiveFile_RejectsSymlinkArchiveDir(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	realArchiveDir := filepath.Join(workspaceStorageRoot(ws), shipmentReconcileArchiveDirName)
	require.NoError(t, os.MkdirAll(filepath.Dir(realArchiveDir), 0o755))

	escapeTarget := t.TempDir()
	if err := os.Symlink(escapeTarget, realArchiveDir); err != nil {
		t.Skipf("symlink creation unsupported on this host, skipping: %v", err)
	}

	err := writeShipmentReconcileArchiveFile(context.Background(), ws, "167.006-T-symlink", []byte("x"))
	require.Error(t, err, "a symlinked archive directory must be rejected, not followed")

	entries, dirErr := os.ReadDir(escapeTarget)
	require.NoError(t, dirErr)
	assert.Empty(t, entries, "nothing must be written through a symlinked archive directory")
}
