package soundness

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// generousTimeout is a hang guard, not a performance bound: the real
// efficiency checks below are structural.
const generousTimeout = 30 * time.Second

// runPlanWithin runs the engine in a goroutine that touches no testing.T and
// hands the result back over a buffered channel; every assertion happens on
// the test goroutine.
func runPlanWithin(t *testing.T, plan Plan) Report {
	t.Helper()
	type result struct {
		report Report
		err    error
	}
	ch := make(chan result, 1)
	go func() {
		r, err := Run(plan, DefaultConfig())
		ch <- result{r, err}
	}()
	select {
	case res := <-ch:
		require.NoError(t, res.err)
		return res.report
	case <-time.After(generousTimeout):
		t.Fatalf("engine did not complete within %s", generousTimeout)
		return Report{}
	}
}

func TestCopilotR4_Contradiction_DecisionVsUnitIsBounded(t *testing.T) {
	const n = 1024
	plan := Plan{}
	for i := 0; i < n; i++ {
		plan.Decisions = append(plan.Decisions, Decision{ID: fmt.Sprintf("D%d", i), Directives: []Directive{{Subject: "use cgo", Stance: StanceForbid}}})
		u := soundUnit(fmt.Sprintf("%d-T", i+1), "U")
		u.Directives = []Directive{{Subject: "use cgo", Stance: StanceRequire}}
		plan.Units = append(plan.Units, u)
	}
	got := rulesOf(runPlanWithin(t, plan))["contradiction"]
	require.NotEmpty(t, got)
	assert.LessOrEqual(t, len(got), 10)
	assert.Contains(t, got[0].Message, "D0")
}

func TestCopilotR4_Contradiction_AggregatesPerUnitSubject(t *testing.T) {
	u := soundUnit("1-T", "A")
	u.Directives = []Directive{{Subject: "use cgo", Stance: StanceRequire}}
	p := Plan{Units: []Unit{u}, Decisions: []Decision{
		{ID: "D1", Directives: []Directive{{Subject: "use cgo", Stance: StanceForbid}}},
		{ID: "D2", Directives: []Directive{{Subject: "use cgo", Stance: StanceForbid}}},
	}}
	got := rulesOf(runPlan(t, p, DefaultConfig()))["contradiction"]
	require.Len(t, got, 1)
	assert.Contains(t, got[0].Message, "D1")
	assert.Contains(t, got[0].Message, "2 decisions")
}

func chainUnits(title, deliverable string) []Unit {
	a, b, c := soundUnit("1-T", title), soundUnit("2-T", title), soundUnit("3-T", title)
	b.Dependencies = []string{"1-T"}
	c.Dependencies = []string{"2-T"}
	if deliverable != "" {
		a.Deliverables, b.Deliverables, c.Deliverables = []string{deliverable}, []string{deliverable}, []string{deliverable}
	}
	return []Unit{a, b, c}
}

func TestCopilotR4_DuplicatedWork_TransitiveChainPasses(t *testing.T) {
	assert.Empty(t, findingsFor(t, "duplicated-work", chainUnits("T", "x.go")...))
}

func TestCopilotR4_UniqueTitles_TransitiveChainPasses(t *testing.T) {
	assert.Empty(t, findingsFor(t, "unique-titles", chainUnits("Same title", "")...))
}

func TestCopilotR4_Related_CyclicDependenciesTerminate(t *testing.T) {
	us := chainUnits("T", "x.go")
	us[0].Dependencies = []string{"3-T"}
	assert.Empty(t, rulesOf(runPlanWithin(t, Plan{Units: us}))["duplicated-work"])
}

func TestCopilotR4_Related_LargeChainIsEfficient(t *testing.T) {
	var units []Unit
	for i := 0; i < 2000; i++ {
		u := soundUnit(fmt.Sprintf("%d-T", i+1), "Same")
		u.Deliverables = []string{"x.go"}
		if i > 0 {
			u.Dependencies = []string{fmt.Sprintf("%d-T", i)}
		}
		units = append(units, u)
	}
	got := rulesOf(runPlanWithin(t, Plan{Units: units}))
	assert.Empty(t, got["duplicated-work"])
	assert.Empty(t, got["unique-titles"])

	// Structural bound: at most two graph walks per unit per rule (up and
	// down), regardless of how many prior owners each unit has, and no
	// per-unit reach sets are retained on the context.
	ctx := &checkContext{plan: Plan{Units: units}, cfg: DefaultConfig(), byID: map[string]*Unit{}}
	structuralFindings(ctx)
	_, err := checkDuplicatedWork(ctx)
	require.NoError(t, err)
	_, err = checkUniqueTitles(ctx)
	require.NoError(t, err)
	assert.LessOrEqual(t, ctx.walks, 2*2*len(units))
}

func TestCopilotR4_Related_UnrelatedStillFlagged(t *testing.T) {
	a, b, c := soundUnit("1-T", "Same"), soundUnit("2-T", "Same"), soundUnit("3-T", "Same")
	b.Dependencies = []string{"1-T"}
	for _, u := range []*Unit{&a, &b, &c} {
		u.Deliverables = []string{"x.go"}
	}
	assert.Len(t, findingsFor(t, "duplicated-work", a, b, c), 1)
	assert.Len(t, findingsFor(t, "unique-titles", a, b, c), 1)
}

func TestCopilotR4_DuplicatedWork_RepeatedPathsFromOneUnit(t *testing.T) {
	a, b, c := soundUnit("1-T", "A"), soundUnit("2-T", "B"), soundUnit("3-T", "C")
	b.Dependencies = []string{"1-T"}
	a.Deliverables = []string{"x.go"}
	for i := 0; i < 100000; i++ {
		b.Deliverables = append(b.Deliverables, "x.go")
	}
	c.Deliverables = []string{"x.go"}
	got := rulesOf(runPlanWithin(t, Plan{Units: []Unit{a, b, c}}))["duplicated-work"]
	require.Len(t, got, 1)
	assert.Equal(t, "3-T", got[0].UnitID)
}

// --- self-audit regressions ---

func TestCopilotR4_Gate_ZeroValueDoesNotPanic(t *testing.T) {
	var g Gate
	d, err := g.Assess(Plan{Units: []Unit{soundUnit("1-T", "A")}})
	require.ErrorIs(t, err, ErrInvalidConfig, "zero config is rejected explicitly instead of panicking on a nil engine")
	assert.True(t, d.Blocked, "a zero-value gate must fail closed")
}

func TestCopilotR4_NewGate_IsolatedFromCallerMutation(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Mode = ModeEnforce
	cfg.CanonicalProcedures = []string{"tdd"}
	g, err := NewGate(cfg)
	require.NoError(t, err)
	cfg.Rules["verification-surface"] = false
	cfg.CanonicalProcedures[0] = "other"
	u := soundUnit("1-T", "A")
	u.Verification = nil
	u.Procedure = "tdd"
	d, err := g.Assess(Plan{Units: []Unit{u}})
	require.NoError(t, err)
	assert.True(t, d.Blocked, "later caller mutation must not disable a rule on an existing gate")
	assert.Contains(t, d.Report.RulesRun, "verification-surface")
	assert.NotContains(t, rulesOf(Report{Findings: d.Findings}), "canonical-procedure")
}

func TestCopilotR4_ImpossibleTest_DeterministicMessage(t *testing.T) {
	u := soundUnit("1-T", "A")
	u.Tests = []TestSpec{{Name: "T", Assertions: []TestAssertion{
		{Subject: "zeta", Op: OpEqual, Value: "1"}, {Subject: "zeta", Op: OpNotEqual, Value: "1"},
		{Subject: "alpha", Op: OpEqual, Value: "1"}, {Subject: "alpha", Op: OpNotEqual, Value: "1"},
		{Subject: "mid", Op: OpEqual, Value: "1"}, {Subject: "mid", Op: OpNotEqual, Value: "1"},
	}}}
	for i := 0; i < 40; i++ {
		got := findingsFor(t, "impossible-test", u)
		require.Len(t, got, 1)
		assert.Contains(t, got[0].Message, `"zeta"`, "first conflicting assertion in declaration order")
	}
}

func TestCopilotR4_MissingArtifact_RepeatedRequiresReportOnce(t *testing.T) {
	u := soundUnit("1-T", "A")
	for i := 0; i < 50; i++ {
		u.Requires = append(u.Requires, "nowhere/missing.go")
	}
	cfg := DefaultConfig()
	cfg.Root = t.TempDir()
	got := rulesOf(runPlan(t, Plan{Units: []Unit{u}}, cfg))["missing-artifact"]
	assert.Len(t, got, 1)
}

func TestCopilotR4_Verifiability_RepeatedVerifiedWithReportsOnce(t *testing.T) {
	u := soundUnit("1-T", "A")
	u.VerifiedWith = []string{"9-T", "9-T", "9-T"}
	got := findingsFor(t, "independent-verifiability", u)
	require.Len(t, got, 1)
	assert.True(t, strings.Contains(got[0].Message, "9-T"))
}

func TestCopilotR4_Verifiability_TransitiveDependencyIsDeclared(t *testing.T) {
	us := chainUnits("T", "")
	us[2].VerifiedWith = []string{"1-T"}
	assert.Empty(t, findingsFor(t, "independent-verifiability", us...), "verification through a transitive dependency is ordered")
}
