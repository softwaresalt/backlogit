package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
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

// TestAbandonCheckpoint_NoHTMLEscape is a regression test for the checkpoint
// JSON readability fix (137-F): AbandonCheckpoint's in-place rewrite must not
// HTML-escape <, >, and & characters in the rewritten checkpoint bytes.
func TestAbandonCheckpoint_NoHTMLEscape(t *testing.T) {
	ws := newCheckpointTargetTestWorkspace(t)
	dir := filepath.Join(ws.RootPath, ".backlogit", checkpointsSubdir)

	cp := validDispositionTestCheckpoint()
	cp.ResumeHint = "a > b && b < c"
	writeDispositionCheckpoint(t, dir, "checkpoint-escape.json", cp)

	ew := newDispositionEventWriter(t, ws)
	require.NoError(t, AbandonCheckpoint(context.Background(), ws, ew, "checkpoint-escape.json", "test reason", "operator@example.com"))

	data, err := os.ReadFile(filepath.Join(dir, "checkpoint-escape.json"))
	require.NoError(t, err)
	s := string(data)

	assert.Contains(t, s, "a > b && b < c")
	assert.NotContains(t, s, `\u003e`)
	assert.NotContains(t, s, `\u003c`)
	assert.NotContains(t, s, `\u0026`)
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

// TestCheckpointDisposition_IndeterminateAuditLeavesTargetUnmoved is the
// companion to TestCheckpointDisposition_FailedAuditLeavesTargetUnmoved: it
// exercises the ErrWriteIndeterminate branch (not just ErrWriteNotApplied) by
// overriding the checkpointAuditAppendFn seam to actually append the audit
// event (mirroring a real indeterminate outcome: bytes committed, durability
// confirmation uncertain) and then return the indeterminate sentinel. Both
// disposition verbs must refuse and leave their target completely untouched
// — nothing moved, nothing rewritten — matching the "either failure class
// leaves the target unmoved" protected invariant.
//
// Must not run with t.Parallel(): overrides the package-level
// checkpointAuditAppendFn seam.
func TestCheckpointDisposition_IndeterminateAuditLeavesTargetUnmoved(t *testing.T) {
	ws := newCheckpointTargetTestWorkspace(t)
	dir := filepath.Join(ws.RootPath, ".backlogit", checkpointsSubdir)

	orig := checkpointAuditAppendFn
	checkpointAuditAppendFn = func(ctx context.Context, ew *events.EventWriter, event events.Event) error {
		// Real append first so the "possibly-applied" indeterminate semantics
		// are faithfully simulated, then surface the sentinel.
		_ = ew.AppendEvent(ctx, event)
		return fmt.Errorf("simulated post-write fsync failure: %w", blerrors.ErrWriteIndeterminate)
	}
	t.Cleanup(func() { checkpointAuditAppendFn = orig })

	ew := newDispositionEventWriter(t, ws)

	t.Run("abandon", func(t *testing.T) {
		validData := writeDispositionCheckpoint(t, dir, "checkpoint-abandon-indet.json", validDispositionTestCheckpoint())
		beforeSum := sha256.Sum256(validData)

		err := AbandonCheckpoint(context.Background(), ws, ew, "checkpoint-abandon-indet.json", "reason", "operator@example.com")
		require.Error(t, err)
		assert.ErrorIs(t, err, blerrors.ErrCheckpointAuditIndeterminate)

		after, ok := hashFile(t, filepath.Join(dir, "checkpoint-abandon-indet.json"))
		require.True(t, ok, "target must remain in place")
		assert.Equal(t, hex.EncodeToString(beforeSum[:]), after, "target bytes must be unchanged on indeterminate audit outcome")
	})

	t.Run("quarantine", func(t *testing.T) {
		malformed := writeMalformedCheckpoint(t, dir, "checkpoint-quarantine-indet.json")
		beforeSum := sha256.Sum256(malformed)

		err := QuarantineCheckpoint(context.Background(), ws, ew, "checkpoint-quarantine-indet.json", "reason", "operator@example.com")
		require.Error(t, err)
		assert.ErrorIs(t, err, blerrors.ErrCheckpointAuditIndeterminate)

		after, ok := hashFile(t, filepath.Join(dir, "checkpoint-quarantine-indet.json"))
		require.True(t, ok, "target must remain in place, unmoved")
		assert.Equal(t, hex.EncodeToString(beforeSum[:]), after, "target bytes must be unchanged on indeterminate audit outcome")

		destPath := filepath.Join(ws.RootPath, ".backlogit", "archive", "checkpoints", "checkpoint-quarantine-indet.json")
		assert.NoFileExists(t, destPath, "nothing must be quarantined when audit outcome is indeterminate")
	})
}

// TestResolveDispositionTarget_RejectsSymlinkedCheckpointsDir asserts that a
// symlinked checkpoints directory (not merely a symlinked leaf file) is
// refused, closing the gap where the directory itself could redirect a
// disposition action to a file living elsewhere under .backlogit.
func TestResolveDispositionTarget_RejectsSymlinkedCheckpointsDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires elevated privileges on Windows CI runners")
	}
	root := t.TempDir()
	backlogitDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogitDir, 0o755))
	require.NoError(t, config.WriteDefaults(backlogitDir))
	ctx := context.Background()
	ws, err := NewWorkspace(ctx, root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ws.Close() })

	realDir := filepath.Join(backlogitDir, "real-checkpoints")
	require.NoError(t, os.MkdirAll(realDir, 0o755))
	require.NoError(t, os.Symlink(realDir, filepath.Join(backlogitDir, checkpointsSubdir)))

	_, err = ResolveDispositionTarget(ws, "checkpoint-x.json")
	require.Error(t, err)
	assert.ErrorIs(t, err, blerrors.ErrCheckpointTargetUnsafe)
}

// TestResolveDispositionTarget_RejectsSymlinkedStorageRoot asserts that a
// symlinked .backlogit directory (not merely a symlinked checkpoints
// subdirectory) pointing entirely outside the workspace root is refused. A
// symlinked storage root would otherwise let confineToStorageRoot's
// containment check pass trivially (the target is "contained" within
// wherever .backlogit points), because that check is relative to
// WorkspaceStorageRoot's own resolved location.
func TestResolveDispositionTarget_RejectsSymlinkedStorageRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires elevated privileges on Windows CI runners")
	}
	root := t.TempDir()
	outsideDir := t.TempDir() // a sibling temp dir, entirely outside root
	require.NoError(t, os.MkdirAll(filepath.Join(outsideDir, checkpointsSubdir), 0o755))

	// .backlogit itself is a symlink pointing outside the workspace root.
	require.NoError(t, os.Symlink(outsideDir, filepath.Join(root, ".backlogit")))

	ws := &Workspace{RootPath: root}
	_, err := ResolveDispositionTarget(ws, "checkpoint-x.json")
	require.Error(t, err)
	assert.ErrorIs(t, err, blerrors.ErrCheckpointTargetUnsafe)
}

// ── 136.014-T: TOCTOU classify-then-move race ─────────────────────────────────

// TestQuarantineCheckpoint_ContentChangedRefusesMove verifies the TOCTOU fix:
// if the source file is replaced with valid JSON after classification but before
// the link, QuarantineCheckpoint must refuse with ErrCheckpointContentChanged.
// Must not run with t.Parallel: overrides checkpointAuditAppendFn seam.
func TestQuarantineCheckpoint_ContentChangedRefusesMove(t *testing.T) {
	ws := newCheckpointTargetTestWorkspace(t)
	dir := filepath.Join(ws.RootPath, ".backlogit", checkpointsSubdir)
	writeMalformedCheckpoint(t, dir, "checkpoint-racey.json")

	// After classification (but before the move), swap the file for a valid one.
	orig := checkpointAuditAppendFn
	checkpointAuditAppendFn = func(ctx context.Context, ew *events.EventWriter, event events.Event) error {
		if err := orig(ctx, ew, event); err != nil {
			return err
		}
		// Overwrite the source with a valid checkpoint, simulating a race.
		valid := validDispositionTestCheckpoint()
		data, _ := json.Marshal(valid)
		_ = os.WriteFile(filepath.Join(dir, "checkpoint-racey.json"), data, 0o644)
		return nil
	}
	t.Cleanup(func() { checkpointAuditAppendFn = orig })

	ew := newDispositionEventWriter(t, ws)
	err := QuarantineCheckpoint(context.Background(), ws, ew, "checkpoint-racey.json", "corrupt", "operator@example.com")
	require.Error(t, err)
	assert.ErrorIs(t, err, blerrors.ErrCheckpointContentChanged, "must refuse when content changed since classification")

	// Source must NOT have been moved.
	assert.FileExists(t, filepath.Join(dir, "checkpoint-racey.json"), "source must remain when content changed")
	destPath := filepath.Join(ws.RootPath, ".backlogit", "archive", "checkpoints", "checkpoint-racey.json")
	assert.NoFileExists(t, destPath, "nothing must be quarantined when content changed")
}

// TestQuarantineCheckpoint_StableContentProceedsNormally verifies that when
// the file content has NOT changed since classification, the hash check passes
// and quarantine completes normally.
func TestQuarantineCheckpoint_StableContentProceedsNormally(t *testing.T) {
	ws := newCheckpointTargetTestWorkspace(t)
	dir := filepath.Join(ws.RootPath, ".backlogit", checkpointsSubdir)
	writeMalformedCheckpoint(t, dir, "checkpoint-stable.json")

	ew := newDispositionEventWriter(t, ws)
	require.NoError(t, QuarantineCheckpoint(context.Background(), ws, ew, "checkpoint-stable.json", "corrupt", "operator@example.com"))
	assert.NoFileExists(t, filepath.Join(dir, "checkpoint-stable.json"))
	assert.FileExists(t, filepath.Join(ws.RootPath, ".backlogit", "archive", "checkpoints", "checkpoint-stable.json"))
}

// ── 136.015-T: Surface moveNoReplace unwind failure ───────────────────────────

// TestMoveNoReplace_JoinsBothErrorsWhenUnwindFails verifies that when both the
// source removal and the dst unwind removal fail, errors.Join is used and both
// errors are present in the returned error (not one silently discarded), and
// ErrWriteIndeterminate is included so callers can detect the indeterminate state.
// Must not run with t.Parallel: overrides osRemove seam.
func TestMoveNoReplace_JoinsBothErrorsWhenUnwindFails(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	src := filepath.Join(srcDir, "src.json")
	dst := filepath.Join(dstDir, "dst.json")
	require.NoError(t, os.WriteFile(src, []byte("payload"), 0o644))

	// Inject seam: first call (Remove src) returns srcErr; second call (unwind
	// Remove dst) returns dstErr. Both errors must appear in the joined result
	// alongside ErrWriteIndeterminate.
	srcErr := fmt.Errorf("injected src remove failure")
	dstRemoveErr := fmt.Errorf("injected dst remove failure")
	calls := 0
	osRemove = func(_ string) error {
		calls++
		if calls == 1 {
			return srcErr
		}
		return dstRemoveErr
	}
	t.Cleanup(func() { osRemove = os.Remove })

	err := moveNoReplace(src, dst, nil, nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, srcErr, "joined error must contain the src remove failure")
	assert.ErrorIs(t, err, dstRemoveErr, "joined error must contain the dst unwind remove failure")
	assert.ErrorIs(t, err, blerrors.ErrWriteIndeterminate, "joined error must include ErrWriteIndeterminate sentinel")
}

// TestQuarantineCheckpoint_UnwindFailureSurfacedAsIndeterminate verifies that
// when moveNoReplace encounters both src-remove and dst-unwind-remove failures
// the joined error includes ErrWriteIndeterminate, and that error propagates
// through QuarantineCheckpoint so callers can detect the indeterminate state.
// Must not run with t.Parallel: overrides osRemove seam.
func TestQuarantineCheckpoint_UnwindFailureSurfacedAsIndeterminate(t *testing.T) {
	ws := newCheckpointTargetTestWorkspace(t)
	dir := filepath.Join(ws.RootPath, ".backlogit", checkpointsSubdir)
	writeMalformedCheckpoint(t, dir, "checkpoint-unwind.json")

	// Inject seam: all Remove calls return an error, causing both the src and
	// dst removals in moveNoReplace to fail. The joined error must include
	// ErrWriteIndeterminate, which MutationEnvelope propagates as "indeterminate"
	// and QuarantineCheckpoint surfaces to the caller.
	osRemove = func(_ string) error {
		return fmt.Errorf("injected remove failure")
	}
	t.Cleanup(func() { osRemove = os.Remove })

	ew := newDispositionEventWriter(t, ws)
	err := QuarantineCheckpoint(context.Background(), ws, ew, "checkpoint-unwind.json", "corrupt", "operator@example.com")
	require.Error(t, err)
	assert.ErrorIs(t, err, blerrors.ErrWriteIndeterminate,
		"double remove failure must surface as ErrWriteIndeterminate through QuarantineCheckpoint")
}

// ── 136.016-T: Durable writes — fsync dirs after move ─────────────────────────

// TestQuarantineCheckpoint_DurableWritesFsyncsBothDirs asserts that when
// durable_writes is enabled, QuarantineCheckpoint fsyncs both the destination
// directory and the source directory after the successful link+remove.
// Must not run with t.Parallel: overrides mkdirDirSyncFn/enabled seams.
func TestQuarantineCheckpoint_DurableWritesFsyncsBothDirs(t *testing.T) {
	ws := newCheckpointTargetTestWorkspace(t)
	ws.Config.DurableWrites = true
	dir := filepath.Join(ws.RootPath, ".backlogit", checkpointsSubdir)
	writeMalformedCheckpoint(t, dir, "checkpoint-durable.json")

	destDir := filepath.Join(ws.RootPath, ".backlogit", "archive", "checkpoints")

	synced := recordDirSyncs(t, "")

	ew := newDispositionEventWriter(t, ws)
	require.NoError(t, QuarantineCheckpoint(context.Background(), ws, ew, "checkpoint-durable.json", "corrupt", "operator@example.com"))

	assert.Contains(t, *synced, destDir, "durable quarantine must fsync the destination directory")
	assert.Contains(t, *synced, dir, "durable quarantine must fsync the source directory")
}

// TestQuarantineCheckpoint_DurableOffSkipsDirFsync asserts that when
// durable_writes is disabled, no directory fsync is performed.
// Must not run with t.Parallel: overrides mkdirDirSyncFn/enabled seams.
func TestQuarantineCheckpoint_DurableOffSkipsDirFsync(t *testing.T) {
	ws := newCheckpointTargetTestWorkspace(t)
	// DurableWrites stays false (default).
	dir := filepath.Join(ws.RootPath, ".backlogit", checkpointsSubdir)
	writeMalformedCheckpoint(t, dir, "checkpoint-nondurable.json")

	synced := recordDirSyncs(t, "")

	ew := newDispositionEventWriter(t, ws)
	require.NoError(t, QuarantineCheckpoint(context.Background(), ws, ew, "checkpoint-nondurable.json", "corrupt", "operator@example.com"))

	assert.Empty(t, *synced, "durable-off quarantine must not fsync any directory")
}

// TestQuarantineCheckpoint_DurableDstFsyncFailureIsIndeterminate asserts that a
// destination-dir fsync failure after a successful link+remove is classified as
// ErrWriteIndeterminate (the move succeeded, durability is uncertain).
// Must not run with t.Parallel: overrides mkdirDirSyncFn/enabled seams.
func TestQuarantineCheckpoint_DurableDstFsyncFailureIsIndeterminate(t *testing.T) {
	ws := newCheckpointTargetTestWorkspace(t)
	ws.Config.DurableWrites = true
	dir := filepath.Join(ws.RootPath, ".backlogit", checkpointsSubdir)
	writeMalformedCheckpoint(t, dir, "checkpoint-dst-fsync-fail.json")

	destDir := filepath.Join(ws.RootPath, ".backlogit", "archive", "checkpoints")
	// Ensure the archive/checkpoints dir exists before recording syncs (MkdirAll
	// also fsyncs when durable, and we want to capture only the move-time syncs).
	require.NoError(t, os.MkdirAll(destDir, 0o755))

	recordDirSyncs(t, destDir) // fail the destination-dir fsync

	ew := newDispositionEventWriter(t, ws)
	err := QuarantineCheckpoint(context.Background(), ws, ew, "checkpoint-dst-fsync-fail.json", "corrupt", "operator@example.com")
	require.Error(t, err)
	assert.True(t, blerrors.IsWriteIndeterminate(err),
		"destination-dir fsync failure after successful move must be ErrWriteIndeterminate")
}

// TestQuarantineCheckpoint_DurableSrcFsyncFailureIsIndeterminate asserts that a
// source-dir fsync failure (after dst-dir fsync succeeded) is also classified as
// ErrWriteIndeterminate.
// Must not run with t.Parallel: overrides mkdirDirSyncFn/enabled seams.
func TestQuarantineCheckpoint_DurableSrcFsyncFailureIsIndeterminate(t *testing.T) {
	ws := newCheckpointTargetTestWorkspace(t)
	ws.Config.DurableWrites = true
	dir := filepath.Join(ws.RootPath, ".backlogit", checkpointsSubdir)
	writeMalformedCheckpoint(t, dir, "checkpoint-src-fsync-fail.json")

	// Pre-create the archive dir so its fsync during MkdirAll does not interfere.
	destDir := filepath.Join(ws.RootPath, ".backlogit", "archive", "checkpoints")
	require.NoError(t, os.MkdirAll(destDir, 0o755))

	recordDirSyncs(t, dir) // fail the source-dir fsync

	ew := newDispositionEventWriter(t, ws)
	err := QuarantineCheckpoint(context.Background(), ws, ew, "checkpoint-src-fsync-fail.json", "corrupt", "operator@example.com")
	require.Error(t, err)
	assert.True(t, blerrors.IsWriteIndeterminate(err),
		"source-dir fsync failure after successful move must be ErrWriteIndeterminate")
}

// ── Existing test (preserved) ──────────────────────────────────────────────────

// TestAbandonCheckpoint_RefusesNonActiveNonAbandonedStatus is a U6-contract
// regression: abandon requires an active checkpoint (the already-abandoned
// case is the sole idempotent exception). A "resolved" checkpoint must be
// refused with ErrCheckpointNotActive rather than silently rewritten to
// "abandoned".
func TestAbandonCheckpoint_RefusesNonActiveNonAbandonedStatus(t *testing.T) {
	ws := newCheckpointTargetTestWorkspace(t)
	dir := filepath.Join(ws.RootPath, ".backlogit", checkpointsSubdir)

	resolvedCP := validDispositionTestCheckpoint()
	resolvedCP.Status = "resolved"
	writeDispositionCheckpoint(t, dir, "checkpoint-resolved.json", resolvedCP)

	ew := newDispositionEventWriter(t, ws)
	err := AbandonCheckpoint(context.Background(), ws, ew, "checkpoint-resolved.json", "reason", "operator@example.com")
	require.Error(t, err)
	assert.ErrorIs(t, err, blerrors.ErrCheckpointNotActive)

	data, readErr := os.ReadFile(filepath.Join(dir, "checkpoint-resolved.json"))
	require.NoError(t, readErr)
	cp, parseErr := events.ParseCheckpoint(data)
	require.NoError(t, parseErr)
	assert.Equal(t, "resolved", cp.Status, "status must remain unchanged when abandon is refused")
	assert.Empty(t, cp.Disposition, "disposition must not be set when abandon is refused")
}

// TestAbandonCheckpointMutate_RefusesWhenSeamReadShowsResolved is a
// regression test (found during 130-S adversarial review): AbandonCheckpoint
// classifies status against its own initial read, but
// events.RewriteCheckpointFile performs an independent read of its own. If a
// concurrent ResolveCheckpoint wins between those two reads, the seam's own
// read reflects the newly resolved document — a state the CAS check alone
// cannot catch, since nothing changes between the seam's read and its write
// in that scenario. abandonCheckpointMutate (the exact callback
// AbandonCheckpoint passes to the seam) must refuse rather than
// unconditionally overwrite the resolved document with
// disposition:"abandoned". Calling events.RewriteCheckpointFile directly
// with the checkpoint already reflecting a post-race resolved state
// exercises exactly what the seam's own read would observe in that race,
// without needing a real concurrent goroutine.
func TestAbandonCheckpointMutate_RefusesWhenSeamReadShowsResolved(t *testing.T) {
	dir := t.TempDir()
	name := "checkpoint-abandon-mutate-resolved.json"
	resolvedCP := validDispositionTestCheckpoint()
	resolvedCP.Status = "resolved"
	body := writeDispositionCheckpoint(t, dir, name, resolvedCP)

	mutate := abandonCheckpointMutate("reason", "operator@example.com", time.Now().UTC())
	err := events.RewriteCheckpointFile(context.Background(), dir, name, mutate)

	require.Error(t, err, "the abandon mutate callback must refuse a document the seam's own read shows resolved")
	assert.ErrorIs(t, err, blerrors.ErrCheckpointNotActive)

	after, readErr := os.ReadFile(filepath.Join(dir, name))
	require.NoError(t, readErr)
	assert.Equal(t, body, after, "a refused rewrite must leave the resolved document's bytes untouched")
}

// TestU4_ValidButNonConformingActiveRefusedWithNonConforming asserts a
// valid-but-non-conforming active document is refused with
// ErrCheckpointNonConforming naming the offending keys (147-F / U4). This
// already holds via U14b's seam-level check at the write step, so it is a
// guard rather than a red function; U4's own red is the ordering — see
// TestU4_RefusalLeavesAuditJSONLByteUnchanged and
// TestU4_NonConformingAlreadyAbandonedReturnsNonConforming below.
func TestU4Guard_ValidButNonConformingActiveRefusedWithNonConforming(t *testing.T) {
	ws := newCheckpointTargetTestWorkspace(t)
	dir := filepath.Join(ws.RootPath, ".backlogit", checkpointsSubdir)
	name := "checkpoint-u4-nonconforming.json"
	body := []byte(`{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","status":"active",` +
		`"created_at":"2026-08-24T00:00:00Z","updated_at":"2026-08-24T00:00:00Z","extra_key":"x"}`)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), body, 0o644))

	ew := newDispositionEventWriter(t, ws)
	err := AbandonCheckpoint(context.Background(), ws, ew, name, "reason", "operator@example.com")

	require.Error(t, err)
	var typed *blerrors.CheckpointNonConformingError
	require.True(t, errors.As(err, &typed))
	assert.Contains(t, typed.Fields, "extra_key")
}

// TestU4_RefusalLeavesAuditJSONLByteUnchanged asserts the disposition audit
// JSONL is byte-unchanged after a non-conforming refusal (the gate is a
// non-writing refusal, placed before the audit append).
func TestU4_RefusalLeavesAuditJSONLByteUnchanged(t *testing.T) {
	ws := newCheckpointTargetTestWorkspace(t)
	dir := filepath.Join(ws.RootPath, ".backlogit", checkpointsSubdir)
	name := "checkpoint-u4-audit-unchanged.json"
	body := []byte(`{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","status":"active",` +
		`"created_at":"2026-08-24T00:00:00Z","updated_at":"2026-08-24T00:00:00Z","extra_key":"x"}`)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), body, 0o644))

	logPath := filepath.Join(WorkspaceLogsRoot(ws.RootPath), "checkpoint-disposition-audit.jsonl")
	beforeData, _ := os.ReadFile(logPath) // may not exist yet; nil is fine

	ew := newDispositionEventWriter(t, ws)
	err := AbandonCheckpoint(context.Background(), ws, ew, name, "reason", "operator@example.com")
	require.Error(t, err)

	afterData, readErr := os.ReadFile(logPath)
	if readErr != nil {
		require.True(t, os.IsNotExist(readErr))
		assert.Empty(t, beforeData)
	} else {
		assert.Equal(t, beforeData, afterData)
	}
}

// TestU4_NonConformingAlreadyAbandonedReturnsNonConforming asserts a
// non-conforming already-abandoned document returns
// ErrCheckpointNonConforming rather than nil — the conformance gate runs
// BEFORE the already-abandoned short-circuit.
func TestU4_NonConformingAlreadyAbandonedReturnsNonConforming(t *testing.T) {
	ws := newCheckpointTargetTestWorkspace(t)
	dir := filepath.Join(ws.RootPath, ".backlogit", checkpointsSubdir)
	name := "checkpoint-u4-abandoned-nonconforming.json"
	body := []byte(`{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","status":"abandoned",` +
		`"disposition":"abandoned","disposition_reason":"stale","disposition_operator":"ship",` +
		`"disposition_at":"2026-08-24T00:00:00Z",` +
		`"created_at":"2026-08-24T00:00:00Z","updated_at":"2026-08-24T00:00:00Z","extra_key":"x"}`)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), body, 0o644))

	ew := newDispositionEventWriter(t, ws)
	err := AbandonCheckpoint(context.Background(), ws, ew, name, "reason", "operator@example.com")

	require.Error(t, err, "must not silently no-op (return nil) for a non-conforming already-abandoned document")
	var typed *blerrors.CheckpointNonConformingError
	require.True(t, errors.As(err, &typed))
}

// TestU17_AbandonValidationWrapPreservesErrCheckpointInvalid asserts
// AbandonCheckpoint's validation-failure wrap keeps ErrCheckpointInvalid
// traversable via errors.Is (147-F / U17). The shipped wrap uses
// fmt.Errorf("%w: %v", ErrCheckpointUseQuarantine, valErr) — the %v verb
// drops the sentinel ValidateCheckpoint returns.
func TestU17_AbandonValidationWrapPreservesErrCheckpointInvalid(t *testing.T) {
	ws := newCheckpointTargetTestWorkspace(t)
	dir := filepath.Join(ws.RootPath, ".backlogit", checkpointsSubdir)

	cp := validDispositionTestCheckpoint()
	cp.Agent = "not-a-real-agent" // fails the `oneof=ship stage` validator tag
	writeDispositionCheckpoint(t, dir, "checkpoint-u17-invalid-agent.json", cp)

	ew := newDispositionEventWriter(t, ws)
	err := AbandonCheckpoint(context.Background(), ws, ew, "checkpoint-u17-invalid-agent.json", "reason", "operator@example.com")

	require.Error(t, err)
	assert.ErrorIs(t, err, blerrors.ErrCheckpointUseQuarantine)
	assert.ErrorIs(t, err, blerrors.ErrCheckpointInvalid,
		"the %%v verb drops ErrCheckpointInvalid from the wrap; must be multi-%%w")
}

// TestU17Guard_MessageTextPreserved pins that the underlying validator
// message text survives the corrected multi-%w wrap unchanged (green on
// landing, committed with the implementation).
func TestU17Guard_MessageTextPreserved(t *testing.T) {
	ws := newCheckpointTargetTestWorkspace(t)
	dir := filepath.Join(ws.RootPath, ".backlogit", checkpointsSubdir)

	cp := validDispositionTestCheckpoint()
	cp.Agent = "not-a-real-agent"
	writeDispositionCheckpoint(t, dir, "checkpoint-u17-message.json", cp)

	parsedForValidation := *cp
	valErr := events.ValidateCheckpoint(&parsedForValidation)
	require.Error(t, valErr)

	ew := newDispositionEventWriter(t, ws)
	err := AbandonCheckpoint(context.Background(), ws, ew, "checkpoint-u17-message.json", "reason", "operator@example.com")

	require.Error(t, err)
	assert.Contains(t, err.Error(), valErr.Error(),
		"the validator's message text must survive the corrected wrap unchanged")
}

// TestU5_ValidNonConformingActiveAcceptedByQuarantine asserts the
// accept-half of scenario 1 (147-F / U5): a valid-but-non-conforming active
// document — schema-valid but carrying an unmodeled top-level key — is
// ACCEPTED by QuarantineCheckpoint rather than refused with
// ErrCheckpointUseAbandon. Without this widening, such a document is
// refused by both abandon (U4, non-conforming) and quarantine (thinks it is
// "valid"), leaving it with no disposition path at all.
func TestU5_ValidNonConformingActiveAcceptedByQuarantine(t *testing.T) {
	ws := newCheckpointTargetTestWorkspace(t)
	dir := filepath.Join(ws.RootPath, ".backlogit", checkpointsSubdir)
	name := "checkpoint-u5-nonconforming.json"
	body := []byte(`{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","status":"active",` +
		`"created_at":"2026-08-24T00:00:00Z","updated_at":"2026-08-24T00:00:00Z","extra_key":"x"}`)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), body, 0o644))

	ew := newDispositionEventWriter(t, ws)
	err := QuarantineCheckpoint(context.Background(), ws, ew, name, "non-conforming", "operator@example.com")
	require.NoError(t, err, "a valid-but-non-conforming active document must be accepted by quarantine")

	assert.NoFileExists(t, filepath.Join(dir, name))
	assert.FileExists(t, filepath.Join(ws.RootPath, ".backlogit", "archive", "checkpoints", name))
}

// TestU5Guard_ConformingActiveRefusedByQuarantine pins scenario 2
// (unchanged, already-shipped behaviour): a conforming active document is
// still refused by quarantine with ErrCheckpointUseAbandon.
func TestU5Guard_ConformingActiveRefusedByQuarantine(t *testing.T) {
	ws := newCheckpointTargetTestWorkspace(t)
	dir := filepath.Join(ws.RootPath, ".backlogit", checkpointsSubdir)
	name := "checkpoint-u5-conforming.json"
	writeDispositionCheckpoint(t, dir, name, validDispositionTestCheckpoint())

	ew := newDispositionEventWriter(t, ws)
	err := QuarantineCheckpoint(context.Background(), ws, ew, name, "reason", "operator@example.com")
	require.Error(t, err)
	assert.ErrorIs(t, err, blerrors.ErrCheckpointUseAbandon)

	assert.FileExists(t, filepath.Join(dir, name), "a refused quarantine must not move the source")
}

// TestU5Guard_ArchivedBytesByteIdenticalToPreQuarantineOriginal asserts
// scenario 1's postcondition: once quarantine accepts a valid-but-non-
// conforming active document, the archived bytes are byte-identical to the
// pre-quarantine original — quarantine remains a verbatim move, never a
// rewrite, even for the newly-widened accept case.
func TestU5Guard_ArchivedBytesByteIdenticalToPreQuarantineOriginal(t *testing.T) {
	ws := newCheckpointTargetTestWorkspace(t)
	dir := filepath.Join(ws.RootPath, ".backlogit", checkpointsSubdir)
	name := "checkpoint-u5-byteidentical.json"
	body := []byte(`{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","status":"active",` +
		`"created_at":"2026-08-24T00:00:00Z","updated_at":"2026-08-24T00:00:00Z","extra_key":"x"}`)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), body, 0o644))

	ew := newDispositionEventWriter(t, ws)
	require.NoError(t, QuarantineCheckpoint(context.Background(), ws, ew, name, "non-conforming", "operator@example.com"))

	archived, readErr := os.ReadFile(filepath.Join(ws.RootPath, ".backlogit", "archive", "checkpoints", name))
	require.NoError(t, readErr)
	assert.Equal(t, body, archived, "archived bytes must be byte-identical to the pre-quarantine original")
}

// TestAbandonCheckpoint_ParseFailureWrapPreservesErrCheckpointCorrupt is a
// regression test (found during 130-S adversarial review): AbandonCheckpoint
// wraps a ParseCheckpoint failure as fmt.Errorf("%w: %v", ErrCheckpointUseQuarantine,
// parseErr) — the %v verb drops the ErrCheckpointCorrupt sentinel ParseCheckpoint
// itself returns, the same class of bug U17 already fixed for the sibling
// validation-failure wrap two lines below.
func TestAbandonCheckpoint_ParseFailureWrapPreservesErrCheckpointCorrupt(t *testing.T) {
	ws := newCheckpointTargetTestWorkspace(t)
	dir := filepath.Join(ws.RootPath, ".backlogit", checkpointsSubdir)
	name := "checkpoint-abandon-unparseable.json"
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("not-json{"), 0o644))

	ew := newDispositionEventWriter(t, ws)
	err := AbandonCheckpoint(context.Background(), ws, ew, name, "reason", "operator@example.com")

	require.Error(t, err)
	assert.ErrorIs(t, err, blerrors.ErrCheckpointUseQuarantine)
	assert.ErrorIs(t, err, blerrors.ErrCheckpointCorrupt,
		"the %%v verb drops ErrCheckpointCorrupt from the wrap; must be multi-%%w")
}
