package soundness

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRemediation_EmptyPlan(t *testing.T) {
	for name, plan := range map[string]Plan{"nil units": {}, "empty units": {Units: []Unit{}}} {
		t.Run(name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.Mode = ModeEnforce
			_, err := Run(plan, cfg)
			require.ErrorIs(t, err, ErrEmptyPlan)
			d, err := Assess(plan, cfg)
			require.ErrorIs(t, err, ErrEmptyPlan)
			assert.True(t, d.Blocked)

			cfg.Mode = ModeReportOnly
			d, err = Assess(plan, cfg)
			require.NoError(t, err)
			require.NotEmpty(t, d.Findings)
			assert.Equal(t, SeverityWarn, d.Findings[0].Severity)
			assert.False(t, d.Blocked)
		})
	}
}

func TestRemediation_MissingArtifact_OutOfRootProducerStillFlagged(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Root = t.TempDir()
	p, c := soundUnit("1-T", "Make"), soundUnit("2-T", "Use")
	p.Deliverables = []string{"../outside.md"}
	c.Requires = []string{"../outside.md"}
	c.Dependencies = []string{"1-T"}
	got := rulesOf(runPlan(t, Plan{Units: []Unit{p, c}}, cfg))["missing-artifact"]
	assert.NotEmpty(t, got)

	p.Deliverables = []string{"/abs/x.md", "a/../../y.md"}
	got = rulesOf(runPlan(t, Plan{Units: []Unit{p}}, cfg))["missing-artifact"]
	assert.Len(t, got, 2)
}

func TestRemediation_MissingArtifact_OrderingWarning(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Root = t.TempDir()
	p, c := soundUnit("1-T", "Make"), soundUnit("2-T", "Use")
	p.Deliverables = []string{"x/new.go"}
	c.Requires = []string{"x/new.go"}
	got := rulesOf(runPlan(t, Plan{Units: []Unit{p, c}}, cfg))["missing-artifact"]
	require.Len(t, got, 1)
	assert.Equal(t, SeverityWarn, got[0].Severity)
	assert.Equal(t, "2-T", got[0].UnitID)

	mid := soundUnit("3-T", "Mid")
	mid.Dependencies = []string{"1-T"}
	c.Dependencies = []string{"3-T"}
	assert.Empty(t, rulesOf(runPlan(t, Plan{Units: []Unit{p, c, mid}}, cfg))["missing-artifact"], "transitive dependency satisfies ordering")
}

func TestRemediation_DuplicatedWork_SameDeliverableTwiceSkipsSelf(t *testing.T) {
	a := soundUnit("1-T", "A")
	a.Deliverables = []string{"x.go", "./x.go"}
	assert.Empty(t, findingsFor(t, "duplicated-work", a))
}

func TestRemediation_VerificationSurface_BlankEntriesDoNotCount(t *testing.T) {
	u := soundUnit("1-T", "A")
	u.Verification = []string{"", "  \t"}
	assert.Len(t, findingsFor(t, "verification-surface", u), 1)
}

func TestRemediation_NormalizePath(t *testing.T) {
	assert.Equal(t, normalizePath("b.go"), normalizePath("a/../b.go"))
	assert.Equal(t, "x/y.go", normalizePath(".\\x\\.\\y.go"))
	assert.Equal(t, "x/parser.go", normalizePath("X/Parser.go"), "paths are case-folded")
	assert.Equal(t, "", normalizePath("  "))
}

func TestRemediation_DependencyOverlappingCycles(t *testing.T) {
	u := map[string]Unit{}
	for _, id := range []string{"1-T", "2-T", "3-T", "4-T"} {
		u[id] = soundUnit(id, id)
	}
	set := func(id string, deps ...string) { x := u[id]; x.Dependencies = deps; u[id] = x }
	set("1-T", "2-T")
	set("2-T", "3-T", "4-T")
	set("3-T", "1-T")
	set("4-T", "1-T")
	got := findingsFor(t, "dependency-consistency", u["1-T"], u["2-T"], u["3-T"], u["4-T"])
	require.Len(t, got, 2, "one finding per back edge")
	assert.NotEqual(t, got[0].Message, got[1].Message)
}
