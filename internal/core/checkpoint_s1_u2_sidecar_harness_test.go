package core

// 153.002-T / S1 U2 — Sidecar create-only (no-replace) quarantine write.
//
// RED harnesses: these tests assert that the disposition sidecar write is
// create-only (O_EXCL-equivalent). Before the fix, atomicfile.WriteFileAtomic
// is an upsert that silently overwrites an existing sidecar — the assertions
// fail (RED). After the fix, the sidecar write uses a create-only path that
// refuses to clobber — tests PASS (GREEN).

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
)

// TestQuarantine_U2_S1_SidecarCollisionRefusesWithoutClobber (153.002-T /
// S1 U2) asserts that when a sidecar file already exists at the quarantine
// destination, QuarantineCheckpoint refuses the operation without clobbering
// the existing sidecar content.
//
// Setup: archive destination is vacant (no checkpoint file there yet), but
// the sidecar path is pre-occupied with sentinel content. Pre-fix: the
// upsert write overwrites the sidecar — the assertion that the original
// content is preserved FAILS (RED). Post-fix: the create-only write detects
// the collision, the MutationEnvelope compensation restores the source, and
// the original sidecar content is preserved (GREEN).
func TestQuarantine_U2_S1_SidecarCollisionRefusesWithoutClobber(t *testing.T) {
	ws := newCheckpointTargetTestWorkspace(t)
	cpDir := filepath.Join(ws.RootPath, ".backlogit", checkpointsSubdir)

	// Write a malformed source checkpoint (quarantine requires a malformed target).
	sourceName := "checkpoint-u2-sidecar-collision.json"
	malformedData := []byte("not-valid-json{u2-collision-test}")
	require.NoError(t, os.MkdirAll(cpDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(cpDir, sourceName), malformedData, 0o644))

	// Pre-create the sidecar at the quarantine destination with sentinel content.
	archiveDir := filepath.Join(ws.RootPath, ".backlogit", "archive", "checkpoints")
	require.NoError(t, os.MkdirAll(archiveDir, 0o755))
	destPath := filepath.Join(archiveDir, sourceName)
	sidecarPath := events.CheckpointDispositionSidecarPath(destPath)
	originalSidecarContent := []byte(`{"filename":"original-sidecar","disposition":"quarantined","reason":"pre-existing","operator":"original@example.com","disposition_at":"2026-01-01T00:00:00Z"}`)
	require.NoError(t, os.WriteFile(sidecarPath, originalSidecarContent, 0o644))
	// NOTE: archive destination (destPath) itself is NOT created — only the sidecar exists.

	ew := newDispositionEventWriter(t, ws)
	err := QuarantineCheckpoint(context.Background(), ws, ew, sourceName, "test-collision", "operator@example.com")

	// Post-fix: the operation must fail because the sidecar is pre-occupied.
	require.Error(t, err, "QuarantineCheckpoint must refuse when the sidecar is already occupied")

	// The original sidecar content must NOT have been overwritten.
	currentSidecar, readErr := os.ReadFile(sidecarPath)
	require.NoError(t, readErr, "sidecar must still exist at its original path")
	assert.Equal(t, originalSidecarContent, currentSidecar,
		"original sidecar content must be preserved (not clobbered by an upsert)")

	// The source checkpoint must be restored to its original location
	// (MutationEnvelope compensation unwound the move).
	assert.FileExists(t, filepath.Join(cpDir, sourceName),
		"source checkpoint must be restored by compensation when sidecar write fails")
}

// TestQuarantine_U2_S1_SidecarOccupiedReturnsDestinationOccupied asserts that the
// error returned on a sidecar collision satisfies ErrCheckpointDestinationOccupied.
func TestQuarantine_U2_S1_SidecarOccupiedReturnsDestinationOccupied(t *testing.T) {
	ws := newCheckpointTargetTestWorkspace(t)
	cpDir := filepath.Join(ws.RootPath, ".backlogit", checkpointsSubdir)

	sourceName := "checkpoint-u2-sidecar-occupied.json"
	require.NoError(t, os.MkdirAll(cpDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(cpDir, sourceName), []byte("bad-json{occupied}"), 0o644))

	archiveDir := filepath.Join(ws.RootPath, ".backlogit", "archive", "checkpoints")
	require.NoError(t, os.MkdirAll(archiveDir, 0o755))
	destPath := filepath.Join(archiveDir, sourceName)
	sidecarPath := events.CheckpointDispositionSidecarPath(destPath)
	require.NoError(t, os.WriteFile(sidecarPath, []byte(`{"filename":"prior"}`), 0o644))

	ew := newDispositionEventWriter(t, ws)
	err := QuarantineCheckpoint(context.Background(), ws, ew, sourceName, "sidecar-occupied", "operator@example.com")

	require.Error(t, err)
	// Post-fix: sidecar collision must be surfaced as ErrCheckpointDestinationOccupied.
	assert.True(t, errors.Is(err, blerrors.ErrCheckpointDestinationOccupied),
		"sidecar collision must return ErrCheckpointDestinationOccupied, got: %v", err)
}

// TestQuarantine_U2_S1_FreshSidecarSucceeds asserts that the normal quarantine
// path (no pre-existing sidecar) is unaffected by the U2 guard.
func TestQuarantine_U2_S1_FreshSidecarSucceeds(t *testing.T) {
	ws := newCheckpointTargetTestWorkspace(t)
	cpDir := filepath.Join(ws.RootPath, ".backlogit", checkpointsSubdir)

	sourceName := "checkpoint-u2-fresh.json"
	require.NoError(t, os.MkdirAll(cpDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(cpDir, sourceName), []byte("bad-json{fresh}"), 0o644))

	ew := newDispositionEventWriter(t, ws)
	err := QuarantineCheckpoint(context.Background(), ws, ew, sourceName, "fresh-test", "operator@example.com")

	require.NoError(t, err, "a fresh quarantine (no pre-existing sidecar) must succeed after U2")

	archiveDir := filepath.Join(ws.RootPath, ".backlogit", "archive", "checkpoints")
	destPath := filepath.Join(archiveDir, sourceName)
	assert.FileExists(t, destPath, "quarantined file must be at archive destination")
	assert.FileExists(t, events.CheckpointDispositionSidecarPath(destPath), "sidecar must be created alongside")
}
