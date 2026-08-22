package events_test

import (
	"context"
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
