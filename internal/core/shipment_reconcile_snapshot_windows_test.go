//go:build windows

package core

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReadShipmentReconcileArchiveSnapshotFile_RejectsReparsePointArchiveFile
// (Copilot PR #440 review, finding 1) proves the Windows-specific archive
// snapshot read now rejects a reparse point planted directly at the archive
// file's own path, opened via FILE_FLAG_OPEN_REPARSE_POINT so the reparse
// object itself is inspected rather than transparently followed -- closing
// the same class of TOCTOU window the prior shared os.Lstat-then-os.ReadFile
// fallback left open (a symlink swapped in between the two calls).
func TestReadShipmentReconcileArchiveSnapshotFile_RejectsReparsePointArchiveFile(t *testing.T) {
	archiveDir := t.TempDir()
	outside := t.TempDir()
	outsideFile := filepath.Join(outside, "outside-shipment.md")
	require.NoError(t, os.WriteFile(outsideFile, []byte("outside content"), 0o644))

	linkPath := filepath.Join(archiveDir, "reparse-shipment.md")
	if err := createReconcileTestJunction(t, linkPath, outside); err == nil {
		// mklink /J only targets directories; a file-level reparse point
		// needs a symlink instead. Undo the (unexpected-success) junction
		// attempt and fall through to the symlink path below.
		_ = os.Remove(linkPath)
	}
	if err := os.Symlink(outsideFile, linkPath); err != nil {
		t.Skipf("symlink creation skipped: %v", err)
	}

	content, err := readShipmentReconcileArchiveSnapshotFile(archiveDir, "reparse-shipment.md")
	require.Error(t, err, "a reparse point at the archive file's own path must be rejected, not followed")
	assert.Nil(t, content)
}

// TestReadShipmentReconcileArchiveSnapshotFile_RejectsIntermediateJunctionSwap
// proves the post-open final-path re-validation catches an intermediate
// directory inside archiveDir being swapped for a junction pointing outside
// the workspace: the file is opened via a pathname derived from the
// (possibly-redirected) real archive directory, but the ACTUAL handle's own
// resolved final path is required to still land inside the originally
// resolved archive directory.
func TestReadShipmentReconcileArchiveSnapshotFile_RejectsIntermediateJunctionSwap(t *testing.T) {
	base := t.TempDir()
	archiveDir := filepath.Join(base, "archive")
	require.NoError(t, os.MkdirAll(archiveDir, 0o755))

	outside := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(outside, "swapped-shipment.md"), []byte("outside payload"), 0o644))

	require.NoError(t, os.Remove(archiveDir))
	if err := createReconcileTestJunction(t, archiveDir, outside); err != nil {
		t.Skipf("junction creation skipped: %v", err)
	}

	content, err := readShipmentReconcileArchiveSnapshotFile(archiveDir, "swapped-shipment.md")
	require.Error(t, err, "an archive directory swapped for a junction to an outside location must be rejected")
	assert.Nil(t, content)
}

// TestReadShipmentReconcileArchiveSnapshotFile_SucceedsWithoutSwap is the
// control case: a normal, stable archive directory and file must continue
// to read successfully exactly as before.
func TestReadShipmentReconcileArchiveSnapshotFile_SucceedsWithoutSwap(t *testing.T) {
	archiveDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(archiveDir, "control-shipment.md"), []byte("control content"), 0o644))

	content, err := readShipmentReconcileArchiveSnapshotFile(archiveDir, "control-shipment.md")
	require.NoError(t, err)
	assert.Equal(t, "control content", string(content))
}

// TestReadShipmentReconcileArchiveSnapshotFile_MissingFileIsNotExist proves
// the not-exist contract shipment_reconcile_snapshot.go's caller relies on
// (errors.Is(err, os.ErrNotExist)) still holds on Windows.
func TestReadShipmentReconcileArchiveSnapshotFile_MissingFileIsNotExist(t *testing.T) {
	archiveDir := t.TempDir()

	_, err := readShipmentReconcileArchiveSnapshotFile(archiveDir, "does-not-exist-shipment.md")
	require.Error(t, err)
	assert.True(t, errors.Is(err, os.ErrNotExist), "a missing archive file must satisfy errors.Is(err, os.ErrNotExist), got: %v", err)
}
