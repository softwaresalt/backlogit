//go:build windows

package core

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createReconcileTestJunction creates a Windows directory junction from
// linkPath -> targetDir using mklink /J (no elevated privileges required,
// unlike symlinks). Skips gracefully (via the returned error) if mklink
// fails, mirroring internal/events/checkpoint_s1_u1_junction_windows_test.go's
// own createJunction helper.
func createReconcileTestJunction(t *testing.T, linkPath, targetDir string) error {
	t.Helper()
	out, err := exec.Command("cmd", "/c", "mklink", "/J", linkPath, targetDir).CombinedOutput()
	if err != nil {
		return fmt.Errorf("mklink /J failed: %v (%s)", err, out)
	}
	t.Cleanup(func() { _ = os.Remove(linkPath) })
	return nil
}

// TestAppendShipmentReconcileEventHandleRelative_RejectsIntermediateJunctionTOCTOU
// (Copilot PR #440 review) proves the fix for the finding that
// FILE_FLAG_OPEN_REPARSE_POINT alone only protects logPath's OWN final path
// component, not an intermediate directory inside logsDir being swapped for
// a reparse point between the pre-open validation and the CreateFile call.
//
// It deterministically simulates that race using the (test-only)
// shipmentReconcileAppendWindowsTOCTOUHook seam: logsDir starts as a real
// directory (so the initial isReparsePointPath/EvalSymlinks snapshot
// succeeds), and the hook — invoked by production code immediately after
// that snapshot and before the file is opened — replaces logsDir with a
// junction pointing OUTSIDE the workspace. The append must fail closed
// (return an error) and must NOT have written eventBytes into the
// junction's target.
func TestAppendShipmentReconcileEventHandleRelative_RejectsIntermediateJunctionTOCTOU(t *testing.T) {
	base := t.TempDir()
	logsDir := filepath.Join(base, "logs")
	require.NoError(t, os.MkdirAll(logsDir, 0o755))

	outside := t.TempDir()

	t.Cleanup(func() { shipmentReconcileAppendWindowsTOCTOUHook = nil })
	hookCalled := false
	shipmentReconcileAppendWindowsTOCTOUHook = func() {
		hookCalled = true
		require.NoError(t, os.Remove(logsDir), "the original real logs directory must be removable (empty) to simulate the swap")
		if err := createReconcileTestJunction(t, logsDir, outside); err != nil {
			t.Skipf("junction creation skipped: %v", err)
		}
	}

	_, err := appendShipmentReconcileEventHandleRelative(logsDir, "toctou-item.jsonl", []byte(`{"x":1}`+"\n"))
	require.Error(t, err, "an intermediate-directory reparse-point swap between validation and open must be rejected")
	assert.True(t, hookCalled, "the TOCTOU simulation hook must have run")

	// CreateFile with OPEN_ALWAYS may itself create an empty (0-byte) file
	// object as a side effect of the open call, before the post-open
	// containment check below runs and fails the call — that is inherent to
	// the Windows CreateFile API and is not itself a data leak. What must
	// never happen is the actual event payload (eventBytes) landing in the
	// swapped-in target: the Write call is never reached because the
	// containment check returns an error first.
	entries, readErr := os.ReadDir(outside)
	require.NoError(t, readErr)
	for _, entry := range entries {
		info, infoErr := entry.Info()
		require.NoError(t, infoErr)
		assert.Zero(t, info.Size(), "any file left behind in the swapped-in junction target must be empty; the event payload must never be written")
	}
}

// TestAppendShipmentReconcileEventHandleRelative_PreResolutionSwapIsRejected is
// the append-writer counterpart of the archive pre-resolution swap test.
func TestAppendShipmentReconcileEventHandleRelative_PreResolutionSwapIsRejected(t *testing.T) {
	swaps := map[string]func(t *testing.T, base, outside string){
		"logs dir to symlink": func(t *testing.T, base, outside string) {
			logsDir := filepath.Join(base, "parent", "logs")
			require.NoError(t, os.Remove(logsDir))
			if err := os.Symlink(outside, logsDir); err != nil {
				t.Skipf("symlink creation skipped: %v", err)
			}
		},
		"parent dir to junction": func(t *testing.T, base, outside string) {
			require.NoError(t, os.Rename(filepath.Join(base, "parent"), filepath.Join(base, "parent.orig")))
			require.NoError(t, os.MkdirAll(filepath.Join(outside, "logs"), 0o755))
			if err := createReconcileTestJunction(t, filepath.Join(base, "parent"), outside); err != nil {
				t.Skipf("junction creation skipped: %v", err)
			}
		},
	}
	for name, swap := range swaps {
		t.Run(name, func(t *testing.T) {
			base, err := filepath.EvalSymlinks(t.TempDir())
			require.NoError(t, err)
			logsDir := filepath.Join(base, "parent", "logs")
			require.NoError(t, os.MkdirAll(logsDir, 0o755))
			outside, err := filepath.EvalSymlinks(t.TempDir())
			require.NoError(t, err)

			t.Cleanup(func() { shipmentReconcileAppendWindowsTOCTOUHook = nil })
			shipmentReconcileAppendWindowsTOCTOUHook = func() { swap(t, base, outside) }

			_, err = appendShipmentReconcileEventHandleRelative(logsDir, "pre-resolution.jsonl", []byte(`{"x":1}`+"\n"))
			require.Error(t, err)
			assertReconcileOutsideDirUntouched(t, outside)
		})
	}
}

// TestShipmentReconcileWindowsHooksAreNilInProduction proves the test-only
// hooks are never set outside tests.
func TestShipmentReconcileWindowsHooksAreNilInProduction(t *testing.T) {
	assert.Nil(t, shipmentReconcileFSWindowsTOCTOUHook)
	assert.Nil(t, shipmentReconcileAppendWindowsTOCTOUHook)
}

// TestAppendShipmentReconcileEventHandleRelative_SucceedsWithoutSwap is the
// control case: with the TOCTOU hook left nil (production behavior), a
// normal append against a stable logsDir must still succeed, proving the
// new snapshot/final-path re-validation does not itself break the
// legitimate path.
func TestAppendShipmentReconcileEventHandleRelative_SucceedsWithoutSwap(t *testing.T) {
	logsDir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err, "callers pass the canonical resolved directory")
	result, err := appendShipmentReconcileEventHandleRelative(logsDir, "control-item.jsonl", []byte(`{"x":1}`+"\n"))
	require.NoError(t, err)
	assert.Equal(t, int64(0), result.preAppendSize)

	got, readErr := os.ReadFile(filepath.Join(logsDir, "control-item.jsonl"))
	require.NoError(t, readErr)
	assert.Equal(t, "{\"x\":1}\n", string(got))
}

func TestAppendShipmentReconcileEvent_RefusesOutsidePointingLogsJunction(t *testing.T) {
	requireAppendRefusesOutsideLogsLink(t, func(logsDir, outside string) {
		if err := createReconcileTestJunction(t, logsDir, outside); err != nil {
			t.Skipf("junction unavailable: %v", err)
		}
	})
}
