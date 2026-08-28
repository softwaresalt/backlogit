package events

import (
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	backlogiterrors "github.com/softwaresalt/backlogit/internal/errors"
)

// U12 — Guarded rewrite seam: contract harness (147-F / 147.035-T,
// red-deliverable). No production change. This is the seam's BEHAVIOUR red:
// it compiles against U11's landed declaration and fails because the seam
// carries no validity and no conformance precondition yet — U11's own
// harness is source-shape and gates only the declared signature. U13
// (147.036-T) turns all three cases green.
//
// Verdict contract: the seam returns the raw verdict errors
// (ErrCheckpointCorrupt, ErrCheckpointInvalid, *CheckpointNonConformingError)
// rather than choosing a verb-facing sentinel; wrapping is the caller's job.

func noopMutate(*CheckpointV1) error { return nil }

// TestU12_UnparseableRefusedWithCorruptAndByteIdentity asserts an
// unparseable document is refused with ErrCheckpointCorrupt and the file
// bytes are SHA-identical afterwards.
func TestU12_UnparseableRefusedWithCorruptAndByteIdentity(t *testing.T) {
	dir := t.TempDir()
	name := "checkpoint-u12-unparseable.json"
	path := filepath.Join(dir, name)
	body := []byte("not-json{")
	require.NoError(t, os.WriteFile(path, body, 0o644))
	before := sha256.Sum256(body)

	err := RewriteCheckpointFile(context.Background(), dir, name, noopMutate)

	require.Error(t, err)
	assert.True(t, errors.Is(err, backlogiterrors.ErrCheckpointCorrupt))

	after, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	assert.Equal(t, before, sha256.Sum256(after))
}

// TestU12_SchemaInvalidRefusedWithInvalidAndByteIdentity asserts a parseable
// but schema-invalid document is refused with ErrCheckpointInvalid and the
// bytes are SHA-identical. It also asserts a mutate error propagates rather
// than being silently absorbed.
func TestU12_SchemaInvalidRefusedWithInvalidAndByteIdentity(t *testing.T) {
	dir := t.TempDir()
	name := "checkpoint-u12-schema-invalid.json"
	path := filepath.Join(dir, name)
	body := []byte(`{"status":"active"}`)
	require.NoError(t, os.WriteFile(path, body, 0o644))
	before := sha256.Sum256(body)

	err := RewriteCheckpointFile(context.Background(), dir, name, noopMutate)

	require.Error(t, err)
	assert.True(t, errors.Is(err, backlogiterrors.ErrCheckpointInvalid))

	after, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	assert.Equal(t, before, sha256.Sum256(after))
}

// TestU12_NonConformingRefusedNamingOffendersWithByteIdentity asserts a
// valid-but-non-conforming document is refused with
// *CheckpointNonConformingError naming the offender paths and the bytes are
// SHA-identical.
func TestU12_NonConformingRefusedNamingOffendersWithByteIdentity(t *testing.T) {
	dir := t.TempDir()
	name := "checkpoint-u12-nonconforming.json"
	path := filepath.Join(dir, name)
	body := []byte(`{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","status":"active",` +
		`"created_at":"2026-08-24T00:00:00Z","updated_at":"2026-08-24T00:00:00Z","extra_key":"x"}`)
	require.NoError(t, os.WriteFile(path, body, 0o644))
	before := sha256.Sum256(body)

	err := RewriteCheckpointFile(context.Background(), dir, name, noopMutate)

	require.Error(t, err)
	var typed *backlogiterrors.CheckpointNonConformingError
	require.True(t, errors.As(err, &typed))
	assert.Contains(t, typed.Fields, "extra_key")

	after, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	assert.Equal(t, before, sha256.Sum256(after))
}

// TestRewriteCheckpointFile_AcceptedMutationPreservesExistingMode is a
// regression test (found during 130-S adversarial review): the seam wrote
// its replacement via syncWriteFileAtomic(path, updated, 0o644), which
// always applies a hardcoded 0644 regardless of the target's existing
// mode, silently widening a more restrictive checkpoint (e.g. 0600) on
// every accepted rewrite. atomicfile.WriteFileAtomic — already used
// elsewhere in this codebase — preserves the destination's existing mode.
// POSIX permission bits are not represented on Windows filesystems, so this
// assertion is skipped there, matching internal/atomicfile's own
// TestWriteFileAtomic_OverwritePreservesExistingMode convention.
func TestRewriteCheckpointFile_AcceptedMutationPreservesExistingMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits are not represented on Windows filesystems")
	}
	dir := t.TempDir()
	name := "checkpoint-u12-mode-preserved.json"
	path := filepath.Join(dir, name)
	body := []byte(`{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","status":"active",` +
		`"created_at":"2026-08-24T00:00:00Z","updated_at":"2026-08-24T00:00:00Z"}`)
	require.NoError(t, os.WriteFile(path, body, 0o600))

	err := RewriteCheckpointFile(context.Background(), dir, name, func(cp *CheckpointV1) error {
		cp.Status = "resolved"
		return nil
	})
	require.NoError(t, err, "a valid, conforming document must be accepted")

	info, statErr := os.Stat(path)
	require.NoError(t, statErr)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(),
		"an accepted rewrite must preserve the source 0600 mode, not a hardcoded 0644")
}

// TestRewriteCheckpointFile_ContentChangedDuringMutateRefusesWrite is a
// regression test (found during 130-S adversarial review): the seam
// checked conformance against the bytes read at the top of the function,
// then committed an unconditional write with no re-verification. A
// concurrent writer could add an unmodeled key to the file between that
// read and the write, and the seam would silently overwrite it — the exact
// evidence-loss condition this feature exists to prevent. The mutate
// callback is a deterministic injection point for this scenario: it runs
// after the conformance check and before the write, so mutating the
// on-disk file from inside it simulates a concurrent writer without a real
// race. The seam must now detect the change and refuse rather than
// overwrite, leaving the (externally rewritten) file exactly as the
// "concurrent writer" left it.
func TestRewriteCheckpointFile_ContentChangedDuringMutateRefusesWrite(t *testing.T) {
	dir := t.TempDir()
	name := "checkpoint-race-during-mutate.json"
	path := filepath.Join(dir, name)
	original := []byte(`{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","status":"active",` +
		`"created_at":"2026-08-24T00:00:00Z","updated_at":"2026-08-24T00:00:00Z"}`)
	require.NoError(t, os.WriteFile(path, original, 0o644))

	concurrentlyWritten := []byte(`{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","status":"active",` +
		`"created_at":"2026-08-24T00:00:00Z","updated_at":"2026-08-24T00:00:00Z","injected_by_race":"x"}`)

	err := RewriteCheckpointFile(context.Background(), dir, name, func(cp *CheckpointV1) error {
		// Simulate a concurrent writer landing between this seam's initial
		// read/conformance-check and its write.
		return os.WriteFile(path, concurrentlyWritten, 0o644)
	})

	require.Error(t, err, "the seam must refuse to overwrite content that changed after classification")
	assert.ErrorIs(t, err, backlogiterrors.ErrCheckpointContentChanged)

	after, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	assert.Equal(t, concurrentlyWritten, after,
		"a refused rewrite must leave the concurrently-written content untouched, not overwrite or restore it")
}
