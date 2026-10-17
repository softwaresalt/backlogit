package evidencedag

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const verdictHead = "0123456789abcdef0123456789abcdef01234567"

func verdictDecl() *Declaration {
	mk := func(id string, terminal bool, fam Family) Node {
		return Node{
			ID: id, Family: fam, Applicability: &Applicability{Always: true},
			Validator: "backlogit/policyengine@v1", ProducingTask: "163.002-T",
			ProducingCommit: CommitHead, OnUnmet: OnUnmetBlock, Terminal: terminal,
		}
	}
	return &Declaration{
		SchemaVersion: SchemaVersion,
		Nodes: []Node{
			mk("first", false, FamilyWorkflowPolicy),
			mk("second", true, FamilyWorkflowPolicy),
			mk("s4node", false, FamilyParity),
		},
		Edges: []Edge{{Node: "second", DependsOn: "first"}, {Node: "second", DependsOn: "s4node"}},
	}
}

func verdictShip() Shipment { return Shipment{ID: "163.002-T", HeadCommit: verdictHead} }

func TestU163_002_VerdictsDecideWorkflowPolicyNodesAndPropagate(t *testing.T) {
	ev, err := NewEvaluator(verdictDecl())
	require.NoError(t, err)

	rep, err := ev.Evaluate(Input{Shipment: verdictShip(), Verdicts: map[string]Verdict{
		"first": {Status: StatusUnmet, Reason: "no evidence"},
	}})
	require.NoError(t, err)
	first, _ := rep.Node("first")
	second, _ := rep.Node("second")
	assert.Equal(t, StatusUnmet, first.Status)
	assert.Equal(t, "no evidence", first.Reason)
	assert.NotEqual(t, StatusVerified, second.Status, "an unmet upstream must propagate")
}

func TestU163_002_VerdictsVerifyOnlyWhenEveryUpstreamIsDecided(t *testing.T) {
	d := verdictDecl()
	d.Nodes = d.Nodes[:2]
	d.Edges = d.Edges[:1]
	ev, err := NewEvaluator(d)
	require.NoError(t, err)
	rep, err := ev.Evaluate(Input{Shipment: verdictShip(), Verdicts: map[string]Verdict{
		"first": {Status: StatusVerified}, "second": {Status: StatusVerified},
	}})
	require.NoError(t, err)
	assert.Equal(t, RollupReady, rep.Rollup().State)
}

func TestU163_002_MissingVerdictFailsClosed(t *testing.T) {
	d := verdictDecl()
	d.Nodes = d.Nodes[:2]
	d.Edges = d.Edges[:1]
	ev, err := NewEvaluator(d)
	require.NoError(t, err)
	rep, err := ev.Evaluate(Input{Shipment: verdictShip(), Verdicts: map[string]Verdict{"second": {Status: StatusVerified}}})
	require.NoError(t, err)
	first, _ := rep.Node("first")
	assert.Equal(t, StatusError, first.Status, "a workflow-policy node without a verdict has no S4 evidence and must error")
	assert.NotEqual(t, RollupReady, rep.Rollup().State)
}

func TestU163_002_VerdictsAreRejectedForInvalidTargets(t *testing.T) {
	ev, err := NewEvaluator(verdictDecl())
	require.NoError(t, err)
	cases := map[string]Input{
		"undeclared node":        {Verdicts: map[string]Verdict{"ghost": {Status: StatusVerified}}},
		"non-policy family node": {Verdicts: map[string]Verdict{"s4node": {Status: StatusVerified}}},
		"waived status":          {Verdicts: map[string]Verdict{"first": {Status: StatusWaived}}},
		"not-applicable status":  {Verdicts: map[string]Verdict{"first": {Status: StatusNotApplicable}}},
		"empty status":           {Verdicts: map[string]Verdict{"first": {}}},
		"verdict plus failure":   {Verdicts: map[string]Verdict{"first": {Status: StatusVerified}}, Failures: map[string]string{"first": "x"}},
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			in.Shipment = verdictShip()
			_, err := ev.Evaluate(in)
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrInvalidInput)
		})
	}
}

func TestU163_002_VerdictChangeInvalidatesTheCache(t *testing.T) {
	d := verdictDecl()
	d.Nodes = d.Nodes[:2]
	d.Edges = d.Edges[:1]
	ev, err := NewEvaluator(d)
	require.NoError(t, err)
	ok := map[string]Verdict{"first": {Status: StatusVerified}, "second": {Status: StatusVerified}}
	bad := map[string]Verdict{"first": {Status: StatusVerified}, "second": {Status: StatusUnmet, Reason: "later"}}

	r1, err := ev.Evaluate(Input{Shipment: verdictShip(), Verdicts: ok})
	require.NoError(t, err)
	r2, err := ev.Evaluate(Input{Shipment: verdictShip(), Verdicts: bad})
	require.NoError(t, err)
	r3, err := ev.Evaluate(Input{Shipment: verdictShip(), Verdicts: ok})
	require.NoError(t, err)
	assert.Equal(t, RollupReady, r1.Rollup().State)
	assert.Equal(t, RollupBlocked, r2.Rollup().State, "a stale cached verdict must never be served")
	assert.Equal(t, RollupReady, r3.Rollup().State)
}

func TestU163_002_GateCheckConsumesVerdicts(t *testing.T) {
	d := verdictDecl()
	d.Nodes = d.Nodes[:2]
	d.Edges = d.Edges[:1]
	g, err := NewGate(d, ModeEnforce, nil)
	require.NoError(t, err)
	dec, err := g.Check(context.Background(), GateInput{Shipment: verdictShip(), Verdicts: map[string]Verdict{
		"first": {Status: StatusVerified}, "second": {Status: StatusVerified},
	}})
	require.NoError(t, err)
	assert.True(t, dec.ReviewReady)

	dec, err = g.Check(context.Background(), GateInput{Shipment: verdictShip(), Verdicts: map[string]Verdict{
		"first": {Status: StatusUnmet, Reason: strings.Repeat("x", 1000)}, "second": {Status: StatusVerified},
	}})
	require.NoError(t, err)
	assert.False(t, dec.ReviewReady)
	assert.False(t, dec.Allowed)
	first, _ := dec.Report.Node("first")
	assert.LessOrEqual(t, len(first.Reason), MaxReasonBytes, "verdict reasons must be bounded")

	_, err = g.Check(context.Background(), GateInput{Shipment: verdictShip(), Verdicts: map[string]Verdict{"ghost": {Status: StatusVerified}}})
	assert.ErrorIs(t, err, ErrInvalidInput)
}

func TestU163_002_VerdictsDoNotAffectDeclarationsWithoutThem(t *testing.T) {
	ev, err := NewEvaluator(verdictDecl())
	require.NoError(t, err)
	rep, err := ev.Evaluate(Input{Shipment: verdictShip()})
	require.NoError(t, err)
	first, _ := rep.Node("first")
	assert.Equal(t, StatusError, first.Status)
}
