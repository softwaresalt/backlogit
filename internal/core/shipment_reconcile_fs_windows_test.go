//go:build windows

package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWriteShipmentReconcileArchiveFileHandleRelative_RejectsIntermediateJunctionTOCTOU
// (Copilot PR #440 review) proves the fix for the finding that
// os.CreateTemp/os.Rename are plain pathname operations re-walking
// archiveDir after the pre-open reparse-point validation, leaving a window
// in which an intermediate directory inside archiveDir could be swapped for
// a reparse point before the write actually lands.
//
// It deterministically simulates that race using the (test-only)
// shipmentReconcileFSWindowsTOCTOUHook seam: archiveDir starts as a real
// directory (so the initial isReparsePointPath/EvalSymlinks snapshot
// succeeds), and the hook — invoked by production code immediately after
// that snapshot and before the temp file is created — replaces archiveDir
// with a junction pointing OUTSIDE the workspace. The write must fail
// closed (return an indeterminate error, since the content may already be
// on disk somewhere) and must NOT report success.
func TestWriteShipmentReconcileArchiveFileHandleRelative_RejectsIntermediateJunctionTOCTOU(t *testing.T) {
	base := t.TempDir()
	archiveDir := filepath.Join(base, "archive")
	require.NoError(t, os.MkdirAll(archiveDir, 0o755))

	outside := t.TempDir()

	t.Cleanup(func() { shipmentReconcileFSWindowsTOCTOUHook = nil })
	hookCalled := false
	shipmentReconcileFSWindowsTOCTOUHook = func() {
		hookCalled = true
		require.NoError(t, os.Remove(archiveDir), "the original real archive directory must be removable (empty) to simulate the swap")
		if err := createReconcileTestJunction(t, archiveDir, outside); err != nil {
			t.Skipf("junction creation skipped: %v", err)
		}
	}

	seams := defaultShipmentReconcileFSSeams()
	err := writeShipmentReconcileArchiveFileHandleRelative(archiveDir, "toctou-shipment.md", []byte("content"), seams)
	require.Error(t, err, "an intermediate-directory reparse-point swap between validation and write must be rejected")
	assert.True(t, hookCalled, "the TOCTOU simulation hook must have run")
}

// TestWriteShipmentReconcileArchiveFileHandleRelative_SucceedsWithoutSwap is
// the control case: with the TOCTOU hook left nil (production behavior), a
// normal write against a stable archiveDir must still succeed, proving the
// new snapshot/post-rename final-path re-validation does not itself break
// the legitimate path.
func TestWriteShipmentReconcileArchiveFileHandleRelative_SucceedsWithoutSwap(t *testing.T) {
	archiveDir := t.TempDir()
	seams := defaultShipmentReconcileFSSeams()
	err := writeShipmentReconcileArchiveFileHandleRelative(archiveDir, "control-shipment.md", []byte("content"), seams)
	require.NoError(t, err)

	got, readErr := os.ReadFile(filepath.Join(archiveDir, "control-shipment.md"))
	require.NoError(t, readErr)
	assert.Equal(t, "content", string(got))
}

// TestWriteShipmentReconcileArchiveFile_EndToEnd_RejectsIntermediateJunctionTOCTOU
// exercises the same simulated TOCTOU race through the full portable
// orchestration entry point (writeShipmentReconcileArchiveFile), confirming
// the hardening is actually reachable from real callers, not just the
// platform-specific primitive in isolation.
func TestWriteShipmentReconcileArchiveFile_EndToEnd_RejectsIntermediateJunctionTOCTOU(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	archiveDir := filepath.Join(workspaceStorageRoot(ws), shipmentReconcileArchiveDirName)
	require.NoError(t, os.MkdirAll(archiveDir, 0o755))

	outside := t.TempDir()

	t.Cleanup(func() { shipmentReconcileFSWindowsTOCTOUHook = nil })
	shipmentReconcileFSWindowsTOCTOUHook = func() {
		require.NoError(t, os.Remove(archiveDir))
		if err := createReconcileTestJunction(t, archiveDir, outside); err != nil {
			t.Skipf("junction creation skipped: %v", err)
		}
	}

	err := writeShipmentReconcileArchiveFile(context.Background(), ws, "167.006-T-e2e-toctou", []byte("x"))
	require.Error(t, err, "the end-to-end entry point must also reject the simulated TOCTOU race")
}
