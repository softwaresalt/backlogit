package events_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	backlogiterrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
)

// unknownFieldsFromErr recovers the sorted Fields slice from a
// *backlogiterrors.CheckpointUnknownFieldError via errors.As, failing the test
// if the error is not of that type or does not match the sentinel through
// Unwrap.
func unknownFieldsFromErr(t *testing.T, err error) []string {
	t.Helper()
	require.Error(t, err)
	require.True(t, errors.Is(err, backlogiterrors.ErrCheckpointUnknownField),
		"error must satisfy errors.Is(err, ErrCheckpointUnknownField)")
	var typed *backlogiterrors.CheckpointUnknownFieldError
	require.True(t, errors.As(err, &typed), "error must be recoverable via errors.As as *CheckpointUnknownFieldError")
	return typed.Fields
}

// assertNoCheckpointWritten fails the test if checkpointDir contains any
// checkpoint-*.json file, proving a rejected create wrote no file.
func assertNoCheckpointWritten(t *testing.T, checkpointDir string) {
	t.Helper()
	entries, err := os.ReadDir(checkpointDir)
	if os.IsNotExist(err) {
		return
	}
	require.NoError(t, err)
	assert.Empty(t, entries, "a rejected create must write no file")
}

// TestCreateCheckpoint_RejectsSingleUnknownTopLevelKey is scenario 1 of
// 146.007-T (U3a): a V1 dump with one unknown top-level key fails create with
// an error satisfying errors.Is(err, ErrCheckpointUnknownField) and naming
// that key, and writes no file.
func TestCreateCheckpoint_RejectsSingleUnknownTopLevelKey(t *testing.T) {
	dir := t.TempDir()
	stateDump := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","unexpected_key":"x"}`

	_, err := events.CreateCheckpoint(context.Background(), dir, stateDump)
	fields := unknownFieldsFromErr(t, err)
	assert.Equal(t, []string{"unexpected_key"}, fields)
	assertNoCheckpointWritten(t, dir)
}

// TestCreateCheckpoint_RejectsTwoUnknownTopLevelKeys is scenario 2 of
// 146.007-T (U3a): a V1 dump with two unknown top-level keys names both,
// sorted, in a single error.
func TestCreateCheckpoint_RejectsTwoUnknownTopLevelKeys(t *testing.T) {
	dir := t.TempDir()
	stateDump := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","zeta_key":"x","alpha_key":"y"}`

	_, err := events.CreateCheckpoint(context.Background(), dir, stateDump)
	fields := unknownFieldsFromErr(t, err)
	assert.Equal(t, []string{"alpha_key", "zeta_key"}, fields)
	assertNoCheckpointWritten(t, dir)
}

// TestCreateCheckpoint_RejectsUnknownNestedProgressKey is scenario 3 of
// 146.007-T (U3a): an unknown key nested inside progress is rejected and the
// error names the nesting path, pinning the decided blast radius of the
// closed namespace (the CheckpointV1 top level AND the nested progress
// object).
func TestCreateCheckpoint_RejectsUnknownNestedProgressKey(t *testing.T) {
	dir := t.TempDir()
	stateDump := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","progress":{"unexpected_nested":"x"}}`

	_, err := events.CreateCheckpoint(context.Background(), dir, stateDump)
	fields := unknownFieldsFromErr(t, err)
	assert.Equal(t, []string{"progress.unexpected_nested"}, fields)
	assertNoCheckpointWritten(t, dir)
}

// TestCreateCheckpoint_MixedCaseDuplicateProgressAlias_NIndependentPairs is
// scenario 4 of 146.007-T (U3a): an N-independent-pair mixed-case duplicate
// table with N = 8 structurally independent fixtures. Each fixture carries
// both a "progress" and a "Progress" alias at the top level and a different
// unknown nested key, and the pairs alternate which alias holds the unknown
// key so no pair's outcome depends on another's. Every fixture must be
// rejected and must name exactly its own expected nested path. A ninth
// fixture places a different unknown key under each alias and asserts both
// paths appear, sorted and de-duplicated, in one error.
//
// N = 8 (not one fixture repeated) because Go's map iteration order is
// unspecified: a single fixture re-run gives no bounded detection signal
// against a map-routing implementation. See
// docs/compound/2026-08-01-n-independent-pair-test-design-for-go-map-iteration-nondeterminism.md.
// Before 146.011-T (U4) lands, the closed-namespace check does not exist at
// all, so every one of the 9 fixtures fails deterministically (100%), which
// trivially satisfies "most or all pairs failing" for the red observation.
func TestCreateCheckpoint_MixedCaseDuplicateProgressAlias_NIndependentPairs(t *testing.T) {
	const n = 8
	failCount := 0

	for i := 1; i <= n; i++ {
		i := i
		t.Run(fmt.Sprintf("pair_%02d", i), func(t *testing.T) {
			dir := t.TempDir()
			unknownKey := fmt.Sprintf("unexpected_field_%02d", i)
			var progressVal, progressCapVal string
			cleanShape := `{"tasks_completed":["146.001-T"]}`
			dirtyShape := fmt.Sprintf(`{%q:"x"}`, unknownKey)
			var wantPath string
			if i%2 == 1 {
				// odd pairs: "progress" carries the unknown key
				progressVal, progressCapVal = dirtyShape, cleanShape
				wantPath = "progress." + unknownKey
			} else {
				// even pairs: "Progress" carries the unknown key
				progressVal, progressCapVal = cleanShape, dirtyShape
				wantPath = "progress." + unknownKey
			}
			stateDump := fmt.Sprintf(
				`{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","progress":%s,"Progress":%s}`,
				progressVal, progressCapVal,
			)

			_, err := events.CreateCheckpoint(context.Background(), dir, stateDump)
			if err == nil {
				t.Errorf("pair %02d: expected create to be rejected, got success", i)
				return
			}
			var typed *backlogiterrors.CheckpointUnknownFieldError
			if !errors.As(err, &typed) {
				failCount++
				t.Errorf("pair %02d: error is not a *CheckpointUnknownFieldError: %v", i, err)
				return
			}
			if len(typed.Fields) != 1 || typed.Fields[0] != wantPath {
				failCount++
				t.Errorf("pair %02d: want Fields=[%s], got %v", i, wantPath, typed.Fields)
				return
			}
			assertNoCheckpointWritten(t, dir)
		})
	}

	t.Logf("N-independent-pair red observation: %d of %d pairs failed", failCount, n)

	// Ninth fixture: a different unknown key under each alias; both paths
	// must appear, sorted and de-duplicated, in one error.
	t.Run("ninth_fixture_both_aliases", func(t *testing.T) {
		dir := t.TempDir()
		stateDump := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","progress":{"zzz_unknown":"x"},"Progress":{"aaa_unknown":"y"}}`

		_, err := events.CreateCheckpoint(context.Background(), dir, stateDump)
		fields := unknownFieldsFromErr(t, err)
		want := []string{"progress.aaa_unknown", "progress.zzz_unknown"}
		sort.Strings(want)
		assert.Equal(t, want, fields)
		assertNoCheckpointWritten(t, dir)
	})
}

// TestCreateCheckpoint_ExactDuplicateProgressKey_DirtyFirst and
// TestCreateCheckpoint_ExactDuplicateProgressKey_CleanFirst close the gap
// Copilot review found on PR #373: json.Unmarshal into a
// map[string]json.RawMessage silently collapses two EXACT-case-duplicate
// top-level keys (not just mixed-case aliases like "progress"/"Progress") to
// a single last-value-wins entry before any validation code runs. A naive
// map-based decode would therefore let an unknown key hidden in a shadowed
// "progress" object escape detection whenever a later identically-cased
// "progress" member happened to be clean. checkClosedSchemaNamespace now
// walks the top-level object as an ordered token stream
// (decodeTopLevelEntries) precisely so every occurrence — dirty first or
// dirty last — is inspected, regardless of which one ParseCheckpoint's own
// struct decode ultimately assigns to CheckpointV1.Progress.
func TestCreateCheckpoint_ExactDuplicateProgressKey_DirtyFirst(t *testing.T) {
	dir := t.TempDir()
	stateDump := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build",` +
		`"progress":{"unexpected_dup":"x"},"progress":{"tasks_completed":["146.001-T"]}}`

	_, err := events.CreateCheckpoint(context.Background(), dir, stateDump)
	fields := unknownFieldsFromErr(t, err)
	assert.Equal(t, []string{"progress.unexpected_dup"}, fields)
	assertNoCheckpointWritten(t, dir)
}

func TestCreateCheckpoint_ExactDuplicateProgressKey_CleanFirst(t *testing.T) {
	dir := t.TempDir()
	stateDump := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build",` +
		`"progress":{"tasks_completed":["146.001-T"]},"progress":{"unexpected_dup":"x"}}`

	_, err := events.CreateCheckpoint(context.Background(), dir, stateDump)
	fields := unknownFieldsFromErr(t, err)
	assert.Equal(t, []string{"progress.unexpected_dup"}, fields)
	assertNoCheckpointWritten(t, dir)
}

// TestCreateCheckpoint_RejectsReservedDispositionFields closes the
// audit-bypass gap Copilot review found on PR #373: the reflection-derived
// create allowlist originally admitted the four administrative disposition
// fields (disposition, disposition_reason, disposition_operator,
// disposition_at), which are documented
// (docs/design-docs/checkpoint-administrative-disposition.md) as populated
// ONLY by AbandonCheckpoint. Admitting them at create let a caller forge
// disposition:"abandoned" directly, so a later legitimate AbandonCheckpoint
// call on that same file would silently no-op via its
// idempotent-already-abandoned short-circuit and produce no audit append.
// Each reserved key is now excluded from checkpointV1TopLevelKeys and
// rejected as an unknown field at create.
func TestCreateCheckpoint_RejectsReservedDispositionFields(t *testing.T) {
	tests := []struct {
		name      string
		fieldJSON string
		wantField string
	}{
		{"disposition", `"disposition":"abandoned"`, "disposition"},
		{"disposition_reason", `"disposition_reason":"synthetic reason text"`, "disposition_reason"},
		{"disposition_operator", `"disposition_operator":"synthetic-operator"`, "disposition_operator"},
		{"disposition_at", `"disposition_at":"2026-01-01T00:00:00Z"`, "disposition_at"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			stateDump := fmt.Sprintf(
				`{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","status":"active",`+
					`"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z",%s}`,
				tt.fieldJSON,
			)

			_, err := events.CreateCheckpoint(context.Background(), dir, stateDump)
			fields := unknownFieldsFromErr(t, err)
			assert.Equal(t, []string{tt.wantField}, fields)
			assertNoCheckpointWritten(t, dir)
		})
	}
}

// TestCreateCheckpoint_ForgedAbandonedDispositionCannotBypassAudit is an
// end-to-end regression for the same Copilot-flagged gap: a dump that
// supplies BOTH status:"abandoned" and disposition:"abandoned" together
// (the exact shape that would have silently defeated AbandonCheckpoint's
// audit trail) is rejected at create, naming BOTH the reserved key
// (disposition) and the reserved status value (status) as offending.
func TestCreateCheckpoint_ForgedAbandonedDispositionCannotBypassAudit(t *testing.T) {
	dir := t.TempDir()
	stateDump := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","status":"abandoned",` +
		`"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z","disposition":"abandoned"}`

	_, err := events.CreateCheckpoint(context.Background(), dir, stateDump)
	fields := unknownFieldsFromErr(t, err)
	assert.Equal(t, []string{"disposition", "status"}, fields)
	assertNoCheckpointWritten(t, dir)
}

// TestCreateCheckpoint_RejectsStatusAbandonedWithoutDisposition closes the
// gap Copilot review found even after the disposition-field reservation
// above: "status" itself remains a legal top-level key, so a dump supplying
// status:"abandoned" WITH NO disposition fields at all still passed both
// checkClosedSchemaNamespace (status is legal) and ValidateCheckpoint
// (abandoned is a legal Status enum value) — persisting a checkpoint that
// LOOKS abandoned but was never audited. Worse, it could never be repaired
// through the governed operation afterward: AbandonCheckpoint refuses any
// non-"active" status before it would ever reach its own
// already-abandoned idempotent check. checkClosedSchemaNamespace now also
// rejects the reserved status value directly.
func TestCreateCheckpoint_RejectsStatusAbandonedWithoutDisposition(t *testing.T) {
	dir := t.TempDir()
	stateDump := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","status":"abandoned",` +
		`"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}`

	_, err := events.CreateCheckpoint(context.Background(), dir, stateDump)
	fields := unknownFieldsFromErr(t, err)
	assert.Equal(t, []string{"status"}, fields)
	assertNoCheckpointWritten(t, dir)
}

// TestCreateCheckpoint_StatusResolvedAndActiveStillAccepted pins the
// non-reserved status values: "active" (the default) and "resolved" (a
// caller may legitimately import an already-closed session directly,
// since resolve carries no audit-trail requirement) both still succeed at
// create.
func TestCreateCheckpoint_StatusResolvedAndActiveStillAccepted(t *testing.T) {
	for _, status := range []string{"active", "resolved"} {
		t.Run(status, func(t *testing.T) {
			dir := t.TempDir()
			stateDump := fmt.Sprintf(
				`{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","status":%q,`+
					`"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}`,
				status,
			)

			result, err := events.CreateCheckpoint(context.Background(), dir, stateDump)
			require.NoError(t, err)
			assert.FileExists(t, result.Path)
		})
	}
}

// TestCreateCheckpoint_NormalCreateOmitsDispositionAt is a second Copilot
// review remediation on PR #373: CheckpointV1.DispositionAt was a value-typed
// time.Time with a ",omitempty" tag that encoding/json cannot honor for a
// struct zero value, so an ORDINARY create with no disposition fields
// supplied at all was still writing the literal
// "disposition_at":"0001-01-01T00:00:00Z" to disk. Once disposition_at
// became a reserved, create-rejected field (see
// TestCreateCheckpoint_RejectsReservedDispositionFields above), that
// spurious zero-value member made a checkpoint's own freshly written bytes
// fail if ever resubmitted through CreateCheckpoint — a checkpoint could not
// round-trip through itself. DispositionAt is now *time.Time, which
// correctly omits under ",omitempty" when nil.
func TestCreateCheckpoint_NormalCreateOmitsDispositionAt(t *testing.T) {
	dir := t.TempDir()
	stateDump := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build"}`

	result, err := events.CreateCheckpoint(context.Background(), dir, stateDump)
	require.NoError(t, err)

	raw, err := os.ReadFile(result.Path)
	require.NoError(t, err)

	var doc map[string]any
	require.NoError(t, json.Unmarshal(raw, &doc))
	_, hasDispositionAt := doc["disposition_at"]
	assert.False(t, hasDispositionAt, "a normal create must never persist the reserved disposition_at field")

	// The written bytes must themselves be accepted by a second create call
	// (round-trip), which is exactly the property the spurious zero-value
	// field broke.
	_, err = events.CreateCheckpoint(context.Background(), dir, string(raw))
	assert.NoError(t, err, "a freshly written checkpoint's own bytes must round-trip through CreateCheckpoint")
}
