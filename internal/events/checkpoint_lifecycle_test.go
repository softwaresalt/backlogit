package events

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	backlogiterrors "github.com/softwaresalt/backlogit/internal/errors"
)

func writeTestCheckpointNamed(t *testing.T, dir, name string, cp *CheckpointV1) {
	t.Helper()
	data, err := json.Marshal(cp)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), data, 0o644))
}

func TestListCheckpoints_EmptyDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".backlogit", "checkpoints")
	require.NoError(t, os.MkdirAll(dir, 0o755))

	summaries, err := ListCheckpoints(context.Background(), dir, CheckpointFilter{})
	require.NoError(t, err)
	assert.Empty(t, summaries)
}

func TestListCheckpoints_ReturnsAll(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".backlogit", "checkpoints")

	cp1 := validCheckpointV1()
	cp1.SessionID = "session-1"
	writeTestCheckpointNamed(t, dir, "checkpoint-20260423-100000.json", cp1)

	cp2 := validCheckpointV1()
	cp2.SessionID = "session-2"
	cp2.Agent = "stage"
	writeTestCheckpointNamed(t, dir, "checkpoint-20260423-110000.json", cp2)

	summaries, err := ListCheckpoints(context.Background(), dir, CheckpointFilter{})
	require.NoError(t, err)
	assert.Len(t, summaries, 2)
}

func TestListCheckpoints_FilterByAgent(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".backlogit", "checkpoints")

	cp1 := validCheckpointV1()
	cp1.Agent = "ship"
	writeTestCheckpointNamed(t, dir, "checkpoint-20260423-100000.json", cp1)

	cp2 := validCheckpointV1()
	cp2.Agent = "stage"
	writeTestCheckpointNamed(t, dir, "checkpoint-20260423-110000.json", cp2)

	summaries, err := ListCheckpoints(context.Background(), dir, CheckpointFilter{Agent: "ship"})
	require.NoError(t, err)
	assert.Len(t, summaries, 1)
	assert.Equal(t, "ship", summaries[0].Agent)
}

func TestListCheckpoints_FilterByStatus(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".backlogit", "checkpoints")

	cp1 := validCheckpointV1()
	cp1.Status = "active"
	writeTestCheckpointNamed(t, dir, "checkpoint-20260423-100000.json", cp1)

	cp2 := validCheckpointV1()
	cp2.Status = "resolved"
	writeTestCheckpointNamed(t, dir, "checkpoint-20260423-110000.json", cp2)

	summaries, err := ListCheckpoints(context.Background(), dir, CheckpointFilter{Status: "active"})
	require.NoError(t, err)
	assert.Len(t, summaries, 1)
	assert.Equal(t, "active", summaries[0].Status)
}

func TestListCheckpoints_FilterByShipmentID(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".backlogit", "checkpoints")

	cp1 := validCheckpointV1()
	cp1.Context.ShipmentID = "044-S"
	writeTestCheckpointNamed(t, dir, "checkpoint-20260423-100000.json", cp1)

	cp2 := validCheckpointV1()
	cp2.Context.ShipmentID = "045-S"
	writeTestCheckpointNamed(t, dir, "checkpoint-20260423-110000.json", cp2)

	summaries, err := ListCheckpoints(context.Background(), dir, CheckpointFilter{ShipmentID: "044-S"})
	require.NoError(t, err)
	assert.Len(t, summaries, 1)
	assert.Equal(t, "044-S", summaries[0].ShipmentID)
}

func TestListCheckpoints_FilterByMaxAge(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".backlogit", "checkpoints")

	old := validCheckpointV1()
	old.CreatedAt = time.Now().UTC().Add(-48 * time.Hour)
	writeTestCheckpointNamed(t, dir, "checkpoint-20260421-100000.json", old)

	recent := validCheckpointV1()
	recent.CreatedAt = time.Now().UTC().Add(-1 * time.Hour)
	writeTestCheckpointNamed(t, dir, "checkpoint-20260423-100000.json", recent)

	summaries, err := ListCheckpoints(context.Background(), dir, CheckpointFilter{MaxAge: 24 * time.Hour})
	require.NoError(t, err)
	assert.Len(t, summaries, 1)
}

// TestListCheckpoints_FlagsBadFilesReadOnly asserts the 136-F/U9 read-only
// contract: ListCheckpoints never moves, deletes, or rewrites a malformed
// checkpoint file. It surfaces NeedsQuarantine and a RemediationCommand so a
// caller can explicitly invoke QuarantineCheckpoint instead.
func TestListCheckpoints_FlagsBadFilesReadOnly(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".backlogit", "checkpoints")
	require.NoError(t, os.MkdirAll(dir, 0o755))

	// Write garbage file.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "checkpoint-bad.json"), []byte("not-json{"), 0o644))

	summaries, err := ListCheckpoints(context.Background(), dir, CheckpointFilter{})
	require.NoError(t, err)
	assert.Len(t, summaries, 1)
	assert.NotEmpty(t, summaries[0].ValidationErr)
	assert.True(t, summaries[0].NeedsQuarantine)
	assert.Contains(t, summaries[0].RemediationCommand, "checkpoint quarantine 'checkpoint-bad.json'")
	assert.False(t, summaries[0].Quarantined, "list must never physically quarantine")

	// The file must remain in place: ListCheckpoints is read-only.
	quarantine := filepath.Join(root, ".backlogit", "quarantine", "checkpoints", "checkpoint-bad.json")
	assert.NoFileExists(t, quarantine)
	assert.FileExists(t, filepath.Join(dir, "checkpoint-bad.json"))
}

func TestListCheckpoints_SortedByCreatedAtDesc(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".backlogit", "checkpoints")

	cp1 := validCheckpointV1()
	cp1.CreatedAt = time.Date(2026, 4, 23, 10, 0, 0, 0, time.UTC)
	writeTestCheckpointNamed(t, dir, "checkpoint-20260423-100000.json", cp1)

	cp2 := validCheckpointV1()
	cp2.CreatedAt = time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
	writeTestCheckpointNamed(t, dir, "checkpoint-20260423-120000.json", cp2)

	summaries, err := ListCheckpoints(context.Background(), dir, CheckpointFilter{})
	require.NoError(t, err)
	require.Len(t, summaries, 2)
	assert.True(t, summaries[0].CreatedAt.After(summaries[1].CreatedAt))
}

func TestGetCheckpoint_Valid(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".backlogit", "checkpoints")
	cp := validCheckpointV1()
	writeTestCheckpointNamed(t, dir, "checkpoint-20260423-100000.json", cp)

	result, err := GetCheckpoint(context.Background(), dir, "checkpoint-20260423-100000.json")
	require.NoError(t, err)
	assert.Equal(t, "ship", result.Agent)
	assert.Equal(t, "test-session-001", result.SessionID)
}

func TestGetCheckpoint_NotFound(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".backlogit", "checkpoints")
	require.NoError(t, os.MkdirAll(dir, 0o755))

	_, err := GetCheckpoint(context.Background(), dir, "checkpoint-missing.json")
	require.Error(t, err)
	assert.ErrorIs(t, err, backlogiterrors.ErrCheckpointNotFound)
}

func TestGetCheckpoint_PathTraversal(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".backlogit", "checkpoints")
	require.NoError(t, os.MkdirAll(dir, 0o755))

	_, err := GetCheckpoint(context.Background(), dir, "../../../etc/passwd")
	require.Error(t, err)
	assert.ErrorIs(t, err, backlogiterrors.ErrCheckpointInvalid)
}

func TestGetCheckpoint_EmptyFilename(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".backlogit", "checkpoints")
	_, err := GetCheckpoint(context.Background(), dir, "")
	require.Error(t, err)
	assert.ErrorIs(t, err, backlogiterrors.ErrCheckpointInvalid)
}

func TestResolveCheckpoint_Active(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".backlogit", "checkpoints")
	cp := validCheckpointV1()
	cp.Status = "active"
	writeTestCheckpointNamed(t, dir, "checkpoint-20260423-100000.json", cp)

	err := ResolveCheckpoint(context.Background(), dir, "checkpoint-20260423-100000.json")
	require.NoError(t, err)

	// Verify it's now resolved.
	resolved, err := GetCheckpoint(context.Background(), dir, "checkpoint-20260423-100000.json")
	require.NoError(t, err)
	assert.Equal(t, "resolved", resolved.Status)
}

func TestResolveCheckpoint_Idempotent(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".backlogit", "checkpoints")
	cp := validCheckpointV1()
	cp.Status = "resolved"
	writeTestCheckpointNamed(t, dir, "checkpoint-20260423-100000.json", cp)

	err := ResolveCheckpoint(context.Background(), dir, "checkpoint-20260423-100000.json")
	assert.NoError(t, err)
}

func TestResolveCheckpoint_NotFound(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".backlogit", "checkpoints")
	require.NoError(t, os.MkdirAll(dir, 0o755))

	err := ResolveCheckpoint(context.Background(), dir, "checkpoint-missing.json")
	require.Error(t, err)
	assert.ErrorIs(t, err, backlogiterrors.ErrCheckpointNotFound)
}

// TestResolveCheckpoint_RefusesAbandoned is a 136-F regression: ResolveCheckpoint
// must not silently undo an administrative abandon disposition by flipping
// Status back to "resolved". Abandon is a terminal, non-resumable state.
func TestResolveCheckpoint_RefusesAbandoned(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".backlogit", "checkpoints")
	cp := validCheckpointV1()
	cp.Status = "abandoned"
	cp.Disposition = DispositionAbandoned
	cp.DispositionReason = "superseded"
	cp.DispositionOperator = "tester@example.com"
	dispositionAt := time.Now().UTC()
	cp.DispositionAt = &dispositionAt
	writeTestCheckpointNamed(t, dir, "checkpoint-20260423-100000.json", cp)

	err := ResolveCheckpoint(context.Background(), dir, "checkpoint-20260423-100000.json")
	require.Error(t, err)
	assert.ErrorIs(t, err, backlogiterrors.ErrCheckpointCannotResolveAbandoned)

	// The checkpoint must remain untouched — still abandoned, not resolved.
	unchanged, getErr := GetCheckpoint(context.Background(), dir, "checkpoint-20260423-100000.json")
	require.NoError(t, getErr)
	assert.Equal(t, "abandoned", unchanged.Status)
	assert.Equal(t, DispositionAbandoned, unchanged.Disposition)
}

func TestResolveCheckpoint_PathTraversal(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".backlogit", "checkpoints")
	err := ResolveCheckpoint(context.Background(), dir, "../../secret.json")
	require.Error(t, err)
	assert.ErrorIs(t, err, backlogiterrors.ErrCheckpointInvalid)
}

func TestCleanupCheckpoints_ArchivesResolved(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".backlogit", "checkpoints")

	cp := validCheckpointV1()
	cp.Status = "resolved"
	writeTestCheckpointNamed(t, dir, "checkpoint-20260423-100000.json", cp)

	result, err := CleanupCheckpoints(context.Background(), dir, 7)
	require.NoError(t, err)
	assert.Equal(t, 1, result.ArchivedCount)
	assert.Contains(t, result.ArchivedFiles, "checkpoint-20260423-100000.json")

	// Verify archived.
	archive := filepath.Join(root, ".backlogit", "archive", "checkpoints", "checkpoint-20260423-100000.json")
	assert.FileExists(t, archive)
	assert.NoFileExists(t, filepath.Join(dir, "checkpoint-20260423-100000.json"))
}

func TestCleanupCheckpoints_ArchivesStale(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".backlogit", "checkpoints")

	cp := validCheckpointV1()
	cp.Status = "active"
	cp.CreatedAt = time.Now().UTC().Add(-10 * 24 * time.Hour)
	writeTestCheckpointNamed(t, dir, "checkpoint-20260413-100000.json", cp)

	result, err := CleanupCheckpoints(context.Background(), dir, 7)
	require.NoError(t, err)
	assert.Equal(t, 1, result.ArchivedCount)
}

func TestCleanupCheckpoints_SkipsActive(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".backlogit", "checkpoints")

	cp := validCheckpointV1()
	cp.Status = "active"
	cp.CreatedAt = time.Now().UTC()
	writeTestCheckpointNamed(t, dir, "checkpoint-20260423-100000.json", cp)

	result, err := CleanupCheckpoints(context.Background(), dir, 7)
	require.NoError(t, err)
	assert.Equal(t, 0, result.ArchivedCount)
	assert.Equal(t, 1, result.SkippedCount)
}

func TestCleanupCheckpoints_InvalidRetention(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".backlogit", "checkpoints")
	_, err := CleanupCheckpoints(context.Background(), dir, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "retentionDays must be > 0")
}

func TestCleanupCheckpoints_EmptyDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".backlogit", "checkpoints")
	require.NoError(t, os.MkdirAll(dir, 0o755))

	result, err := CleanupCheckpoints(context.Background(), dir, 7)
	require.NoError(t, err)
	assert.Equal(t, 0, result.ArchivedCount)
}

func TestCleanupCheckpoints_NegativeRetention(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".backlogit", "checkpoints")
	_, err := CleanupCheckpoints(context.Background(), dir, -1)
	require.Error(t, err)
}

func TestResolveCheckpoint_ConcurrentResolve(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".backlogit", "checkpoints")

	// Create multiple active checkpoints.
	for i := 0; i < 5; i++ {
		cp := validCheckpointV1()
		cp.Status = "active"
		cp.SessionID = fmt.Sprintf("session-%d", i)
		name := fmt.Sprintf("checkpoint-20260423-10000%d.json", i)
		writeTestCheckpointNamed(t, dir, name, cp)
	}

	// Resolve all concurrently.
	var wg sync.WaitGroup
	errs := make([]error, 5)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			name := fmt.Sprintf("checkpoint-20260423-10000%d.json", idx)
			errs[idx] = ResolveCheckpoint(context.Background(), dir, name)
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		assert.NoError(t, err, "resolve goroutine %d should not fail", i)
	}

	// Verify all resolved.
	summaries, err := ListCheckpoints(context.Background(), dir, CheckpointFilter{Status: "resolved"})
	require.NoError(t, err)
	assert.Len(t, summaries, 5)
}

func TestListCheckpoints_FilterByFeatureID(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".backlogit", "checkpoints")

	cp1 := validCheckpointV1()
	cp1.Context.FeatureID = "045-F"
	writeTestCheckpointNamed(t, dir, "checkpoint-20260423-100000.json", cp1)

	cp2 := validCheckpointV1()
	cp2.Context.FeatureID = "046-F"
	writeTestCheckpointNamed(t, dir, "checkpoint-20260423-110000.json", cp2)

	summaries, err := ListCheckpoints(context.Background(), dir, CheckpointFilter{FeatureID: "045-F"})
	require.NoError(t, err)
	assert.Len(t, summaries, 1)
	assert.Equal(t, "045-F", summaries[0].FeatureID)
}

func TestCleanupCheckpoints_MixedEligibility(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".backlogit", "checkpoints")

	// Resolved: should archive.
	resolved := validCheckpointV1()
	resolved.Status = "resolved"
	writeTestCheckpointNamed(t, dir, "checkpoint-20260423-100000.json", resolved)

	// Active but stale: should archive.
	stale := validCheckpointV1()
	stale.Status = "active"
	stale.CreatedAt = time.Now().UTC().Add(-30 * 24 * time.Hour)
	writeTestCheckpointNamed(t, dir, "checkpoint-20260323-100000.json", stale)

	// Active and recent: should skip.
	fresh := validCheckpointV1()
	fresh.Status = "active"
	fresh.CreatedAt = time.Now().UTC()
	writeTestCheckpointNamed(t, dir, "checkpoint-20260423-120000.json", fresh)

	result, err := CleanupCheckpoints(context.Background(), dir, 7)
	require.NoError(t, err)
	assert.Equal(t, 2, result.ArchivedCount)
	assert.Equal(t, 1, result.SkippedCount)
}





// TestListCheckpoints_RemediationCommandIsShellSafe is the follow-up
// regression to TestListCheckpoints_FlagsBadFilesReadOnly: the advertised
// remediation command must be safe to run verbatim in a POSIX shell even
// when the checkpoint filename contains shell metacharacters, and the
// "<reason>" placeholder must not be interpreted as input redirection.
func TestListCheckpoints_RemediationCommandIsShellSafe(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".backlogit", "checkpoints")
	require.NoError(t, os.MkdirAll(dir, 0o755))

	// A filename containing a space and a single quote and a redirection
	// metacharacter, still matching the checkpoint-*.json glob.
	weirdName := "checkpoint-weird 'name' & rm -rf.json"
	require.NoError(t, os.WriteFile(filepath.Join(dir, weirdName), []byte("not-json{"), 0o644))

	summaries, err := ListCheckpoints(context.Background(), dir, CheckpointFilter{})
	require.NoError(t, err)
	require.Len(t, summaries, 1)

	cmd := summaries[0].RemediationCommand
	// The filename must appear fully single-quoted (embedded quotes escaped),
	// never bare — a bare filename with spaces/metacharacters would not be
	// safely copy-pastable into a shell.
	assert.Contains(t, cmd, "'checkpoint-weird '\\''name'\\'' & rm -rf.json'")
	// The reason placeholder must be quoted so "<" is never live redirection.
	assert.Contains(t, cmd, "--reason '<reason>'")
}

// TestResolveCheckpoint_NoHTMLEscape verifies that ResolveCheckpoint writes
// checkpoint JSON without HTML-escaping special characters. Previously
// json.Marshal was used, which escaped >, <, and & as \u003e/\u003c/\u0026.
func TestResolveCheckpoint_NoHTMLEscape(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".backlogit", "checkpoints")

	cp := validCheckpointV1()
	// Embed characters that json.Marshal HTML-escapes by default.
	cp.ResumeHint = "URL: https://example.com/a?x=1&y=2, expr: a > b && b < c"
	cp.Context.Branch = "feat/compare-a>b"
	cp.Status = "active"

	name := "checkpoint-20260423-999999.json"
	writeTestCheckpointNamed(t, dir, name, cp)

	err := ResolveCheckpoint(context.Background(), dir, name)
	require.NoError(t, err)

	data, readErr := os.ReadFile(filepath.Join(dir, name))
	require.NoError(t, readErr)
	s := string(data)

	assert.Contains(t, s, "&", "ampersand must not be Unicode-escaped in resolved checkpoint")
	assert.Contains(t, s, ">", "greater-than must not be Unicode-escaped in resolved checkpoint")
	assert.Contains(t, s, "<", "less-than must not be Unicode-escaped in resolved checkpoint")
	assert.NotContains(t, s, `\u0026`, "\\u0026 must not appear in resolved checkpoint")
	assert.NotContains(t, s, `\u003e`, "\\u003e must not appear in resolved checkpoint")
	assert.NotContains(t, s, `\u003c`, "\\u003c must not appear in resolved checkpoint")
}

// TestU6_ValidButNonConformingListsNeedsQuarantineWithIntent asserts a
// valid-but-non-conforming file lists with NeedsQuarantine: true and a
// RemediationIntent naming verb "quarantine", the bare target filename,
// RequiresApproval: true, and ApprovalClass "A4c" — and the summary carries
// no shell text (147-F / U6).
func TestU6_ValidButNonConformingListsNeedsQuarantineWithIntent(t *testing.T) {
	dir := t.TempDir()
	name := "checkpoint-u6-nonconforming.json"
	body := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","status":"active",` +
		`"created_at":"2026-08-24T00:00:00Z","updated_at":"2026-08-24T00:00:00Z","extra_key":"x"}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644))
	before := sha256.Sum256([]byte(body))

	summaries, err := ListCheckpoints(context.Background(), dir, CheckpointFilter{})
	require.NoError(t, err)
	require.Len(t, summaries, 1)

	assert.True(t, summaries[0].NeedsQuarantine)
	require.NotNil(t, summaries[0].RemediationIntent)
	assert.Equal(t, "quarantine", summaries[0].RemediationIntent.Verb)
	assert.Equal(t, name, summaries[0].RemediationIntent.TargetFilename)
	assert.True(t, summaries[0].RemediationIntent.RequiresApproval)
	assert.Equal(t, "A4c", summaries[0].RemediationIntent.ApprovalClass)
	assert.Equal(t, "non_conforming", summaries[0].RemediationIntent.Reason)

	after, readErr := os.ReadFile(filepath.Join(dir, name))
	require.NoError(t, readErr)
	assert.Equal(t, before, sha256.Sum256(after), "ListCheckpoints must be read-only")
}

// TestU6_FailsBothValidationAndConformanceReportsBothReasons asserts a file
// failing both validation and conformance reports both reasons in
// ValidationErr.
func TestU6_FailsBothValidationAndConformanceReportsBothReasons(t *testing.T) {
	dir := t.TempDir()
	name := "checkpoint-u6-both.json"
	// Schema-invalid (missing required fields) AND carries an unmodeled key.
	body := `{"status":"active","extra_key":"x"}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644))
	before := sha256.Sum256([]byte(body))

	summaries, err := ListCheckpoints(context.Background(), dir, CheckpointFilter{})
	require.NoError(t, err)
	require.Len(t, summaries, 1)

	assert.True(t, summaries[0].NeedsQuarantine)
	assert.NotEmpty(t, summaries[0].ValidationErr)
	// Both the schema-validation reason and the conformance reason must be
	// present; conformance must not overwrite the validation reason. The
	// schema-validator error alone never names the offending key, so this
	// assertion only holds once the conformance branch has also run and
	// appended its own reason.
	assert.Contains(t, summaries[0].ValidationErr, "extra_key")

	after, readErr := os.ReadFile(filepath.Join(dir, name))
	require.NoError(t, readErr)
	assert.Equal(t, before, sha256.Sum256(after), "ListCheckpoints must be read-only")
}

// TestU6Guard_VerdictComputedBeforeFilterBlock asserts the conformance
// verdict is computed before the filter block, using a filter that matches
// the non-conforming document.
func TestU6Guard_VerdictComputedBeforeFilterBlock(t *testing.T) {
	dir := t.TempDir()
	name := "checkpoint-u6-filtered.json"
	body := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","status":"active",` +
		`"created_at":"2026-08-24T00:00:00Z","updated_at":"2026-08-24T00:00:00Z","extra_key":"x"}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644))

	summaries, err := ListCheckpoints(context.Background(), dir, CheckpointFilter{Agent: "ship"})
	require.NoError(t, err)
	require.Len(t, summaries, 1)
	assert.True(t, summaries[0].NeedsQuarantine)
	require.NotNil(t, summaries[0].RemediationIntent)
}

// TestU6b_ValidButNonConformingResultProjectsConformanceFields asserts
// GetCheckpointResult on a valid-but-non-conforming document returns
// Valid:true, Conforming:false, NeedsQuarantine:true, a RemediationIntent
// naming verb "quarantine" and approval class "A4c", and
// NonConformingFields.Paths naming the offenders in raw, unquoted form
// (147-F / U6b).
func TestU6b_ValidButNonConformingResultProjectsConformanceFields(t *testing.T) {
	dir := t.TempDir()
	name := "checkpoint-u6b-nonconforming.json"
	body := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","status":"active",` +
		`"created_at":"2026-08-24T00:00:00Z","updated_at":"2026-08-24T00:00:00Z","extra_key":"x"}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644))

	result, err := GetCheckpointResult(context.Background(), dir, name)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Valid)
	assert.False(t, result.Conforming)
	assert.True(t, result.NeedsQuarantine)
	require.NotNil(t, result.RemediationIntent)
	assert.Equal(t, "quarantine", result.RemediationIntent.Verb)
	assert.Equal(t, "A4c", result.RemediationIntent.ApprovalClass)
	assert.Contains(t, result.NonConformingFields.Paths, "extra_key")
	for _, p := range result.NonConformingFields.Paths {
		assert.NotContains(t, p, `"`, "NonConformingFields.Paths must carry raw, unquoted offender paths")
	}
}

// TestU6b_ConformingResultProjectsConformingTrue asserts a conforming file
// returns Conforming:true, a nil RemediationIntent, and an empty
// NonConformingFields.Paths with Truncated:false.
func TestU6b_ConformingResultProjectsConformingTrue(t *testing.T) {
	dir := t.TempDir()
	name := "checkpoint-u6b-conforming.json"
	body := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","status":"active",` +
		`"created_at":"2026-08-24T00:00:00Z","updated_at":"2026-08-24T00:00:00Z"}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644))

	result, err := GetCheckpointResult(context.Background(), dir, name)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Conforming)
	assert.Nil(t, result.RemediationIntent)
	assert.Empty(t, result.NonConformingFields.Paths)
	assert.False(t, result.NonConformingFields.Truncated)
}

// TestU6bGuard_ByteUnchangedAfterGet pins GetCheckpoint's read-only contract.
func TestU6bGuard_ByteUnchangedAfterGet(t *testing.T) {
	dir := t.TempDir()
	name := "checkpoint-u6b-readonly.json"
	body := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","status":"active",` +
		`"created_at":"2026-08-24T00:00:00Z","updated_at":"2026-08-24T00:00:00Z"}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644))
	before := sha256.Sum256([]byte(body))

	_, err := GetCheckpointResult(context.Background(), dir, name)
	require.NoError(t, err)

	after, readErr := os.ReadFile(filepath.Join(dir, name))
	require.NoError(t, readErr)
	assert.Equal(t, before, sha256.Sum256(after))
}

// TestU6d_NonConformingResolvedStatusSurvivesActiveFilter asserts a
// valid-but-non-conforming status:"resolved" file is still returned when
// filter.Status == "active", carrying NeedsQuarantine:true (147-F / U6d).
func TestU6d_NonConformingResolvedStatusSurvivesActiveFilter(t *testing.T) {
	dir := t.TempDir()
	name := "checkpoint-u6d-resolved-nonconforming.json"
	body := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","status":"resolved",` +
		`"created_at":"2026-08-24T00:00:00Z","updated_at":"2026-08-24T00:00:00Z","extra_key":"x"}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644))

	summaries, err := ListCheckpoints(context.Background(), dir, CheckpointFilter{Status: "active"})
	require.NoError(t, err)
	require.Len(t, summaries, 1, "a quarantine candidate must bypass the filter block entirely")
	assert.True(t, summaries[0].NeedsQuarantine)
}

// TestU6d_NonConformingSurvivesAgentFilterMismatch asserts a non-conforming
// file is still returned when filter.Agent names a different agent,
// matching the parse-failure precedent.
func TestU6d_NonConformingSurvivesAgentFilterMismatch(t *testing.T) {
	dir := t.TempDir()
	name := "checkpoint-u6d-agent-mismatch.json"
	body := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","status":"active",` +
		`"created_at":"2026-08-24T00:00:00Z","updated_at":"2026-08-24T00:00:00Z","extra_key":"x"}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644))

	summaries, err := ListCheckpoints(context.Background(), dir, CheckpointFilter{Agent: "stage"})
	require.NoError(t, err)
	require.Len(t, summaries, 1, "a quarantine candidate must bypass an agent filter mismatch")
	assert.True(t, summaries[0].NeedsQuarantine)
}

// TestU6dGuard_ConformingResolvedStatusStillDroppedByActiveFilter proves the
// exemption is scoped to quarantine candidates and is not a blanket filter
// bypass: a conforming status:"resolved" file is still dropped by an
// "active" status filter.
func TestU6dGuard_ConformingResolvedStatusStillDroppedByActiveFilter(t *testing.T) {
	dir := t.TempDir()
	name := "checkpoint-u6d-resolved-conforming.json"
	body := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","status":"resolved",` +
		`"created_at":"2026-08-24T00:00:00Z","updated_at":"2026-08-24T00:00:00Z"}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644))

	summaries, err := ListCheckpoints(context.Background(), dir, CheckpointFilter{Status: "active"})
	require.NoError(t, err)
	assert.Empty(t, summaries, "a conforming, non-matching document must still be dropped by the filter")
}

// TestU6e_UnparseableListsRemediationIntentUnparseable asserts an
// unparseable file lists with NeedsQuarantine:true and a RemediationIntent
// whose Reason is "unparseable" (147-F / U6e).
func TestU6e_UnparseableListsRemediationIntentUnparseable(t *testing.T) {
	dir := t.TempDir()
	name := "checkpoint-u6e-unparseable.json"
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("not-json{"), 0o644))

	summaries, err := ListCheckpoints(context.Background(), dir, CheckpointFilter{})
	require.NoError(t, err)
	require.Len(t, summaries, 1)
	assert.True(t, summaries[0].NeedsQuarantine)
	require.NotNil(t, summaries[0].RemediationIntent)
	assert.Equal(t, "unparseable", summaries[0].RemediationIntent.Reason)
	assert.Equal(t, name, summaries[0].RemediationIntent.TargetFilename)
	assert.True(t, summaries[0].RemediationIntent.RequiresApproval)
}

// TestU6e_SchemaInvalidConformingListsRemediationIntentSchemaInvalid asserts
// a schema-invalid but conforming file lists with a RemediationIntent whose
// Reason is "schema_invalid".
func TestU6e_SchemaInvalidConformingListsRemediationIntentSchemaInvalid(t *testing.T) {
	dir := t.TempDir()
	name := "checkpoint-u6e-schema-invalid.json"
	// Parses fine, has only modeled keys (conforming), but fails
	// ValidateCheckpoint (missing required fields).
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(`{"status":"active"}`), 0o644))

	summaries, err := ListCheckpoints(context.Background(), dir, CheckpointFilter{})
	require.NoError(t, err)
	require.Len(t, summaries, 1)
	require.NotNil(t, summaries[0].RemediationIntent)
	assert.Equal(t, "schema_invalid", summaries[0].RemediationIntent.Reason)
}

