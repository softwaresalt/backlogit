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

// TestWriteShipmentReconcileArchiveFileHandleRelative_TOCTOUSwapLeaksRealPayload
// (Copilot PR #440 review residual-risk investigation; flipped by 201.005-T)
// asserts the SHARPER form of the finding above: when the archive directory is
// swapped for a junction pointing OUTSIDE the workspace after validation, no
// payload byte may reach the outside location, and the call must still fail
// closed. The writer binds every create and rename to a pinned directory
// handle, so a swapped path can no longer redirect the write.
func TestWriteShipmentReconcileArchiveFileHandleRelative_TOCTOUSwapLeaksRealPayload(t *testing.T) {
	base := t.TempDir()
	archiveDir := filepath.Join(base, "archive")
	require.NoError(t, os.MkdirAll(archiveDir, 0o755))

	outside := t.TempDir()
	const payload = "this is the real archive payload content, not a placeholder"

	t.Cleanup(func() { shipmentReconcileFSWindowsTOCTOUHook = nil })
	shipmentReconcileFSWindowsTOCTOUHook = func() {
		require.NoError(t, os.Remove(archiveDir), "the original real archive directory must be removable (empty) to simulate the swap")
		if err := createReconcileTestJunction(t, archiveDir, outside); err != nil {
			t.Skipf("junction creation skipped: %v", err)
		}
	}

	seams := defaultShipmentReconcileFSSeams()
	err := writeShipmentReconcileArchiveFileHandleRelative(archiveDir, "leak-shipment.md", []byte(payload), seams)
	require.Error(t, err, "the swapped-directory race must still be reported as a failure")

	entries, readErr := os.ReadDir(outside)
	require.NoError(t, readErr)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, readFileErr := os.ReadFile(filepath.Join(outside, entry.Name()))
		require.NoError(t, readFileErr)
		assert.NotContains(t, string(data), payload[:8], "no payload byte may reach the swapped-in outside location (found in %s)", entry.Name())
		assert.Empty(t, data, "no file content may be written to the swapped-in outside location (found in %s)", entry.Name())
	}
}

// TestWriteShipmentReconcileArchiveFileHandleRelative_PreResolutionSwapIsRejected
// proves the pin is bound to the trusted pre-resolution path. The hook runs
// after the reparse-point pre-check and before any re-resolution: swapping the
// archive directory (a symlink, which EvalSymlinks follows, or a junction)
// or a parent directory (a junction that redirects the same path) for a link to
// an outside directory must be rejected with nothing written outside.
func TestWriteShipmentReconcileArchiveFileHandleRelative_PreResolutionSwapIsRejected(t *testing.T) {
	swaps := map[string]func(t *testing.T, base, outside string){
		"archive dir to symlink": func(t *testing.T, base, outside string) {
			archiveDir := filepath.Join(base, "parent", "archive")
			require.NoError(t, os.Remove(archiveDir))
			if err := os.Symlink(outside, archiveDir); err != nil {
				t.Skipf("symlink creation skipped: %v", err)
			}
		},
		"archive dir to junction": func(t *testing.T, base, outside string) {
			archiveDir := filepath.Join(base, "parent", "archive")
			require.NoError(t, os.Remove(archiveDir))
			if err := createReconcileTestJunction(t, archiveDir, outside); err != nil {
				t.Skipf("junction creation skipped: %v", err)
			}
		},
		"parent dir to junction": func(t *testing.T, base, outside string) {
			require.NoError(t, os.Rename(filepath.Join(base, "parent"), filepath.Join(base, "parent.orig")))
			require.NoError(t, os.MkdirAll(filepath.Join(outside, "archive"), 0o755))
			if err := createReconcileTestJunction(t, filepath.Join(base, "parent"), outside); err != nil {
				t.Skipf("junction creation skipped: %v", err)
			}
		},
	}
	for name, swap := range swaps {
		t.Run(name, func(t *testing.T) {
			base, err := filepath.EvalSymlinks(t.TempDir())
			require.NoError(t, err)
			archiveDir := filepath.Join(base, "parent", "archive")
			require.NoError(t, os.MkdirAll(archiveDir, 0o755))
			outside, err := filepath.EvalSymlinks(t.TempDir())
			require.NoError(t, err)

			t.Cleanup(func() { shipmentReconcileFSWindowsTOCTOUHook = nil })
			shipmentReconcileFSWindowsTOCTOUHook = func() { swap(t, base, outside) }

			err = writeShipmentReconcileArchiveFileHandleRelative(archiveDir, "pre-resolution.md", []byte("payload"), defaultShipmentReconcileFSSeams())
			require.Error(t, err)
			assertReconcileOutsideDirUntouched(t, outside)
		})
	}
}

// assertReconcileOutsideDirUntouched requires that no file anywhere under dir
// holds any bytes.
func assertReconcileOutsideDirUntouched(t *testing.T, dir string) {
	t.Helper()
	require.NoError(t, filepath.WalkDir(dir, func(path string, entry os.DirEntry, walkErr error) error {
		require.NoError(t, walkErr)
		if entry.IsDir() {
			return nil
		}
		info, infoErr := entry.Info()
		require.NoError(t, infoErr)
		assert.Zero(t, info.Size(), "payload reached %s", path)
		return nil
	}))
}

// TestWriteShipmentReconcileArchiveFileHandleRelative_SucceedsWithoutSwap is
// the control case: with the TOCTOU hook left nil (production behavior), a
// normal write against a stable archiveDir must still succeed, proving the
// new snapshot/post-rename final-path re-validation does not itself break
// the legitimate path.
func TestWriteShipmentReconcileArchiveFileHandleRelative_SucceedsWithoutSwap(t *testing.T) {
	archiveDir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err, "callers pass the canonical resolved directory")
	seams := defaultShipmentReconcileFSSeams()
	err = writeShipmentReconcileArchiveFileHandleRelative(archiveDir, "control-shipment.md", []byte("content"), seams)
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
