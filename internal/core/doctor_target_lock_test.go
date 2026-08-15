package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDoctorTarget_AcquiresAndReleasesLock proves validation takes the per-task
// lock and releases its advisory handle; a second validation of the same file
// still succeeds (would fail/busy if the lock leaked).
func TestDoctorTarget_AcquiresAndReleasesLock(t *testing.T) {
	ws, queueDir := newTargetTestWorkspace(t)
	path := filepath.Join(queueDir, "100.001-T.md")
	require.NoError(t, os.WriteFile(path, []byte(validTaskContent), 0o644))

	res, err := DoctorTarget(ws, path)
	require.NoError(t, err)
	require.Equal(t, DoctorTargetPass, res.Kind)

	sidecar := taskLockSidecarPath(path)
	_, statErr := os.Stat(sidecar)
	assert.NoError(t, statErr, "stable lock sidecar must remain after validation")

	// Re-validation succeeds → the lock was truly released, not leaked.
	res2, err := DoctorTarget(ws, path)
	require.NoError(t, err)
	assert.Equal(t, DoctorTargetPass, res2.Kind)
}

// TestDoctorTarget_BusyWhenLockHeld proves that when the task lock is already
// held, validation returns the busy kind (mapped to exit 4 by U2) rather than
// blocking or performing a mid-write read.
func TestDoctorTarget_BusyWhenLockHeld(t *testing.T) {
	ws, queueDir := newTargetTestWorkspace(t)
	path := filepath.Join(queueDir, "100.001-T.md")
	require.NoError(t, os.WriteFile(path, []byte(validTaskContent), 0o644))

	// Hold the lock via the same primitive doctor uses.
	unlock, err := lockTaskFile(path)
	require.NoError(t, err)
	defer func() { _ = unlock() }()

	res, err := DoctorTarget(ws, path)
	require.NoError(t, err)
	assert.False(t, res.OK)
	assert.Equal(t, DoctorTargetBusy, res.Kind, "held lock must yield busy: %+v", res)
}
