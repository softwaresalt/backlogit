package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
)

// validDispositionTestCheckpoint returns a schema-valid CheckpointV1 for
// disposition tests.
func validDispositionTestCheckpoint() *events.CheckpointV1 {
	now := time.Now().UTC()
	return &events.CheckpointV1{
		SchemaVersion: 1,
		Agent:         "ship",
		SessionID:     "disposition-test-session",
		Phase:         "build",
		Status:        "active",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

func writeDispositionCheckpoint(t *testing.T, dir, filename string, cp *events.CheckpointV1) []byte {
	t.Helper()
	data, err := json.Marshal(cp)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, filename), data, 0o644))
	return data
}

func writeMalformedCheckpoint(t *testing.T, dir, filename string) []byte {
	t.Helper()
	data := []byte("not-valid-json{")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, filename), data, 0o644))
	return data
}

func hashFile(t *testing.T, path string) (string, bool) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false
		}
		require.NoError(t, err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), true
}

func newDispositionEventWriter(t *testing.T, ws *Workspace) *events.EventWriter {
	t.Helper()
	logsDir := WorkspaceLogsRoot(ws.RootPath)
	return NewWorkspaceEventWriter(ws, logsDir)
}

// TestAbandonCheckpoint_MutatesOnlyNamedTarget is a protected-invariant test:
// hashing every file in the checkpoints directory before and after
// AbandonCheckpoint must show exactly one changed file — the named target.
func TestAbandonCheckpoint_MutatesOnlyNamedTarget(t *testing.T) {
	ws := newCheckpointTargetTestWorkspace(t)
	dir := filepath.Join(ws.RootPath, ".backlogit", checkpointsSubdir)

	writeDispositionCheckpoint(t, dir, "checkpoint-target.json", validDispositionTestCheckpoint())
	writeDispositionCheckpoint(t, dir, "checkpoint-control.json", validDispositionTestCheckpoint())

	before := map[string]string{}
	for _, name := range []string{"checkpoint-target.json", "checkpoint-control.json"} {
		h, ok := hashFile(t, filepath.Join(dir, name))
		require.True(t, ok)
		before[name] = h
	}

	ew := newDispositionEventWriter(t, ws)
	require.NoError(t, AbandonCheckpoint(context.Background(), ws, ew, "checkpoint-target.json", "test reason", "operator@example.com"))

	changed := 0
	for _, name := range []string{"checkpoint-target.json", "checkpoint-control.json"} {
		h, ok := hashFile(t, filepath.Join(dir, name))
		require.True(t, ok, "file %s must still exist in place (abandon rewrites in place, never moves)", name)
		if h != before[name] {
			changed++
			assert.Equal(t, "checkpoint-target.json", name, "only the named target may change")
		}
	}
	assert.Equal(t, 1, changed, "exactly one file must have changed")
}

// TestQuarantineCheckpoint_BytesByteIdentical is a protected-invariant test:
// the quarantined file's bytes must be byte-identical to the original.
func TestQuarantineCheckpoint_BytesByteIdentical(t *testing.T) {
	ws := newCheckpointTargetTestWorkspace(t)
	dir := filepath.Join(ws.RootPath, ".backlogit", checkpointsSubdir)

	original := writeMalformedCheckpoint(t, dir, "checkpoint-malformed.json")
	originalSum := sha256.Sum256(original)

	ew := newDispositionEventWriter(t, ws)
	require.NoError(t, QuarantineCheckpoint(context.Background(), ws, ew, "checkpoint-malformed.json", "corrupt", "operator@example.com"))

	// Source must no longer exist at its original path.
	assert.NoFileExists(t, filepath.Join(dir, "checkpoint-malformed.json"))

	destPath := filepath.Join(ws.RootPath, ".backlogit", "archive", "checkpoints", "checkpoint-malformed.json")
	quarantined, err := os.ReadFile(destPath)
	require.NoError(t, err)
	quarantinedSum := sha256.Sum256(quarantined)
	assert.Equal(t, originalSum, quarantinedSum, "quarantined bytes must be byte-identical to the original")

	// A disposition sidecar record must exist alongside the quarantined file.
	assert.FileExists(t, destPath+".disposition.json")
}

// TestAbandonCheckpoint_MalformedRefusesNamingQuarantine asserts the
// disjoint-verb contract: abandon on a malformed target is refused and names
// quarantine as the correct verb.
func TestAbandonCheckpoint_MalformedRefusesNamingQuarantine(t *testing.T) {
	ws := newCheckpointTargetTestWorkspace(t)
	dir := filepath.Join(ws.RootPath, ".backlogit", checkpointsSubdir)
	writeMalformedCheckpoint(t, dir, "checkpoint-malformed.json")

	ew := newDispositionEventWriter(t, ws)
	err := AbandonCheckpoint(context.Background(), ws, ew, "checkpoint-malformed.json", "reason", "operator@example.com")
	require.Error(t, err)
	assert.ErrorIs(t, err, blerrors.ErrCheckpointUseQuarantine)
	assert.FileExists(t, filepath.Join(dir, "checkpoint-malformed.json"), "malformed target must remain unmoved on refusal")
}

// TestQuarantineCheckpoint_ValidRefusesNamingAbandon asserts the disjoint-verb
// contract: quarantine on a valid target is refused and names abandon as the
// correct verb.
func TestQuarantineCheckpoint_ValidRefusesNamingAbandon(t *testing.T) {
	ws := newCheckpointTargetTestWorkspace(t)
	dir := filepath.Join(ws.RootPath, ".backlogit", checkpointsSubdir)
	writeDispositionCheckpoint(t, dir, "checkpoint-valid.json", validDispositionTestCheckpoint())

	ew := newDispositionEventWriter(t, ws)
	err := QuarantineCheckpoint(context.Background(), ws, ew, "checkpoint-valid.json", "reason", "operator@example.com")
	require.Error(t, err)
	assert.ErrorIs(t, err, blerrors.ErrCheckpointUseAbandon)
	assert.FileExists(t, filepath.Join(dir, "checkpoint-valid.json"), "valid target must remain unmoved on refusal")
}

func TestAbandonCheckpoint_EmptyReasonRefused(t *testing.T) {
	ws := newCheckpointTargetTestWorkspace(t)
	dir := filepath.Join(ws.RootPath, ".backlogit", checkpointsSubdir)
	writeDispositionCheckpoint(t, dir, "checkpoint-valid.json", validDispositionTestCheckpoint())

	ew := newDispositionEventWriter(t, ws)
	err := AbandonCheckpoint(context.Background(), ws, ew, "checkpoint-valid.json", "", "operator@example.com")
	require.Error(t, err)
	assert.ErrorIs(t, err, blerrors.ErrCheckpointReasonRequired)
}

func TestAbandonCheckpoint_EmptyOperatorRefused(t *testing.T) {
	ws := newCheckpointTargetTestWorkspace(t)
	dir := filepath.Join(ws.RootPath, ".backlogit", checkpointsSubdir)
	writeDispositionCheckpoint(t, dir, "checkpoint-valid.json", validDispositionTestCheckpoint())

	ew := newDispositionEventWriter(t, ws)
	err := AbandonCheckpoint(context.Background(), ws, ew, "checkpoint-valid.json", "reason", "")
	require.Error(t, err)
	assert.ErrorIs(t, err, blerrors.ErrCheckpointOperatorRequired)
}

func TestQuarantineCheckpoint_EmptyReasonRefused(t *testing.T) {
	ws := newCheckpointTargetTestWorkspace(t)
	dir := filepath.Join(ws.RootPath, ".backlogit", checkpointsSubdir)
	writeMalformedCheckpoint(t, dir, "checkpoint-malformed.json")

	ew := newDispositionEventWriter(t, ws)
	err := QuarantineCheckpoint(context.Background(), ws, ew, "checkpoint-malformed.json", "", "operator@example.com")
	require.Error(t, err)
	assert.ErrorIs(t, err, blerrors.ErrCheckpointReasonRequired)
}

func TestQuarantineCheckpoint_EmptyOperatorRefused(t *testing.T) {
	ws := newCheckpointTargetTestWorkspace(t)
	dir := filepath.Join(ws.RootPath, ".backlogit", checkpointsSubdir)
	writeMalformedCheckpoint(t, dir, "checkpoint-malformed.json")

	ew := newDispositionEventWriter(t, ws)
	err := QuarantineCheckpoint(context.Background(), ws, ew, "checkpoint-malformed.json", "reason", "")
	require.Error(t, err)
	assert.ErrorIs(t, err, blerrors.ErrCheckpointOperatorRequired)
}

// TestAbandonCheckpoint_AlreadyAbandonedIsIdempotent asserts a repeat abandon
// call is a no-op that preserves the original disposition reason.
func TestAbandonCheckpoint_AlreadyAbandonedIsIdempotent(t *testing.T) {
	ws := newCheckpointTargetTestWorkspace(t)
	dir := filepath.Join(ws.RootPath, ".backlogit", checkpointsSubdir)
	writeDispositionCheckpoint(t, dir, "checkpoint-valid.json", validDispositionTestCheckpoint())

	ew := newDispositionEventWriter(t, ws)
	ctx := context.Background()
	require.NoError(t, AbandonCheckpoint(ctx, ws, ew, "checkpoint-valid.json", "first reason", "operator-1@example.com"))

	// Second call with different reason/operator must be a no-op preserving the original.
	require.NoError(t, AbandonCheckpoint(ctx, ws, ew, "checkpoint-valid.json", "second reason", "operator-2@example.com"))

	data, err := os.ReadFile(filepath.Join(dir, "checkpoint-valid.json"))
	require.NoError(t, err)
	cp, err := events.ParseCheckpoint(data)
	require.NoError(t, err)
	assert.Equal(t, "first reason", cp.DispositionReason)
	assert.Equal(t, "operator-1@example.com", cp.DispositionOperator)
}

// TestQuarantineCheckpoint_DestinationOccupiedRefused asserts the
// clobber-refuse guard: a pre-existing file at the quarantine destination
// blocks the move rather than being overwritten.
func TestQuarantineCheckpoint_DestinationOccupiedRefused(t *testing.T) {
	ws := newCheckpointTargetTestWorkspace(t)
	dir := filepath.Join(ws.RootPath, ".backlogit", checkpointsSubdir)
	writeMalformedCheckpoint(t, dir, "checkpoint-malformed.json")

	destDir := filepath.Join(ws.RootPath, ".backlogit", "archive", "checkpoints")
	require.NoError(t, os.MkdirAll(destDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(destDir, "checkpoint-malformed.json"), []byte("pre-existing"), 0o644))

	ew := newDispositionEventWriter(t, ws)
	err := QuarantineCheckpoint(context.Background(), ws, ew, "checkpoint-malformed.json", "reason", "operator@example.com")
	require.Error(t, err)
	assert.ErrorIs(t, err, blerrors.ErrCheckpointDestinationOccupied)
	assert.FileExists(t, filepath.Join(dir, "checkpoint-malformed.json"), "source must remain unmoved when destination is occupied")
}

// TestCheckpointDisposition_FailedAuditLeavesTargetUnmoved asserts the
// protected invariant that a failed audit append (either failure class)
// leaves the target file untouched — nothing moved, nothing rewritten.
func TestCheckpointDisposition_FailedAuditLeavesTargetUnmoved(t *testing.T) {
	ws := newCheckpointTargetTestWorkspace(t)
	dir := filepath.Join(ws.RootPath, ".backlogit", checkpointsSubdir)

	// Force the audit append to fail by pointing the EventWriter's logsDir at
	// a path that is blocked by a regular file, so MkdirAll fails before any
	// bytes are appended (ErrWriteNotApplied class).
	blockedLogsDir := filepath.Join(t.TempDir(), "not-a-dir")
	require.NoError(t, os.WriteFile(blockedLogsDir, []byte("blocking file"), 0o644))
	brokenEW := events.NewEventWriter(blockedLogsDir)

	t.Run("abandon", func(t *testing.T) {
		validData := writeDispositionCheckpoint(t, dir, "checkpoint-abandon-audit-fail.json", validDispositionTestCheckpoint())
		beforeSum := sha256.Sum256(validData)

		err := AbandonCheckpoint(context.Background(), ws, brokenEW, "checkpoint-abandon-audit-fail.json", "reason", "operator@example.com")
		require.Error(t, err)
		assert.ErrorIs(t, err, blerrors.ErrCheckpointAuditNotApplied)

		after, ok := hashFile(t, filepath.Join(dir, "checkpoint-abandon-audit-fail.json"))
		require.True(t, ok, "target must remain in place")
		assert.Equal(t, hex.EncodeToString(beforeSum[:]), after, "target bytes must be unchanged when audit fails")
	})

	t.Run("quarantine", func(t *testing.T) {
		malformed := writeMalformedCheckpoint(t, dir, "checkpoint-quarantine-audit-fail.json")
		beforeSum := sha256.Sum256(malformed)

		err := QuarantineCheckpoint(context.Background(), ws, brokenEW, "checkpoint-quarantine-audit-fail.json", "reason", "operator@example.com")
		require.Error(t, err)
		assert.ErrorIs(t, err, blerrors.ErrCheckpointAuditNotApplied)

		after, ok := hashFile(t, filepath.Join(dir, "checkpoint-quarantine-audit-fail.json"))
		require.True(t, ok, "target must remain in place, unmoved")
		assert.Equal(t, hex.EncodeToString(beforeSum[:]), after, "target bytes must be unchanged when audit fails")

		destPath := filepath.Join(ws.RootPath, ".backlogit", "archive", "checkpoints", "checkpoint-quarantine-audit-fail.json")
		assert.NoFileExists(t, destPath, "nothing must be quarantined when audit fails")
	})
}
