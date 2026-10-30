package parity_test

// 156.003-T (U3): the recurring-failure corpus for the cross-surface golden
// parity harness. These tests SEED typed scenarios for the known recurring
// failure modes and assert the comparator classifies each correctly:
//
//   - typed-error → exit 1 collapse                → FAIL (exit_code)
//   - domainError mapping drift                    → FAIL (structured_error)
//   - retryability disagreement                    → FAIL (retryability)
//   - omitempty array loss                         → FAIL (serialization)
//   - known CLI gate-payload drift (report_only)   → report_only, tracked 166-F
//
// The harness (not the leaf internal/faultline package) RESOLVES the tracked
// defect to a CURRENT, NON-TERMINAL backlog item. The resolution conformance
// tests prove 166-F (queued) is accepted, 999-F (nonexistent) is rejected, and
// the archived 156.007-T is rejected.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/faultline/parity"
)

// runScenario executes a corpus scenario through CompareResults and runs the
// scenario's own Validate hook against the report.
func runScenario(t *testing.T, sc parity.Scenario) parity.ComparisonReport {
	t.Helper()
	require.NotNil(t, sc.Run, "scenario %q must define Run", sc.Name)
	require.NotNil(t, sc.Validate, "scenario %q must define Validate", sc.Name)

	results := sc.Run(t, nil)
	report, err := parity.CompareResults(context.Background(), sc.Name, results)
	require.NoError(t, err)
	sc.Validate(t, report)
	return report
}

func TestU3SeedCorpus_TypedErrorCollapse(t *testing.T) {
	report := runScenario(t, parity.TypedErrorCollapseScenario())
	require.NotNil(t, report.Evidence)
	require.Equal(t, "fail", report.Evidence.Status)
}

func TestU3SeedCorpus_DomainErrorDrift(t *testing.T) {
	report := runScenario(t, parity.DomainErrorDriftScenario())
	require.NotNil(t, report.Evidence)
	require.Equal(t, "fail", report.Evidence.Status)
}

func TestU3SeedCorpus_RetryabilityDisagreement(t *testing.T) {
	report := runScenario(t, parity.RetryabilityDisagreementScenario())
	require.NotNil(t, report.Evidence)
	require.Equal(t, "fail", report.Evidence.Status)
}

func TestU3SeedCorpus_OmitemptyArrayLoss(t *testing.T) {
	report := runScenario(t, parity.OmitemptyArrayLossScenario())
	require.NotNil(t, report.Evidence)
	require.Equal(t, "fail", report.Evidence.Status)
}

func TestU3SeedCorpus_KnownGatePayloadDrift(t *testing.T) {
	report := runScenario(t, parity.KnownGatePayloadDriftScenario())

	for _, d := range report.Dimensions {
		require.NotEqual(t, parity.StatusReportOnly, d.Status,
			"dimension %q must not be report_only", d.Dimension)
	}
	byName := map[string]string{}
	for _, d := range report.Dimensions {
		byName[d.Dimension] = d.Status
	}
	require.Equal(t, parity.StatusFail, byName["remediation"])
	require.Equal(t, parity.StatusFail, byName["retryability"])
	require.NotNil(t, report.Evidence)
	require.Equal(t, "fail", report.Evidence.Status)
}

// TestU3SeedCorpus_CorpusRegistryComplete asserts SeedCorpus exposes every
// seeded scenario so the corpus can be driven as a table.
func TestU3SeedCorpus_CorpusRegistryComplete(t *testing.T) {
	corpus := parity.SeedCorpus()
	require.Len(t, corpus, 5)
	for _, sc := range corpus {
		sc := sc
		t.Run(sc.Name, func(t *testing.T) {
			runScenario(t, sc)
		})
	}
}

// backlogitDirForResolution locates the CURRENT workspace's `.backlogit`
// storage root by walking up from the test working directory.
func backlogitDirForResolution(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	require.NoError(t, err)
	dir, err := parity.LocateBacklogitDir(wd)
	require.NoError(t, err, "must locate the .backlogit workspace above the test dir")
	return dir
}

func TestU3SeedCorpus_TrackedDefectResolution_Valid(t *testing.T) {
	// Hermetic fixture: independent of the live workspace lifecycle of 166-F.
	dir := filepath.Join(t.TempDir(), ".backlogit")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "queue"), 0o750))
	writeItem := func(id, status string) {
		t.Helper()
		content := "---\nid: " + id + "\nstatus: " + status + "\n---\n\nbody\n"
		require.NoError(t, os.WriteFile(filepath.Join(dir, "queue", id+".md"), []byte(content), 0o600))
	}
	writeItem("166-F", "queued")
	writeItem("900-F", "done")

	require.Equal(t, "166-F", parity.TrackedDefectGatePayload)
	require.NoError(t, parity.ResolveTrackedDefect(dir, parity.TrackedDefectGatePayload),
		"non-terminal queued item must resolve")
	err := parity.ResolveTrackedDefect(dir, "900-F")
	require.ErrorIs(t, err, parity.ErrTrackedDefectTerminal)

	// Any report_only dimension in the corpus must resolve against the live workspace.
	live := backlogitDirForResolution(t)
	for _, sc := range parity.SeedCorpus() {
		results := sc.Run(t, nil)
		report, cmpErr := parity.CompareResults(context.Background(), sc.Name, results)
		require.NoError(t, cmpErr)
		for _, d := range report.Dimensions {
			if d.Status == parity.StatusReportOnly {
				require.NoError(t, parity.ResolveTrackedDefect(live, d.TrackedDefect))
			}
		}
	}
}

func TestU3SeedCorpus_TrackedDefectResolution_Nonexistent(t *testing.T) {
	dir := backlogitDirForResolution(t)
	err := parity.ResolveTrackedDefect(dir, "999-F")
	require.Error(t, err, "a nonexistent tracked defect must be rejected")
	require.ErrorIs(t, err, parity.ErrTrackedDefectNotFound)
}

func TestU3SeedCorpus_TrackedDefectResolution_Archived(t *testing.T) {
	dir := backlogitDirForResolution(t)
	// 156.007-T was the original planned owner but is now archived (terminal).
	err := parity.ResolveTrackedDefect(dir, "156.007-T")
	require.Error(t, err, "an archived tracked defect must be rejected")
	require.True(t,
		errors.Is(err, parity.ErrTrackedDefectTerminal) || errors.Is(err, parity.ErrTrackedDefectNotFound),
		"archived ID must be rejected as terminal or absent, got: %v", err)
}
