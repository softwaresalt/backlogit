package events_test

import (
	"context"
	"embed"
	"errors"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	backlogiterrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
)

//go:embed testdata/legacy-corpus/*.json
var legacyCorpusFS embed.FS

// liveCorpusQuarantineBaseline is the known set of live checkpoint filenames
// under .backlogit/checkpoints that already need quarantine as of this
// commit (146.009-T / U3c). Every one of them is a pre-schema_version legacy
// document, so every one is expected to fail ValidateCheckpoint on the
// required schema_version field. This baseline is what makes the live-corpus
// assertion meaningful: it guards against 146.006-T (U2) or 146.011-T (U4)
// causing files OUTSIDE this known set to newly need quarantine, without
// requiring every future addition to the live corpus to be enumerated here.
var liveCorpusQuarantineBaseline = map[string]bool{
	"checkpoint-20260406-171334.json": true,
	"checkpoint-20260411-051040.json": true,
	"checkpoint-20260421-164238.json": true,
	"checkpoint-20260424-162622.json": true,
	"checkpoint-20260424-174043.json": true,
	"checkpoint-20260424-204116.json": true,
	"checkpoint-20260426-031618.json": true,
	"checkpoint-20260426-045333.json": true,
	"checkpoint-20260801-051014.json": true,
}

// TestCreateCheckpoint_ValidV1WithUnknownKeys_ListedUnflagged is scenario 1 of
// 146.009-T (U3c), superseded by 146.011-T (U4): a complete, schema-valid V1
// dump that also carries an unknown top-level key is now REJECTED at create
// (the closed schema namespace), so it never reaches ListCheckpoints at all.
// Before U4 this scenario asserted the opposite (create succeeded and the
// resulting file was listed unflagged); U4's create-boundary closed-namespace
// check makes that pre-U4 behavior obsolete by design, so this test is
// updated in place rather than left asserting a superseded contract.
func TestCreateCheckpoint_ValidV1WithUnknownKeys_ListedUnflagged(t *testing.T) {
	dir := t.TempDir()
	stateDump := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","status":"active",` +
		`"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z",` +
		`"extra_diagnostic":"should not affect validation"}`

	_, err := events.CreateCheckpoint(context.Background(), dir, stateDump)
	require.Error(t, err, "post-146.011-T (U4) an unknown top-level key must reject create")
	var typed *backlogiterrors.CheckpointUnknownFieldError
	require.True(t, errors.As(err, &typed), "the rejection must be recoverable as *CheckpointUnknownFieldError")
	assert.Equal(t, []string{"extra_diagnostic"}, typed.Fields)

	summaries, err := events.ListCheckpoints(context.Background(), dir, events.CheckpointFilter{})
	require.NoError(t, err)
	assert.Empty(t, summaries, "a rejected create must write no file, so nothing is listed")
}

// TestListCheckpoints_LegacyCorpus_MatchesGoldenTriple is scenario 2 of
// 146.009-T (U3c): the synthetic legacy fixtures are materialized into
// t.TempDir(), listed through ListCheckpoints, and every entry's
// (NeedsQuarantine, validation-error class, RemediationCommand) triple is
// identical to the pre-U2 golden table. The guard is that 146.006-T (U2) and
// 146.011-T (U4) change no classification, not that legacy files are clean:
// every fixture here lacks schema_version and is therefore expected to need
// quarantine with an ErrCheckpointInvalid-class validation error.
//
// Fixture provenance: every fixture under testdata/legacy-corpus/ is
// hand-written synthetic content modelled on shapes observed in the live
// corpus (unmodeled keys such as pr_number, ci_status, review_gate,
// items_blocked, follow_up_tasks). No bytes are copied from
// .backlogit/checkpoints, and this test never writes to that directory.
func TestListCheckpoints_LegacyCorpus_MatchesGoldenTriple(t *testing.T) {
	entries, err := legacyCorpusFS.ReadDir("testdata/legacy-corpus")
	require.NoError(t, err)
	require.NotEmpty(t, entries, "the synthetic legacy-corpus fixture set must not be empty")

	dir := t.TempDir()
	for _, e := range entries {
		data, err := legacyCorpusFS.ReadFile(path.Join("testdata/legacy-corpus", e.Name()))
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(dir, "checkpoint-"+e.Name()), data, 0o644))
	}

	summaries, err := events.ListCheckpoints(context.Background(), dir, events.CheckpointFilter{})
	require.NoError(t, err)
	require.Len(t, summaries, len(entries), "one summary per enumerated fixture: an added or removed fixture without updated expectations must fail this test")

	for _, s := range summaries {
		assert.True(t, s.NeedsQuarantine, "fixture %s: every legacy (schema_version-less) fixture is expected to need quarantine (pre-U2 golden baseline)", s.Filename)
		assert.NotEmpty(t, s.ValidationErr, "fixture %s: expected a non-empty ErrCheckpointInvalid-class validation error", s.Filename)
		assert.NotEmpty(t, s.RemediationCommand, "fixture %s: NeedsQuarantine implies a non-empty RemediationCommand", s.Filename)
	}
}

// TestListCheckpoints_LiveCorpus_NoNewQuarantineEntries is the live-corpus
// half of scenario 2: the real .backlogit/checkpoints directory is listed
// READ-ONLY (ListCheckpoints never mutates) and asserted to report no NEW
// NeedsQuarantine entries relative to liveCorpusQuarantineBaseline. This is
// what actually guards the real files, as distinct from the synthetic
// fixture table above.
func TestListCheckpoints_LiveCorpus_NoNewQuarantineEntries(t *testing.T) {
	repoRoot, err := os.Getwd()
	require.NoError(t, err)
	liveDir := filepath.Join(repoRoot, "..", "..", ".backlogit", "checkpoints")
	info, statErr := os.Stat(liveDir)
	if statErr != nil || !info.IsDir() {
		t.Skip("live .backlogit/checkpoints directory not present in this environment")
	}

	before, err := os.ReadDir(liveDir)
	require.NoError(t, err)

	summaries, err := events.ListCheckpoints(context.Background(), liveDir, events.CheckpointFilter{})
	require.NoError(t, err)

	for _, s := range summaries {
		if s.NeedsQuarantine && !liveCorpusQuarantineBaseline[s.Filename] {
			t.Errorf("live checkpoint %s newly needs quarantine; it is not in the recorded baseline", s.Filename)
		}
	}

	after, err := os.ReadDir(liveDir)
	require.NoError(t, err)
	assert.Equal(t, len(before), len(after), "ListCheckpoints must never add or remove files from the live corpus (read-only)")
}

// TestCreateCheckpoint_MalformedCreatedAt_IsCorruptNotUnknownField is part of
// scenario 3 of 146.009-T (U3c): a V1 dump with a malformed created_at still
// fails with ErrCheckpointCorrupt, never ErrCheckpointUnknownField, pinning
// the misclassification guard.
func TestCreateCheckpoint_MalformedCreatedAt_IsCorruptNotUnknownField(t *testing.T) {
	dir := t.TempDir()
	stateDump := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","status":"active","created_at":"not-a-timestamp","updated_at":"2026-01-01T00:00:00Z"}`

	_, err := events.CreateCheckpoint(context.Background(), dir, stateDump)
	require.Error(t, err)
	assert.True(t, errors.Is(err, backlogiterrors.ErrCheckpointCorrupt), "a malformed created_at must be classified as ErrCheckpointCorrupt")
	assert.False(t, errors.Is(err, backlogiterrors.ErrCheckpointUnknownField), "a malformed created_at must never be misclassified as an unknown-field rejection")
}

// TestCreateCheckpoint_ResumeHintAccepted is the tag-option-stripping half of
// scenario 3 of 146.009-T (U3c): a dump carrying resume_hint — a modeled key
// whose tag carries the ",omitempty" option — is accepted, pinning that tag
// -option stripping (R13/R14) does not misclassify a modeled field as
// unknown. Originally this scenario used disposition_reason; 146.011-T's
// Copilot-review remediation (PR #373) moved the disposition_* fields to
// checkpointV1ReservedKeys (see checkpoint_strict.go), so resume_hint is a
// non-reserved ",omitempty" field standing in for the same tag-stripping
// guarantee.
func TestCreateCheckpoint_ResumeHintAccepted(t *testing.T) {
	dir := t.TempDir()
	stateDump := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","status":"active",` +
		`"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z",` +
		`"resume_hint":"synthetic resume hint text"}`

	result, err := events.CreateCheckpoint(context.Background(), dir, stateDump)
	require.NoError(t, err)
	assert.FileExists(t, result.Path)
}
