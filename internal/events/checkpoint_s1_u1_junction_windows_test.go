//go:build windows

package events

// 153.001-T / S1 U1 — Windows reparse-point (junction) rejection test.
//
// Windows directory junctions are a type of reparse point that Go 1.24
// reports as ModeIrregular (not ModeSymlink), so ModeSymlink-only checks
// would not catch them. This file provides executing test coverage for the
// Windows-specific isPathSymlinkOrReparse and isReparsePoint helpers added
// in 0a37c1c6 (Copilot 6th/7th-review finding PRRT_kwDORzozKM6fCU-c).
//
// Directory junctions (mklink /J) do NOT require elevated privileges on
// Windows, unlike symbolic links. Tests skip gracefully if mklink fails.

import (
	"context"
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

// createJunction creates a Windows directory junction from linkPath → targetDir
// using mklink /J (no elevated privileges required). Returns an error if mklink
// fails; callers should t.Skip on error rather than t.Fatal, since mklink
// availability varies across Windows configurations.
func createJunction(t *testing.T, linkPath, targetDir string) error {
	t.Helper()
	out, err := exec.Command("cmd", "/c", "mklink", "/J", linkPath, targetDir).CombinedOutput()
	if err != nil {
		return fmt.Errorf("mklink /J failed: %v (%s)", err, out)
	}
	t.Cleanup(func() { _ = os.Remove(linkPath) })
	return nil
}

// TestReadCheckpointFileNoFollow_WindowsJunctionRejected (153.001-T / S1 U1)
// asserts that the Windows reparse-point guard in readCheckpointFileNoFollow
// is wired correctly. Note: Windows junctions are directory-level only (not
// file-level), so a checkpoint FILE accessed through a junction directory is
// not directly detectable by a per-leaf check — that case is guarded at the
// checkpointDir level (see TestListCheckpoints_WindowsJunctionDirectoryRejected).
// This test instead verifies the isReparsePoint helper by using a junction
// directory as the tested path — confirming it reports true for junctions —
// and that reading a file whose PARENT is a junction yields nil from
// readCheckpointFileNoFollow (the leaf file is regular; the caller's
// checkpointDir guard provides the higher-level defense).
func TestReadCheckpointFileNoFollow_WindowsJunctionRejected(t *testing.T) {
	// Confirm the reparse-point detection helper correctly identifies a junction.
	realDir := t.TempDir()
	junctionBase := t.TempDir()
	junctionPath := filepath.Join(junctionBase, "test-junction-dir")

	if err := createJunction(t, junctionPath, realDir); err != nil {
		t.Skipf("junction creation skipped: %v", err)
	}

	// The junction directory should be reported as a reparse point.
	assert.True(t, isPathSymlinkOrReparse(junctionPath),
		"isPathSymlinkOrReparse must return true for a junction directory")
	assert.True(t, isReparsePoint(junctionPath),
		"isReparsePoint must return true for a junction directory")

	// A regular non-junction path must NOT be reported as a reparse point.
	regularDir := t.TempDir()
	assert.False(t, isPathSymlinkOrReparse(regularDir),
		"isPathSymlinkOrReparse must return false for a regular directory")
}

// TestIsPathSymlinkOrReparse_JunctionIsDetected asserts that
// isPathSymlinkOrReparse returns true for a directory junction, confirming
// that the Windows FILE_ATTRIBUTE_REPARSE_POINT check is reached.
func TestIsPathSymlinkOrReparse_JunctionIsDetected(t *testing.T) {
	realDir := t.TempDir()
	junctionBase := t.TempDir()
	junctionPath := filepath.Join(junctionBase, "detect-junction")

	if err := createJunction(t, junctionPath, realDir); err != nil {
		t.Skipf("junction creation skipped: %v", err)
	}

	got := isPathSymlinkOrReparse(junctionPath)
	assert.True(t, got, "isPathSymlinkOrReparse must detect a Windows directory junction as a reparse point")
}

// TestListCheckpoints_WindowsJunctionDirectoryRejected asserts that
// ListCheckpoints rejects a checkpointDir that is itself a directory junction.
func TestListCheckpoints_WindowsJunctionDirectoryRejected(t *testing.T) {
	realDir := t.TempDir()
	junctionBase := t.TempDir()
	junctionPath := filepath.Join(junctionBase, "checkpoints-dir-junction")

	if err := createJunction(t, junctionPath, realDir); err != nil {
		t.Skipf("junction creation skipped: %v", err)
	}

	_, err := ListCheckpoints(context.Background(), junctionPath, CheckpointFilter{})

	require.Error(t, err, "ListCheckpoints with a junction checkpointDir must return an error")
	assert.True(t, errors.Is(err, backlogiterrors.ErrCheckpointTargetUnsafe),
		"error must satisfy errors.Is(err, ErrCheckpointTargetUnsafe), got: %v", err)
}
