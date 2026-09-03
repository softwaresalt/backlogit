package events_test

// 153.001-T / S1 U1 — Symlink rejection in read/mutate-lite verbs.
//
// RED harnesses: these tests assert that GetCheckpoint, GetCheckpointResult,
// and ResolveCheckpoint reject a symlinked checkpoint target with
// ErrCheckpointTargetUnsafe. Before the fix, ensurePathContained does not
// evaluate symlinks, so both leaf-symlink and intermediate-dir-symlink reads
// are silently followed — these assertions FAIL (RED). After the fix, the
// shared seam walks every path component with os.Lstat and rejects on any
// symlink — tests PASS (GREEN).
//
// All tests skip on Windows: symlink creation requires elevated privileges on
// Windows CI runners. The production guard uses os.Lstat on every platform.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	backlogiterrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
)

// validU1CheckpointBody is a minimal schema-valid V1 checkpoint payload used
// by every harness test in this file.
const validU1CheckpointBody = `{"schema_version":1,"agent":"ship","session_id":"u1-s1-test","phase":"build","status":"active","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}`

// TestGetCheckpoint_U1_S1_LeafSymlinkRejected asserts that GetCheckpoint
// returns ErrCheckpointTargetUnsafe when the named checkpoint file is a
// symlink pointing outside the checkpoint directory. RED: before the fix the
// symlink is followed silently and the target is returned successfully.
func TestGetCheckpoint_U1_S1_LeafSymlinkRejected(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires elevated privileges on Windows CI runners")
	}

	// Write a real file outside the checkpoint directory.
	realFile := filepath.Join(t.TempDir(), "real-checkpoint.json")
	require.NoError(t, os.WriteFile(realFile, []byte(validU1CheckpointBody), 0o644))

	// Create the checkpoint directory and a symlink in it pointing to the real file.
	cpDir := t.TempDir()
	symlinkName := "checkpoint-leaf-symlink.json"
	require.NoError(t, os.Symlink(realFile, filepath.Join(cpDir, symlinkName)))

	_, err := events.GetCheckpoint(context.Background(), cpDir, symlinkName)

	require.Error(t, err, "GetCheckpoint on a leaf-symlink must return an error")
	assert.True(t, errors.Is(err, backlogiterrors.ErrCheckpointTargetUnsafe),
		"error must satisfy errors.Is(err, ErrCheckpointTargetUnsafe), got: %v", err)
}

// TestGetCheckpointResult_U1_S1_LeafSymlinkRejected asserts that
// GetCheckpointResult returns ErrCheckpointTargetUnsafe on a leaf-symlink.
func TestGetCheckpointResult_U1_S1_LeafSymlinkRejected(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires elevated privileges on Windows CI runners")
	}

	realFile := filepath.Join(t.TempDir(), "real-checkpoint.json")
	require.NoError(t, os.WriteFile(realFile, []byte(validU1CheckpointBody), 0o644))

	cpDir := t.TempDir()
	symlinkName := "checkpoint-result-leaf-symlink.json"
	require.NoError(t, os.Symlink(realFile, filepath.Join(cpDir, symlinkName)))

	_, err := events.GetCheckpointResult(context.Background(), cpDir, symlinkName)

	require.Error(t, err, "GetCheckpointResult on a leaf-symlink must return an error")
	assert.True(t, errors.Is(err, backlogiterrors.ErrCheckpointTargetUnsafe),
		"error must satisfy errors.Is(err, ErrCheckpointTargetUnsafe), got: %v", err)
}

// TestResolveCheckpoint_U1_S1_LeafSymlinkRejected asserts that
// ResolveCheckpoint returns ErrCheckpointTargetUnsafe on a leaf-symlink.
func TestResolveCheckpoint_U1_S1_LeafSymlinkRejected(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires elevated privileges on Windows CI runners")
	}

	realFile := filepath.Join(t.TempDir(), "real-checkpoint.json")
	require.NoError(t, os.WriteFile(realFile, []byte(validU1CheckpointBody), 0o644))

	cpDir := t.TempDir()
	symlinkName := "checkpoint-resolve-leaf-symlink.json"
	require.NoError(t, os.Symlink(realFile, filepath.Join(cpDir, symlinkName)))

	err := events.ResolveCheckpoint(context.Background(), cpDir, symlinkName)

	require.Error(t, err, "ResolveCheckpoint on a leaf-symlink must return an error")
	assert.True(t, errors.Is(err, backlogiterrors.ErrCheckpointTargetUnsafe),
		"error must satisfy errors.Is(err, ErrCheckpointTargetUnsafe), got: %v", err)
}

// TestGetCheckpoint_U1_S1_IntermediateDirSymlinkRejected asserts that
// GetCheckpoint returns ErrCheckpointTargetUnsafe when the checkpoint
// directory path itself is a symlink (an intermediate directory symlink in
// the access path). RED: before the fix the symlinked dir is silently
// traversed. After the fix the chain check detects the symlink.
func TestGetCheckpoint_U1_S1_IntermediateDirSymlinkRejected(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires elevated privileges on Windows CI runners")
	}

	// Real checkpoint directory: write a valid checkpoint there.
	realDir := t.TempDir()
	cpName := "checkpoint-via-symlinked-dir.json"
	require.NoError(t, os.WriteFile(filepath.Join(realDir, cpName), []byte(validU1CheckpointBody), 0o644))

	// Create a symlink that points to the real directory.
	symlinkParent := t.TempDir()
	symlinkDir := filepath.Join(symlinkParent, "checkpoints-link")
	require.NoError(t, os.Symlink(realDir, symlinkDir))

	// Access the checkpoint through the symlinked dir.
	_, err := events.GetCheckpoint(context.Background(), symlinkDir, cpName)

	require.Error(t, err, "GetCheckpoint through a symlinked directory must return an error")
	assert.True(t, errors.Is(err, backlogiterrors.ErrCheckpointTargetUnsafe),
		"error must satisfy errors.Is(err, ErrCheckpointTargetUnsafe), got: %v", err)
}

// TestGetCheckpoint_U1_S1_NonSymlinkedPathAccepted asserts that a legitimate
// (non-symlinked) checkpoint read is unaffected by the U1 guard.
func TestGetCheckpoint_U1_S1_NonSymlinkedPathAccepted(t *testing.T) {
	cpDir := t.TempDir()
	cpName := "checkpoint-real.json"
	require.NoError(t, os.WriteFile(filepath.Join(cpDir, cpName), []byte(validU1CheckpointBody), 0o644))

	cp, err := events.GetCheckpoint(context.Background(), cpDir, cpName)

	require.NoError(t, err, "a non-symlinked checkpoint read must succeed after U1")
	assert.NotNil(t, cp)
}
