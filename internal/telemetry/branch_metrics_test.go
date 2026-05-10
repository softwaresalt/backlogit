package telemetry_test

import (
	"errors"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/telemetry"
)

// ---- ExtractArtifactIDs -----------------------------------------------------

func TestExtractArtifactIDs_FeatureBranch(t *testing.T) {
	ship, feat := telemetry.ExtractArtifactIDs("feature/057-f-schema-discoverability")
	assert.Equal(t, "", ship)
	assert.Equal(t, "057-F", feat)
}

func TestExtractArtifactIDs_FeatureBranch_UpperF(t *testing.T) {
	ship, feat := telemetry.ExtractArtifactIDs("feature/058-F-branch-metrics")
	assert.Equal(t, "", ship)
	assert.Equal(t, "058-F", feat)
}

func TestExtractArtifactIDs_ShipBranch(t *testing.T) {
	ship, feat := telemetry.ExtractArtifactIDs("ship/055s-lifecycle-hygiene")
	assert.Equal(t, "055-S", ship)
	assert.Equal(t, "", feat)
}

func TestExtractArtifactIDs_StageBranch(t *testing.T) {
	ship, feat := telemetry.ExtractArtifactIDs("chore/stage-056-s-schema-discoverability")
	assert.Equal(t, "056-S", ship)
	assert.Equal(t, "", feat)
}

func TestExtractArtifactIDs_StageBranch_CompactFormat(t *testing.T) {
	ship, feat := telemetry.ExtractArtifactIDs("chore/stage-056s-schema")
	assert.Equal(t, "056-S", ship)
	assert.Equal(t, "", feat)
}

func TestExtractArtifactIDs_ChoreBranch_NoMatch(t *testing.T) {
	ship, feat := telemetry.ExtractArtifactIDs("chore/copilot-review-fixes")
	assert.Equal(t, "", ship)
	assert.Equal(t, "", feat)
}

func TestExtractArtifactIDs_PostMergeBranch_NoMatch(t *testing.T) {
	ship, feat := telemetry.ExtractArtifactIDs("post-merge/autoharness-tune")
	assert.Equal(t, "", ship)
	assert.Equal(t, "", feat)
}

func TestExtractArtifactIDs_Empty(t *testing.T) {
	ship, feat := telemetry.ExtractArtifactIDs("")
	assert.Equal(t, "", ship)
	assert.Equal(t, "", feat)
}

// ---- AggregateBranches ------------------------------------------------------

func TestAggregateBranches_GroupsByBranch(t *testing.T) {
	now := time.Now()
	sessions := []telemetry.SessionSummaryRecord{
		{Branch: "feature/001-f-foo", TotalTokens: 1000, ModelCalls: 10, ToolCalls: 5, HarvestedAt: now.Add(-2 * time.Hour)},
		{Branch: "feature/001-f-foo", TotalTokens: 2000, ModelCalls: 20, ToolCalls: 10, HarvestedAt: now.Add(-1 * time.Hour)},
		{Branch: "chore/fix-x", TotalTokens: 500, ModelCalls: 5, ToolCalls: 3, HarvestedAt: now},
	}

	profiles := telemetry.AggregateBranches(sessions)
	require.Len(t, profiles, 2)

	// Sorted by LastSeen desc — chore/fix-x is most recent
	assert.Equal(t, "chore/fix-x", profiles[0].Branch)
	assert.Equal(t, "chore", profiles[0].BranchType)
	assert.Equal(t, 1, profiles[0].Sessions)
	assert.Equal(t, 500, profiles[0].TotalTokens)

	assert.Equal(t, "feature/001-f-foo", profiles[1].Branch)
	assert.Equal(t, "feature", profiles[1].BranchType)
	assert.Equal(t, 2, profiles[1].Sessions)
	assert.Equal(t, 3000, profiles[1].TotalTokens)
	assert.InDelta(t, 1500.0, profiles[1].AvgTokens, 0.01)
	assert.Equal(t, "001-F", profiles[1].FeatureID)
}

func TestAggregateBranches_FiltersGhostSessions(t *testing.T) {
	now := time.Now()
	sessions := []telemetry.SessionSummaryRecord{
		{Branch: "feature/a", TotalTokens: 1000, ModelCalls: 10, ToolCalls: 5, HarvestedAt: now},
		{Branch: "feature/a", TotalTokens: 0, ModelCalls: 0, ToolCalls: 0, HarvestedAt: now}, // ghost
	}

	profiles := telemetry.AggregateBranches(sessions)
	require.Len(t, profiles, 1)
	assert.Equal(t, 1, profiles[0].Sessions) // ghost excluded
	assert.Equal(t, 1000, profiles[0].TotalTokens)
}

func TestAggregateBranches_Empty(t *testing.T) {
	profiles := telemetry.AggregateBranches(nil)
	assert.Empty(t, profiles)
}

func TestAggregateBranches_TaskCount(t *testing.T) {
	now := time.Now()
	sessions := []telemetry.SessionSummaryRecord{
		{Branch: "feature/x", TotalTokens: 100, ModelCalls: 1, ToolCalls: 1, CompletedTasks: []string{"T1", "T2"}, HarvestedAt: now},
		{Branch: "feature/x", TotalTokens: 200, ModelCalls: 2, ToolCalls: 2, CompletedTasks: []string{"T3"}, HarvestedAt: now},
	}

	profiles := telemetry.AggregateBranches(sessions)
	require.Len(t, profiles, 1)
	assert.Equal(t, 3, profiles[0].TaskCount)
}

func TestAggregateBranches_PeakUtilAverage(t *testing.T) {
	now := time.Now()
	p1 := 0.8
	p2 := 0.6
	sessions := []telemetry.SessionSummaryRecord{
		{Branch: "feature/x", TotalTokens: 100, ModelCalls: 1, ToolCalls: 1, PeakUtilization: &p1, HarvestedAt: now},
		{Branch: "feature/x", TotalTokens: 200, ModelCalls: 2, ToolCalls: 2, PeakUtilization: &p2, HarvestedAt: now},
	}

	profiles := telemetry.AggregateBranches(sessions)
	require.Len(t, profiles, 1)
	require.NotNil(t, profiles[0].AvgPeakUtil)
	assert.InDelta(t, 0.7, *profiles[0].AvgPeakUtil, 0.01)
}

func TestAggregateBranches_FirstSeenLastSeen(t *testing.T) {
	t1 := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 5, 2, 10, 0, 0, 0, time.UTC)
	sessions := []telemetry.SessionSummaryRecord{
		{Branch: "feature/x", TotalTokens: 100, ModelCalls: 1, ToolCalls: 1, HarvestedAt: t1},
		{Branch: "feature/x", TotalTokens: 200, ModelCalls: 2, ToolCalls: 2, HarvestedAt: t2},
		{Branch: "feature/x", TotalTokens: 150, ModelCalls: 1, ToolCalls: 1, HarvestedAt: t3},
	}

	profiles := telemetry.AggregateBranches(sessions)
	require.Len(t, profiles, 1)
	assert.Equal(t, t1, profiles[0].FirstSeen)
	assert.Equal(t, t2, profiles[0].LastSeen)
}

// ---- parseMergeLines --------------------------------------------------------

func TestParseMergeLines_Standard(t *testing.T) {
	input := `619cb3e Merge pull request #110 from softwaresalt/chore/copilot-review-fixes-108-109
ef28b29 Merge pull request #109 from softwaresalt/feature/057-f-schema-discoverability
f1c639d Merge pull request #108 from softwaresalt/chore/stage-056-s-schema-discoverability
9752238 Merge pull request #106 from softwaresalt/ship/055s-lifecycle-hygiene
`
	result, err := telemetry.ParseMergeLines(strings.NewReader(input))
	require.NoError(t, err)

	assert.Equal(t, "#110", result["chore/copilot-review-fixes-108-109"])
	assert.Equal(t, "#109", result["feature/057-f-schema-discoverability"])
	assert.Equal(t, "#108", result["chore/stage-056-s-schema-discoverability"])
	assert.Equal(t, "#106", result["ship/055s-lifecycle-hygiene"])
}

func TestParseMergeLines_NonMergeLines(t *testing.T) {
	input := `2cda22b feat(db): add SQL schema introspection
619cb3e Merge pull request #110 from softwaresalt/chore/fixes
some random text
`
	result, err := telemetry.ParseMergeLines(strings.NewReader(input))
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "#110", result["chore/fixes"])
}

func TestParseMergeLines_Empty(t *testing.T) {
	result, err := telemetry.ParseMergeLines(strings.NewReader(""))
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestParseMergeLines_DuplicateBranch_KeepsFirst(t *testing.T) {
	// If a branch was PR'd multiple times, keep the first (most recent in git log).
	input := `aaa Merge pull request #200 from owner/feature/x
bbb Merge pull request #100 from owner/feature/x
`
	result, err := telemetry.ParseMergeLines(strings.NewReader(input))
	require.NoError(t, err)
	assert.Equal(t, "#200", result["feature/x"])
}

func TestParseMergeLines_ReaderError(t *testing.T) {
	// A failing reader must surface as an error from ParseMergeLines.
	r := iotest.ErrReader(errors.New("simulated I/O failure"))
	_, err := telemetry.ParseMergeLines(r)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "scan merge lines")
}

// ---- EnrichBranchProfiles ---------------------------------------------------

func TestEnrichBranchProfiles_MatchesAndMisses(t *testing.T) {
	profiles := []telemetry.BranchProfile{
		{Branch: "feature/057-f-slug"},
		{Branch: "chore/fix-x"},
	}
	prMap := map[string]string{
		"feature/057-f-slug": "#109",
	}

	telemetry.EnrichBranchProfiles(profiles, prMap)
	assert.Equal(t, "#109", profiles[0].PRNumber)
	assert.Equal(t, "", profiles[1].PRNumber)
}
