package docline

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 154.001-T (U1) harness: NewMigrateReport must produce Applied and Skipped
// as non-nil, zero-length slices so they marshal as [] (not null or absent)
// for BOTH the dry-run (res==nil) and the zero-apply (res!=nil, empty results)
// cases. The harness decodes the marshalled JSON and asserts those two keys
// directly — not their sibling fields.
//
// RED until:
//   - MigrateReport.Applied / .Skipped omitempty tags are removed, AND
//   - NewMigrateReport initialises both to []string{} before the res guard.

// TestU1MigrateReportArrays_DryRunAppliedSkippedAreArrays covers the dry-run
// path (res == nil): applied and skipped must appear as [] in the JSON output,
// never null or absent.
func TestU1MigrateReportArrays_DryRunAppliedSkippedAreArrays(t *testing.T) {
	t.Parallel()

	plan := MigrationPlan{Changes: []Change{}}
	report := NewMigrateReport(plan, nil, true) // dry-run: res == nil

	raw, err := json.Marshal(report)
	require.NoError(t, err, "MigrateReport must marshal without error")

	var decoded map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &decoded), "JSON must round-trip to a map")

	appliedRaw, ok := decoded["applied"]
	require.True(t, ok, "applied key must be present in dry-run JSON (always-array contract)")
	assert.Equal(t, "[]", string(appliedRaw),
		"applied must be [] not null in dry-run JSON")

	skippedRaw, ok := decoded["skipped"]
	require.True(t, ok, "skipped key must be present in dry-run JSON (always-array contract)")
	assert.Equal(t, "[]", string(skippedRaw),
		"skipped must be [] not null in dry-run JSON")
}

// TestU1MigrateReportArrays_ZeroApplyAppliedSkippedAreArrays covers the
// zero-apply path (res != nil, but Applied and Skipped are both nil/empty):
// the always-array contract must hold even when no file was applied or skipped.
func TestU1MigrateReportArrays_ZeroApplyAppliedSkippedAreArrays(t *testing.T) {
	t.Parallel()

	plan := MigrationPlan{Changes: []Change{}}
	res := &Result{Applied: nil, Skipped: nil} // zero-apply: res != nil but empty
	report := NewMigrateReport(plan, res, false)

	raw, err := json.Marshal(report)
	require.NoError(t, err, "MigrateReport must marshal without error")

	var decoded map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &decoded), "JSON must round-trip to a map")

	appliedRaw, ok := decoded["applied"]
	require.True(t, ok, "applied key must be present in zero-apply JSON (always-array contract)")
	assert.Equal(t, "[]", string(appliedRaw),
		"applied must be [] not null for a zero-apply result")

	skippedRaw, ok := decoded["skipped"]
	require.True(t, ok, "skipped key must be present in zero-apply JSON (always-array contract)")
	assert.Equal(t, "[]", string(skippedRaw),
		"skipped must be [] not null for a zero-apply result")
}
