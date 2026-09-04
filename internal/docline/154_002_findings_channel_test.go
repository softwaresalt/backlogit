package docline

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 154.002-T (U2a) harness: MigrationPlan must carry Findings []Finding and
// MigrateReport must carry Findings []FindingReport (always-array), mapped in
// NewMigrateReport so a zero-findings plan marshals "findings":[] never
// null or absent.
//
// RED until:
//   - service.go adds Findings []Finding to MigrationPlan, AND
//   - report.go adds Findings []FindingReport to MigrateReport with
//     always-array initialisation in NewMigrateReport.
//
// The reflect-based assertions (TestU2aFindingsChannel_MigrationPlanHasFindings
// and TestU2aFindingsChannel_MigrateReportHasFindings) compile without the
// declarations existing and fail at runtime until the fields are added — this
// is the source-shape harness pattern for declarations (P-002.1, cycle-31).

// TestU2aFindingsChannel_MigrationPlanHasFindings is the source-shape
// assertion for the MigrationPlan.Findings declaration.
func TestU2aFindingsChannel_MigrationPlanHasFindings(t *testing.T) {
	t.Parallel()

	planType := reflect.TypeOf(MigrationPlan{})
	field, ok := planType.FieldByName("Findings")
	require.True(t, ok,
		"MigrationPlan must declare a Findings field; add Findings []Finding to service.go")
	assert.Equal(t, "[]docline.Finding", field.Type.String(),
		"MigrationPlan.Findings must be []Finding")
}

// TestU2aFindingsChannel_MigrateReportHasFindings is the source-shape
// assertion for the MigrateReport.Findings declaration.
func TestU2aFindingsChannel_MigrateReportHasFindings(t *testing.T) {
	t.Parallel()

	reportType := reflect.TypeOf(MigrateReport{})
	field, ok := reportType.FieldByName("Findings")
	require.True(t, ok,
		"MigrateReport must declare a Findings field; add Findings []FindingReport to report.go")
	assert.Equal(t, "[]docline.FindingReport", field.Type.String(),
		"MigrateReport.Findings must be []FindingReport")
}

// TestU2aFindingsChannel_ZeroFindingsMigrateReportMarshalsFindingsAsArray is
// the behavioral always-array assertion: a MigrateReport built from a plan
// with zero findings must marshal "findings":[] not null or absent.
func TestU2aFindingsChannel_ZeroFindingsMigrateReportMarshalsFindingsAsArray(t *testing.T) {
	t.Parallel()

	// Construct a plan with no Findings (pre-U2a the field doesn't exist;
	// NewMigrateReport won't include findings in the JSON at all).
	plan := MigrationPlan{Changes: []Change{}}
	report := NewMigrateReport(plan, nil, true)

	raw, err := json.Marshal(report)
	require.NoError(t, err, "MigrateReport must marshal without error")

	var decoded map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &decoded), "JSON must round-trip to a map")

	findingsRaw, ok := decoded["findings"]
	require.True(t, ok,
		"findings key must be present in MigrateReport JSON (always-array contract)")
	assert.Equal(t, "[]", string(findingsRaw),
		"findings must be [] for a zero-findings plan (never null or absent)")
}
