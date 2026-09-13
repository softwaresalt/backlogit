//go:build windows

package events

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	backlogiterrors "github.com/softwaresalt/backlogit/internal/errors"
)

// createItemLogLockTestJunction creates a Windows directory junction from
// linkPath -> targetDir using mklink /J (no elevated privileges required),
// mirroring checkpoint_s1_u1_junction_windows_test.go's own createJunction
// helper (kept package-local here to avoid depending on _test.go-only
// symbols across files in a way that couples unrelated test suites).
func createItemLogLockTestJunction(t *testing.T, linkPath, targetDir string) error {
	t.Helper()
	out, err := exec.Command("cmd", "/c", "mklink", "/J", linkPath, targetDir).CombinedOutput()
	if err != nil {
		return fmt.Errorf("mklink /J failed: %v (%s)", err, out)
	}
	t.Cleanup(func() { _ = os.Remove(linkPath) })
	return nil
}

// TestOpenItemLogLockHandle_RejectsReparsePointNamespaceDirectory
// (Copilot PR #440 review, finding 3) proves openItemLogLockHandle now
// achieves parity with
// internal/core/shipment_reconcile_lock_windows.go's
// openShipmentReconcileLockHandleRelative: a namespace (parent) directory
// that is itself a reparse point must be rejected before the sidecar file
// is ever opened, rather than silently followed via
// FILE_FLAG_OPEN_REPARSE_POINT (which only protects the FINAL path
// component, lockPath itself, not its parent).
func TestOpenItemLogLockHandle_RejectsReparsePointNamespaceDirectory(t *testing.T) {
	outside := t.TempDir()
	base := t.TempDir()
	namespaceDir := filepath.Join(base, "itemlog")

	if err := createItemLogLockTestJunction(t, namespaceDir, outside); err != nil {
		t.Skipf("junction creation skipped: %v", err)
	}

	lockPath := filepath.Join(namespaceDir, ".deadbeef.lock")

	file, busy, err := openItemLogLockHandle(lockPath)
	require.Error(t, err, "a reparse-point namespace directory must be rejected")
	assert.False(t, busy)
	assert.Nil(t, file)
	assert.True(t, errors.Is(err, backlogiterrors.ErrValidation),
		"error must satisfy errors.Is(err, ErrValidation), got: %v", err)

	entries, readErr := os.ReadDir(outside)
	require.NoError(t, readErr)
	assert.Empty(t, entries, "nothing must be created through the reparse-point namespace directory")
}

// TestOpenItemLogLockHandle_SucceedsWithRegularNamespaceDirectory is the
// control case: a normal (non-reparse) namespace directory must continue to
// work exactly as before.
func TestOpenItemLogLockHandle_SucceedsWithRegularNamespaceDirectory(t *testing.T) {
	namespaceDir := t.TempDir()
	lockPath := filepath.Join(namespaceDir, ".control.lock")

	file, busy, err := openItemLogLockHandle(lockPath)
	require.NoError(t, err)
	assert.False(t, busy)
	require.NotNil(t, file)
	_ = file.Close()

	_, statErr := os.Stat(lockPath)
	assert.NoError(t, statErr, "the sidecar file must have been created")
}
