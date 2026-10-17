package evidencedag

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/faultline"
)

const (
	testHead  = "1111111111111111111111111111111111111111"
	otherHead = "2222222222222222222222222222222222222222"
	parityVal = "backlogit/parity@v1"
	testTaskA = "162.001-T"
	testTaskB = "162.002-T"
	validYAML = "schema_version: 1\nnodes:\n  - id: parity-base\n    family: parity\n    applicability: {always: true}\n    validator: backlogit/parity@v1\n    producing_task: 162.001-T\n    producing_commit: head\n    on_unmet: block\n    terminal: false\n  - id: parity-term\n    family: parity\n    applicability: {always: true}\n    validator: backlogit/parity@v1\n    producing_task: 162.001-T\n    producing_commit: head\n    on_unmet: block\n    terminal: true\nedges:\n  - {node: parity-term, depends_on: parity-base}\n"
	validJSON = `{"schema_version":1,"nodes":[{"id":"parity-base","family":"parity","applicability":{"always":true},"validator":"backlogit/parity@v1","producing_task":"162.001-T","producing_commit":"head","on_unmet":"block","terminal":false},{"id":"parity-term","family":"parity","applicability":{"always":true},"validator":"backlogit/parity@v1","producing_task":"162.001-T","producing_commit":"head","on_unmet":"block","terminal":true}],"edges":[{"node":"parity-term","depends_on":"parity-base"}]}`
)

func pnode(id string, terminal bool) Node {
	return Node{
		ID: id, Family: FamilyParity, Applicability: &Applicability{Always: true},
		Validator: parityVal, ProducingTask: testTaskA, ProducingCommit: CommitHead,
		OnUnmet: OnUnmetBlock, Terminal: terminal,
	}
}

func decl(nodes []Node, edges ...Edge) *Declaration {
	return &Declaration{SchemaVersion: SchemaVersion, Nodes: nodes, Edges: edges}
}

func ship() Shipment {
	return Shipment{ID: "144-S", HeadCommit: testHead, Attributes: map[string]string{"surface": "cli"}}
}

func parityArtifact(task, commit, status string) faultline.EvidenceArtifact {
	ev := faultline.ParityEvidence{ScenarioID: "s1", Dimension: "d1", Surfaces: []string{"cli"}, DivergentFields: []string{}}
	if status == faultline.StatusFail {
		ev.DivergentFields = []string{"title"}
	}
	raw, _ := json.Marshal(ev)
	return faultline.EvidenceArtifact{
		SchemaVersion: faultline.EvidenceSchemaVersion, ProducingTask: task, ProducingCommit: commit,
		NodeFamily: faultline.NodeFamilyParity, Applicability: []string{"cli"},
		ValidatorIdentity: parityVal, VerifiedEvidence: raw, Status: status,
	}
}

func rec(t *testing.T, node string, a faultline.EvidenceArtifact) EvidenceRecord {
	t.Helper()
	r, err := NewEvidenceRecord(ship(), node, a)
	require.NoError(t, err)
	return r
}

func goodRec(t *testing.T, node string) EvidenceRecord {
	return rec(t, node, parityArtifact(testTaskA, testHead, faultline.StatusPass))
}

func mustEval(t *testing.T, d *Declaration) *Evaluator {
	t.Helper()
	e, err := NewEvaluator(d)
	require.NoError(t, err)
	return e
}

func statusOf(t *testing.T, r Report, id string) Status {
	t.Helper()
	n, ok := r.Node(id)
	require.True(t, ok, "node %s missing", id)
	return n.Status
}

func TestU162_001_LoadYAMLAndJSON(t *testing.T) {
	y, err := Load([]byte(validYAML))
	require.NoError(t, err)
	j, err := Load([]byte(validJSON))
	require.NoError(t, err)
	assert.Equal(t, y, j)
	require.Len(t, y.Nodes, 2)
	assert.Equal(t, FamilyParity, y.Nodes[0].Family)
}

func TestU162_001_LoadRefusals(t *testing.T) {
	big := strings.Repeat("a", MaxDeclarationBytes+1)
	var many strings.Builder
	many.WriteString("schema_version: 1\nnodes:\n")
	for i := 0; i < MaxNodes+1; i++ {
		fmt.Fprintf(&many, "  - {id: n%d, family: parity, applicability: {always: true}, validator: v, producing_task: 162.001-T, producing_commit: head, on_unmet: block, terminal: true}\n", i)
	}
	cases := []struct {
		name string
		in   string
		is   error
	}{
		{"unsupported version", strings.Replace(validYAML, "schema_version: 1", "schema_version: 2", 1), ErrUnsupportedVersion},
		{"missing version", strings.Replace(validYAML, "schema_version: 1\n", "", 1), ErrUnsupportedVersion},
		{"unknown field yaml", validYAML + "extra: 1\n", ErrInvalidDeclaration},
		{"unknown field json", strings.Replace(validJSON, `"terminal":true`, `"terminal":true,"bogus":1`, 1), ErrInvalidDeclaration},
		{"duplicate json key", strings.Replace(validJSON, `"schema_version":1`, `"schema_version":1,"schema_version":1`, 1), ErrInvalidDeclaration},
		{"oversize", big, ErrInvalidDeclaration},
		{"too many nodes", many.String(), ErrInvalidDeclaration},
		{"empty", "", ErrInvalidDeclaration},
		{"trailing json", validJSON + `{}`, ErrInvalidDeclaration},
		{"unknown family", strings.Replace(validYAML, "family: parity", "family: bogus", 1), ErrInvalidDeclaration},
		{"missing applicability", strings.Replace(validYAML, "    applicability: {always: true}\n", "", 1), ErrInvalidDeclaration},
		{"bad on_unmet", strings.Replace(validYAML, "on_unmet: block", "on_unmet: maybe", 1), ErrInvalidDeclaration},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, err := Load([]byte(tc.in))
			require.Error(t, err)
			assert.Nil(t, d)
			assert.ErrorIs(t, err, tc.is)
		})
	}
}

func TestU162_001_ValidateStructure(t *testing.T) {
	cases := []struct {
		name string
		d    *Declaration
		want string
	}{
		{"duplicate id", decl([]Node{pnode("a", false), pnode("a", true)}), "duplicate_id"},
		{"dangling edge", decl([]Node{pnode("a", true)}, Edge{"a", "ghost"}), "dangling_edge"},
		{"dangling source", decl([]Node{pnode("a", true)}, Edge{"ghost", "a"}), "dangling_edge"},
		{"self edge", decl([]Node{pnode("a", true)}, Edge{"a", "a"}), "self_edge"},
		{"duplicate edge", decl([]Node{pnode("a", false), pnode("b", true)}, Edge{"b", "a"}, Edge{"b", "a"}), "duplicate_edge"},
		{"cycle", decl([]Node{pnode("a", false), pnode("b", false), pnode("c", true)}, Edge{"a", "b"}, Edge{"b", "a"}, Edge{"c", "a"}), "cycle"},
		{"no terminal", decl([]Node{pnode("a", false)}), "no_terminal"},
		{"unreachable", decl([]Node{pnode("a", false), pnode("t", true)}), "unreachable_node"},
		{"bad id", decl([]Node{pnode("_a", true)}), "bad_id"},
		{"unknown family", decl([]Node{func() Node { n := pnode("a", true); n.Family = "x"; return n }()}), "unknown_family"},
		{"empty nodes", decl(nil), "no_terminal"},
		{"both predicate", decl([]Node{func() Node {
			n := pnode("a", true)
			n.Applicability = &Applicability{Always: true, Requires: map[string][]string{"k": {"v"}}}
			return n
		}()}), "applicability"},
		{"empty predicate", decl([]Node{func() Node { n := pnode("a", true); n.Applicability = &Applicability{}; return n }()}), "applicability"},
		{"bad task", decl([]Node{func() Node { n := pnode("a", true); n.ProducingTask = "nope"; return n }()}), "bad_task"},
		{"bad commit", decl([]Node{func() Node { n := pnode("a", true); n.ProducingCommit = "abc"; return n }()}), "bad_commit"},
		{"required evidence mismatch", decl([]Node{func() Node { n := pnode("a", true); n.RequiredEvidence = "red_green"; return n }()}), "evidence_family"},
		{"unknown waiver family", decl([]Node{func() Node { n := pnode("a", true); n.WaiverFamily = "zzz"; return n }()}), "unknown_family"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.d.Validate()
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrInvalidDeclaration)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
	require.NoError(t, decl([]Node{pnode("a", false), pnode("t", true)}, Edge{"t", "a"}).Validate())
}

func TestU162_001_CyclePathAndDeterminism(t *testing.T) {
	d := decl([]Node{pnode("c", false), pnode("a", false), pnode("b", false), pnode("t", true)},
		Edge{"a", "b"}, Edge{"b", "c"}, Edge{"c", "a"}, Edge{"t", "a"})
	err1 := d.Validate()
	require.Error(t, err1)
	assert.Contains(t, err1.Error(), "a -> b -> c -> a")
	for i := 0; i < 5; i++ {
		assert.Equal(t, err1.Error(), d.Validate().Error())
	}
	// Edge/node declaration order must not change the diagnostics.
	d2 := decl([]Node{pnode("t", true), pnode("b", false), pnode("a", false), pnode("c", false)},
		Edge{"t", "a"}, Edge{"c", "a"}, Edge{"b", "c"}, Edge{"a", "b"})
	assert.Equal(t, err1.Error(), d2.Validate().Error())
}

func TestU162_001_EvaluateVerifiedAndUnmet(t *testing.T) {
	d := decl([]Node{pnode("base", false), pnode("term", true)}, Edge{"term", "base"})
	e := mustEval(t, d)
	rep, err := e.Evaluate(Input{Shipment: ship(), Evidence: map[string]EvidenceRecord{"base": goodRec(t, "base"), "term": goodRec(t, "term")}})
	require.NoError(t, err)
	assert.Equal(t, StatusVerified, statusOf(t, rep, "base"))
	assert.Equal(t, StatusVerified, statusOf(t, rep, "term"))
	assert.Equal(t, RollupReady, rep.Rollup().State)

	rep, err = mustEval(t, d).Evaluate(Input{Shipment: ship(), Evidence: map[string]EvidenceRecord{"base": goodRec(t, "base")}})
	require.NoError(t, err)
	assert.Equal(t, StatusUnmet, statusOf(t, rep, "term"))
	assert.Equal(t, RollupBlocked, rep.Rollup().State)
	assert.Equal(t, []string{"term"}, rep.Rollup().Unmet)

	failing := rec(t, "term", parityArtifact(testTaskA, testHead, faultline.StatusFail))
	rep, err = mustEval(t, d).Evaluate(Input{Shipment: ship(), Evidence: map[string]EvidenceRecord{"base": goodRec(t, "base"), "term": failing}})
	require.NoError(t, err)
	assert.Equal(t, StatusUnmet, statusOf(t, rep, "term"))
}

func TestU162_001_UpstreamPropagationIsTransitive(t *testing.T) {
	d := decl([]Node{pnode("a", false), pnode("b", false), pnode("c", true)}, Edge{"b", "a"}, Edge{"c", "b"})
	ev := map[string]EvidenceRecord{"b": goodRec(t, "b"), "c": goodRec(t, "c")}
	rep, err := mustEval(t, d).Evaluate(Input{Shipment: ship(), Evidence: ev})
	require.NoError(t, err)
	assert.Equal(t, StatusUnmet, statusOf(t, rep, "a"))
	assert.Equal(t, StatusUnmet, statusOf(t, rep, "b"), "own evidence must not mask an unmet upstream")
	assert.Equal(t, StatusUnmet, statusOf(t, rep, "c"), "unmet must propagate transitively")
	assert.NotEqual(t, RollupReady, rep.Rollup().State)

	ev["a"] = EvidenceRecord{SchemaVersion: 1, ShipmentID: "144-S", NodeID: "a", HeadCommit: testHead, Artifact: json.RawMessage(`{"schema_version":1}`)}
	rep, err = mustEval(t, d).Evaluate(Input{Shipment: ship(), Evidence: ev})
	require.NoError(t, err)
	assert.Equal(t, StatusError, statusOf(t, rep, "a"))
	assert.Equal(t, StatusError, statusOf(t, rep, "c"), "upstream error propagates as error")
	assert.Equal(t, RollupUnverifiable, rep.Rollup().State)
}

func TestU162_001_ApplicabilityResolvedPerShipment(t *testing.T) {
	n := pnode("mut", true)
	n.Applicability = &Applicability{Requires: map[string][]string{"surface": {"mcp", "cli"}}}
	other := pnode("par", true)
	d := decl([]Node{n, other})
	ev := map[string]EvidenceRecord{"par": goodRec(t, "par")}

	s := ship()
	rep, err := mustEval(t, d).Evaluate(Input{Shipment: s, Evidence: ev})
	require.NoError(t, err)
	assert.Equal(t, StatusUnmet, statusOf(t, rep, "mut"))

	s.Attributes = map[string]string{"surface": "docs"}
	rep, err = mustEval(t, d).Evaluate(Input{Shipment: s, Evidence: ev})
	require.NoError(t, err)
	assert.Equal(t, StatusNotApplicable, statusOf(t, rep, "mut"))
	assert.Equal(t, RollupReady, rep.Rollup().State)

	s.Attributes = nil
	rep, err = mustEval(t, d).Evaluate(Input{Shipment: s, Evidence: ev})
	require.NoError(t, err)
	assert.Equal(t, StatusError, statusOf(t, rep, "mut"), "missing attribute is unverifiable, never not-applicable")
	assert.Equal(t, RollupUnverifiable, rep.Rollup().State)

	_, err = ResolveApplicability(n, Shipment{ID: "144-S", HeadCommit: testHead, Attributes: map[string]string{"surface": "mcp"}})
	require.NoError(t, err)
}

func TestU162_001_EmptyApplicableTerminalSetIsNotSatisfied(t *testing.T) {
	n := pnode("t", true)
	n.Applicability = &Applicability{Requires: map[string][]string{"surface": {"mcp"}}}
	rep, err := mustEval(t, decl([]Node{n})).Evaluate(Input{Shipment: ship()})
	require.NoError(t, err)
	assert.Equal(t, StatusNotApplicable, statusOf(t, rep, "t"))
	ru := rep.Rollup()
	assert.Equal(t, RollupUnverifiable, ru.State)
	assert.Zero(t, ru.ApplicableTerminals)

	assert.Equal(t, RollupUnverifiable, Report{}.Rollup().State, "zero-value report is never ready")
}

func TestU162_001_NilAndEmptyEvidenceMapsAreEquivalent(t *testing.T) {
	d := decl([]Node{pnode("t", true)})
	r1, err := mustEval(t, d).Evaluate(Input{Shipment: ship(), Evidence: nil})
	require.NoError(t, err)
	r2, err := mustEval(t, d).Evaluate(Input{Shipment: ship(), Evidence: map[string]EvidenceRecord{}})
	require.NoError(t, err)
	assert.Equal(t, StatusUnmet, statusOf(t, r1, "t"))
	assert.Equal(t, r1.Nodes, r2.Nodes)
}

func TestU162_001_NonConformantEvidenceNeverVerifies(t *testing.T) {
	d := decl([]Node{pnode("t", true)})
	good := goodRec(t, "t")

	garbage := good
	garbage.Artifact = json.RawMessage(`{"schema_version":1}`)
	empty := good
	empty.Artifact = nil
	unknownFamily := good
	unknownFamily.Artifact = json.RawMessage(strings.Replace(string(good.Artifact), `"node_family":"parity"`, `"node_family":"mutation"`, 1))
	extra := good
	extra.Artifact = json.RawMessage(strings.Replace(string(good.Artifact), `"status":"pass"`, `"status":"pass","bonus":1`, 1))
	for name, r := range map[string]EvidenceRecord{"garbage": garbage, "empty": empty, "unknown family": unknownFamily, "extra field": extra} {
		t.Run(name, func(t *testing.T) {
			rep, err := mustEval(t, d).Evaluate(Input{Shipment: ship(), Evidence: map[string]EvidenceRecord{"t": r}})
			require.NoError(t, err)
			assert.Equal(t, StatusError, statusOf(t, rep, "t"))
			assert.NotEqual(t, RollupReady, rep.Rollup().State)
		})
	}
}

func TestU162_001_FamilyWithoutEvidenceContractIsUnverifiable(t *testing.T) {
	n := pnode("m", true)
	n.Family = FamilyMutationState
	rep, err := mustEval(t, decl([]Node{n})).Evaluate(Input{Shipment: ship(), Evidence: map[string]EvidenceRecord{"m": goodRec(t, "m")}})
	require.NoError(t, err)
	assert.Equal(t, StatusError, statusOf(t, rep, "m"))
	assert.Equal(t, RollupUnverifiable, rep.Rollup().State)
}

func TestU162_001_ReportOnlyTerminalDoesNotBlockButIsAdvisory(t *testing.T) {
	n := pnode("adv", true)
	n.OnUnmet = OnUnmetReport
	d := decl([]Node{n, pnode("req", true)})
	rep, err := mustEval(t, d).Evaluate(Input{Shipment: ship(), Evidence: map[string]EvidenceRecord{"req": goodRec(t, "req")}})
	require.NoError(t, err)
	ru := rep.Rollup()
	assert.Equal(t, RollupReady, ru.State)
	assert.Equal(t, []string{"adv"}, ru.Advisory)
}

func TestU162_001_WaiversMustBeVerified(t *testing.T) {
	d := decl([]Node{pnode("t", true)})
	forged := Waiver{ID: "w1", ShipmentID: "144-S", NodeID: "t", HeadCommit: testHead, Principal: "mallory"}
	rep, err := mustEval(t, d).Evaluate(Input{Shipment: ship(), Waivers: map[string]Waiver{"t": forged}})
	require.NoError(t, err)
	assert.Equal(t, StatusError, statusOf(t, rep, "t"))

	ok := forged
	ok.verified = true
	rep, err = mustEval(t, d).Evaluate(Input{Shipment: ship(), Waivers: map[string]Waiver{"t": ok}})
	require.NoError(t, err)
	assert.Equal(t, StatusWaived, statusOf(t, rep, "t"))
	assert.Equal(t, RollupReady, rep.Rollup().State)
	n, _ := rep.Node("t")
	assert.Equal(t, "mallory", n.Waiver)

	wrong := ok
	wrong.HeadCommit = otherHead
	rep, err = mustEval(t, d).Evaluate(Input{Shipment: ship(), Waivers: map[string]Waiver{"t": wrong}})
	require.NoError(t, err)
	assert.Equal(t, StatusError, statusOf(t, rep, "t"))
}

func TestU162_001_IncrementalEvaluationRecomputesOnlyAffected(t *testing.T) {
	d := decl([]Node{pnode("a", false), pnode("b", false), pnode("t", true), pnode("solo", true)}, Edge{"b", "a"}, Edge{"t", "b"})
	e := mustEval(t, d)
	ev := map[string]EvidenceRecord{"a": goodRec(t, "a"), "b": goodRec(t, "b"), "t": goodRec(t, "t"), "solo": goodRec(t, "solo")}
	r1, err := e.Evaluate(Input{Shipment: ship(), Evidence: ev})
	require.NoError(t, err)
	assert.Equal(t, 4, r1.Computed)
	assert.Equal(t, RollupReady, r1.Rollup().State)

	r2, err := e.Evaluate(Input{Shipment: ship(), Evidence: ev})
	require.NoError(t, err)
	assert.Equal(t, 0, r2.Computed)
	assert.Equal(t, 4, r2.CacheHits)
	assert.Equal(t, r1.Nodes, r2.Nodes)

	// Changing upstream evidence must invalidate every transitive dependent.
	ev["a"] = rec(t, "a", parityArtifact(testTaskA, testHead, faultline.StatusFail))
	r3, err := e.Evaluate(Input{Shipment: ship(), Evidence: ev})
	require.NoError(t, err)
	assert.Equal(t, 3, r3.Computed, "a, b, t recomputed; solo cached")
	assert.Equal(t, 1, r3.CacheHits)
	assert.Equal(t, StatusUnmet, statusOf(t, r3, "t"), "stale cached verified must not survive an upstream change")
	assert.Equal(t, StatusVerified, statusOf(t, r3, "solo"))

	// A fresh evaluator must agree with the incremental one.
	fresh, err := mustEval(t, d).Evaluate(Input{Shipment: ship(), Evidence: ev})
	require.NoError(t, err)
	assert.Equal(t, fresh.Nodes, r3.Nodes)

	// Restoring the evidence returns to verified.
	ev["a"] = goodRec(t, "a")
	r4, err := e.Evaluate(Input{Shipment: ship(), Evidence: ev})
	require.NoError(t, err)
	assert.Equal(t, StatusVerified, statusOf(t, r4, "t"))

	// A different head commit or shipment is a different key.
	s := ship()
	s.HeadCommit = otherHead
	r5, err := e.Evaluate(Input{Shipment: s, Evidence: ev})
	require.NoError(t, err)
	assert.Equal(t, 4, r5.Computed)
}

func TestU162_001_ReportIsOrderedAndInputValidated(t *testing.T) {
	d := decl([]Node{pnode("z", true), pnode("m", true), pnode("a", true)})
	rep, err := mustEval(t, d).Evaluate(Input{Shipment: ship()})
	require.NoError(t, err)
	ids := []string{}
	for _, n := range rep.Nodes {
		ids = append(ids, n.ID)
	}
	assert.Equal(t, []string{"a", "m", "z"}, ids)

	e := mustEval(t, d)
	for name, s := range map[string]Shipment{
		"empty id": {HeadCommit: testHead},
		"bad head": {ID: "144-S", HeadCommit: "abc"},
		"too many attrs": {ID: "144-S", HeadCommit: testHead, Attributes: func() map[string]string {
			m := map[string]string{}
			for i := 0; i <= MaxAttributes; i++ {
				m[fmt.Sprint("k", i)] = "v"
			}
			return m
		}()},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := e.Evaluate(Input{Shipment: s})
			assert.ErrorIs(t, err, ErrInvalidInput)
		})
	}
}

func TestU162_001_NewEvaluatorRejectsInvalidDeclaration(t *testing.T) {
	_, err := NewEvaluator(decl([]Node{pnode("a", false)}))
	assert.ErrorIs(t, err, ErrInvalidDeclaration)
	_, err = NewEvaluator(nil)
	assert.ErrorIs(t, err, ErrInvalidDeclaration)
}
