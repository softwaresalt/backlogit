package policyengine

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/evidence"
	"github.com/softwaresalt/backlogit/internal/evidencedag"
	"github.com/softwaresalt/backlogit/internal/redhonesty"
)

// shipmentRequest builds a request for the given tasks, each with the
// satisfied evidence set mutated by mut (when non-nil for that task).
func shipmentRequest(e *env, shipment string, tasks []string, mut map[string]func(map[NodeType]Record)) ShipmentRequest {
	req := ShipmentRequest{Surface: SurfaceCLI, ShipmentID: shipment, Manifest: tasks}
	defer func(old string) { e.task = old }(e.task)
	for _, id := range tasks {
		e.task = id
		recs := e.good()
		if m := mut[id]; m != nil {
			m(recs)
		}
		req.Tasks = append(req.Tasks, e.request(recs))
	}
	return req
}

func checkShipment(t *testing.T, eng *Engine, req ShipmentRequest) ShipmentDecision {
	t.Helper()
	d, err := eng.CheckShipment(context.Background(), req)
	require.NoError(t, err)
	return d
}

func TestU163_004_FullySatisfiedShipmentProceeds(t *testing.T) {
	e := newEnv(t, enforceRegistry)
	d := checkShipment(t, e.engine(), shipmentRequest(e, "145-S", []string{"163.002-T", "163.001-T"}, nil))
	assert.Equal(t, DispositionPass, d.Disposition, "%+v", d.Errors)
	assert.True(t, d.Allowed)
	assert.True(t, d.ReviewReady)
	assert.Equal(t, evidencedag.RollupReady, d.Rollup.State)
	assert.Equal(t, 2, d.Rollup.ApplicableTerminals)
	require.Len(t, d.Tasks, 2)
	assert.Equal(t, "163.001-T", d.Tasks[0].TaskID, "tasks must be ordered by id")
	assert.Equal(t, "163.002-T", d.Tasks[1].TaskID)
}

func TestU163_004_ShipmentWithAnUnsatisfiedTaskGateCannotBecomeReviewReady(t *testing.T) {
	e := newEnv(t, enforceRegistry)
	for _, gate := range []NodeType{NodeHarnessScaffold, NodeRedPhaseVerify, NodeGreenPhaseVerify, NodeMergeReady} {
		t.Run(string(gate), func(t *testing.T) {
			g := gate
			req := shipmentRequest(e, "145-S", []string{"163.001-T", "163.002-T"}, map[string]func(map[NodeType]Record){
				"163.002-T": func(m map[NodeType]Record) { delete(m, g) },
			})
			d := checkShipment(t, e.engine(), req)
			assert.Equal(t, DispositionBlock, d.Disposition)
			assert.False(t, d.Allowed)
			assert.False(t, d.ReviewReady)
			assert.Equal(t, evidencedag.RollupBlocked, d.Rollup.State)
			assert.Equal(t, []string{"163.002-T"}, d.Rollup.Unmet)
			assert.True(t, d.Tasks[0].Allowed, "the satisfied task stays satisfied")
			assert.False(t, d.Tasks[1].Allowed)
			assert.True(t, hasViolation(d.Tasks[1], g, CodeEvidenceAbsent))
		})
	}
}

func TestU163_004_DerivedGateFailureBlocksTheShipment(t *testing.T) {
	e := newEnv(t, enforceRegistry)
	e.deriv.ci.AllPassed = false
	d := checkShipment(t, e.engine(), shipmentRequest(e, "145-S", []string{"163.001-T"}, nil))
	assert.False(t, d.Allowed)
	assert.False(t, d.ReviewReady)
}

func TestU163_004_UnverifiableTaskMakesTheShipmentUnverifiable(t *testing.T) {
	e := newEnv(t, enforceRegistry)
	req := shipmentRequest(e, "145-S", []string{"163.001-T"}, map[string]func(map[NodeType]Record){
		"163.001-T": func(m map[NodeType]Record) {
			m[NodeGreenPhaseVerify] = e.rec(NodeGreenPhaseVerify, sha(999), e.at(3, 30), OutcomePass, nil)
		},
	})
	d := checkShipment(t, e.engine(), req)
	assert.False(t, d.Allowed)
	assert.Equal(t, evidencedag.RollupUnverifiable, d.Rollup.State)
	assert.Equal(t, []string{"163.001-T"}, d.Rollup.Unverifiable)
}

func TestU163_004_MissingConstituentTaskBlocks(t *testing.T) {
	e := newEnv(t, enforceRegistry)
	req := shipmentRequest(e, "145-S", []string{"163.001-T", "163.002-T"}, nil)
	req.Tasks = req.Tasks[:1]
	d := checkShipment(t, e.engine(), req)
	assert.False(t, d.Allowed)
	assert.False(t, d.ReviewReady)
	found := false
	for _, v := range d.Errors {
		if v.Code == CodeTaskMissing && v.Node == "163.002-T" {
			found = true
		}
	}
	assert.True(t, found, "%+v", d.Errors)
}

func TestU163_004_ShipmentRequestValidationFailsClosed(t *testing.T) {
	e := newEnv(t, canonicalRegistryYAML) // report-only: malformed input must still block
	eng := e.engine()
	base := func() ShipmentRequest { return shipmentRequest(e, "145-S", []string{"163.001-T", "163.002-T"}, nil) }
	bad := map[string]func(*ShipmentRequest){
		"empty manifest":      func(r *ShipmentRequest) { r.Manifest = []string{} },
		"nil manifest":        func(r *ShipmentRequest) { r.Manifest = nil },
		"bad shipment id":     func(r *ShipmentRequest) { r.ShipmentID = "../x" },
		"bad manifest id":     func(r *ShipmentRequest) { r.Manifest[0] = "nope" },
		"duplicate manifest":  func(r *ShipmentRequest) { r.Manifest[1] = r.Manifest[0] },
		"unexpected task":     func(r *ShipmentRequest) { r.Tasks[1].TaskID = "163.009-T" },
		"duplicate task":      func(r *ShipmentRequest) { r.Tasks[1].TaskID = r.Tasks[0].TaskID },
		"unknown surface":     func(r *ShipmentRequest) { r.Surface = "bot" },
		"too many tasks":      func(r *ShipmentRequest) { r.Manifest = make([]string, MaxShipmentTasks+1) },
		"task bad head":       func(r *ShipmentRequest) { r.Tasks[0].HeadCommit = "main" },
		"task unknown record": func(r *ShipmentRequest) { r.Tasks[0].Records = append(r.Tasks[0].Records, Record{Node: "deploy"}) },
	}
	for name, mut := range bad {
		t.Run(name, func(t *testing.T) {
			req := base()
			mut(&req)
			d, err := eng.CheckShipment(context.Background(), req)
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrInvalidRequest)
			assert.False(t, d.Allowed)
			assert.False(t, d.ReviewReady)
			assert.Equal(t, DispositionBlock, d.Disposition)
		})
	}
	t.Run("cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		d, err := eng.CheckShipment(ctx, base())
		require.Error(t, err)
		assert.False(t, d.Allowed)
	})
	t.Run("nil engine", func(t *testing.T) {
		var nilEng *Engine
		d, err := nilEng.CheckShipment(context.Background(), base())
		require.Error(t, err)
		assert.False(t, d.Allowed)
	})
}

func TestU163_004_ReportOnlyShipmentIsAllowedButNeverReviewReady(t *testing.T) {
	e := newEnv(t, canonicalRegistryYAML)
	req := shipmentRequest(e, "145-S", []string{"163.001-T"}, map[string]func(map[NodeType]Record){
		"163.001-T": func(m map[NodeType]Record) { delete(m, NodeHarnessScaffold) },
	})
	d := checkShipment(t, e.engine(), req)
	assert.Equal(t, DispositionReportOnly, d.Disposition)
	assert.True(t, d.Allowed)
	assert.False(t, d.ReviewReady, "the truthful roll-up verdict must survive report-only")
	assert.NotEqual(t, evidencedag.RollupReady, d.Rollup.State)
}

func TestU163_004_ShipmentParityAcrossSurfaces(t *testing.T) {
	e := newEnv(t, enforceRegistry)
	eng := e.engine()
	var keys []string
	for _, s := range Surfaces() {
		req := shipmentRequest(e, "145-S", []string{"163.001-T", "163.002-T"}, map[string]func(map[NodeType]Record){
			"163.002-T": func(m map[NodeType]Record) { delete(m, NodeRedPhaseVerify) },
		})
		req.Surface = s
		d := checkShipment(t, eng, req)
		assert.Equal(t, s, d.Surface)
		for i := range d.Tasks {
			d.Tasks[i].Surface = ""
		}
		d.Surface = ""
		b, err := json.Marshal(d)
		require.NoError(t, err)
		keys = append(keys, string(b))
	}
	for _, k := range keys[1:] {
		assert.JSONEq(t, keys[0], k)
	}
}

func TestU163_004_ShipmentDecisionIsDeterministic(t *testing.T) {
	e := newEnv(t, enforceRegistry)
	eng := e.engine()
	req := shipmentRequest(e, "145-S", []string{"163.002-T", "163.001-T"}, nil)
	a, _ := json.Marshal(checkShipment(t, eng, req))
	b, _ := json.Marshal(checkShipment(t, eng, req))
	assert.JSONEq(t, string(a), string(b))
}

func TestU163_004_PolicyVerdictJoinsTheS10TerminalNodeVerification(t *testing.T) {
	e := newEnv(t, enforceRegistry)
	eng := e.engine()
	base := &evidencedag.Declaration{
		SchemaVersion: evidencedag.SchemaVersion,
		Nodes: []evidencedag.Node{{
			ID: "base-gate", Family: evidencedag.FamilyWorkflowPolicy, Applicability: &evidencedag.Applicability{Always: true},
			Validator: "base@v1", ProducingTask: "163.004-T", ProducingCommit: evidencedag.CommitHead,
			OnUnmet: evidencedag.OnUnmetBlock, Terminal: true,
		}},
	}
	merged, err := evidencedag.Compose(base, Contribution("163.004-T"))
	require.NoError(t, err)
	require.Len(t, merged.Nodes, 2)
	var policyNode evidencedag.Node
	for _, n := range merged.Nodes {
		if n.ID == ShipmentPolicyNodeID {
			policyNode = n
		}
	}
	assert.Equal(t, "workflow-policy", policyNode.Source)
	assert.True(t, policyNode.Terminal)
	assert.Equal(t, evidencedag.FamilyWorkflowPolicy, policyNode.Family)

	gate, err := evidencedag.NewGate(merged, evidencedag.ModeEnforce, nil)
	require.NoError(t, err)
	ship := evidencedag.Shipment{ID: "145-S", HeadCommit: e.c[4]}

	ready := checkShipment(t, eng, shipmentRequest(e, "145-S", []string{"163.001-T"}, nil))
	dec, err := gate.Check(context.Background(), evidencedag.GateInput{Shipment: ship, Verdicts: map[string]evidencedag.Verdict{
		"base-gate": {Status: evidencedag.StatusVerified}, ShipmentPolicyNodeID: ready.Verdict(),
	}})
	require.NoError(t, err)
	assert.True(t, dec.ReviewReady, "%+v", dec.Rollup)

	unready := checkShipment(t, eng, shipmentRequest(e, "145-S", []string{"163.001-T"}, map[string]func(map[NodeType]Record){
		"163.001-T": func(m map[NodeType]Record) { delete(m, NodeHarnessScaffold) },
	}))
	dec, err = gate.Check(context.Background(), evidencedag.GateInput{Shipment: ship, Verdicts: map[string]evidencedag.Verdict{
		"base-gate": {Status: evidencedag.StatusVerified}, ShipmentPolicyNodeID: unready.Verdict(),
	}})
	require.NoError(t, err)
	assert.False(t, dec.ReviewReady, "an unsatisfied policy roll-up must block the authoritative review-ready gate")
	assert.False(t, dec.Allowed)
	assert.Contains(t, dec.Rollup.Unmet, ShipmentPolicyNodeID)
}

func TestU163_004_VerdictMapsEveryRollupState(t *testing.T) {
	cases := map[evidencedag.RollupState]evidencedag.Status{
		evidencedag.RollupReady:        evidencedag.StatusVerified,
		evidencedag.RollupBlocked:      evidencedag.StatusUnmet,
		evidencedag.RollupUnverifiable: evidencedag.StatusError,
		evidencedag.RollupUnproven:     evidencedag.StatusUnmet,
		"":                             evidencedag.StatusError,
	}
	for state, want := range cases {
		d := ShipmentDecision{Rollup: evidencedag.Rollup{State: state}, ReviewReady: state == evidencedag.RollupReady}
		assert.Equal(t, want, d.Verdict().Status, string(state))
	}
	// A ready flag without a ready roll-up must not verify.
	assert.NotEqual(t, evidencedag.StatusVerified, ShipmentDecision{ReviewReady: true}.Verdict().Status)
}

type incidentFixture struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Mode        string `json:"mode"`
	TaskID      string `json:"task_id"`
	Commits     []struct {
		Name    string   `json:"name"`
		AtS     int      `json:"at_s"`
		Parents []string `json:"parents"`
	} `json:"commits"`
	Head    string `json:"head"`
	Records []struct {
		Node        NodeType `json:"node"`
		Commit      string   `json:"commit"`
		AtS         int      `json:"at_s"`
		Outcome     string   `json:"outcome"`
		RedArtifact bool     `json:"red_artifact"`
	} `json:"records"`
	Expect struct {
		Disposition Disposition `json:"disposition"`
		Violations  []struct {
			Node string `json:"node"`
			Code string `json:"code"`
		} `json:"violations"`
	} `json:"expect"`
}

// replay builds the fixture's repository and evidence and returns the request
// and an engine that has no knowledge of the expected outcome.
func replay(t *testing.T, fx incidentFixture) (*Engine, Request) {
	t.Helper()
	reg := enforceRegistry
	if fx.Mode == "report-only" {
		reg = canonicalRegistryYAML
	}
	registry, err := LoadRegistry([]byte(reg))
	require.NoError(t, err)
	t0 := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	mem := NewMemGit()
	shaOf := map[string]string{}
	for i, c := range fx.Commits {
		shaOf[c.Name] = sha(1000 + i)
		var parents []string
		for _, p := range c.Parents {
			require.Contains(t, shaOf, p, "fixture %s: parent %s must be declared first", fx.ID, p)
			parents = append(parents, shaOf[p])
		}
		mem.AddCommit(shaOf[c.Name], t0.Add(time.Duration(c.AtS)*time.Second), parents...)
	}
	head := shaOf[fx.Head]
	require.NotEmpty(t, head)
	mem.SetRef("refs/remotes/origin/feat/x", head)
	base := shaOf[fx.Commits[0].Name]

	var recs []Record
	for _, r := range fx.Records {
		var art json.RawMessage
		if r.RedArtifact {
			a, err := redhonesty.BuildArtifact(fx.TaskID, base, shaOf[r.Commit], []redhonesty.Verdict{{
				Test:     redhonesty.NewTest{Package: ".", File: "x_test.go", Name: "TestX"},
				Baseline: redhonesty.ResultFail, Head: redhonesty.ResultPass,
			}})
			require.NoError(t, err)
			art, err = json.Marshal(a)
			require.NoError(t, err)
		}
		rec, err := NewRecord(fx.TaskID, r.Node, shaOf[r.Commit], t0.Add(time.Duration(r.AtS)*time.Second), r.Outcome, art)
		require.NoError(t, err)
		recs = append(recs, rec)
	}
	deriv := &fakeDeriv{
		review: evidence.ReviewResult{Status: evidence.StatusVerified, PR: 7, HeadSHA: head, ReviewedCurrent: true},
		ci:     evidence.CIResult{Status: evidence.StatusVerified, PR: 7, HeadSHA: head, AllPassed: true},
	}
	eng, err := NewEngine(registry, Options{
		Git: mem, Derivation: deriv, ProtectedRefs: []string{"refs/remotes/origin/feat/x"},
		Now: func() time.Time { return t0.Add(time.Hour) },
	})
	require.NoError(t, err)
	return eng, Request{Surface: SurfaceCLI, TaskID: fx.TaskID, HeadCommit: head, PR: 7, Records: recs}
}

func loadIncidents(t *testing.T) []incidentFixture {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join("testdata", "incidents", "*.json"))
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(paths), 3)
	var out []incidentFixture
	for _, p := range paths {
		raw, err := os.ReadFile(p)
		require.NoError(t, err)
		var fx incidentFixture
		require.NoError(t, json.Unmarshal(raw, &fx), p)
		require.NotEmpty(t, fx.Description, p)
		out = append(out, fx)
	}
	return out
}

func TestU163_004_IncidentReplayScenariosBlock(t *testing.T) {
	ids := map[string]bool{}
	for _, fx := range loadIncidents(t) {
		ids[fx.ID] = true
		t.Run(fx.ID, func(t *testing.T) {
			eng, req := replay(t, fx)
			for _, s := range Surfaces() {
				req.Surface = s
				d, err := eng.Check(context.Background(), req)
				require.NoError(t, err)
				assert.Equal(t, fx.Expect.Disposition, d.Disposition, "%s on %s: %+v", fx.ID, s, d.Errors)
				wantAllowed := fx.Expect.Disposition == DispositionPass || fx.Expect.Disposition == DispositionReportOnly
				assert.Equal(t, wantAllowed, d.Allowed, "%s on %s", fx.ID, s)
				if fx.Expect.Disposition != DispositionPass {
					assert.NotEmpty(t, fx.Expect.Violations, "%s: a non-pass fixture must name its violations", fx.ID)
					assert.NotEmpty(t, d.Errors, "%s on %s: a non-pass decision must carry a violation", fx.ID, s)
				}
				for _, want := range fx.Expect.Violations {
					assert.True(t, hasViolation(d, NodeType(want.Node), want.Code), "%s: expected %s/%s in %+v", fx.ID, want.Node, want.Code, d.Errors)
				}
			}
		})
	}
	assert.True(t, ids["INC-P002-131S-148F"], "the 131-S/148-F incident fixture is required")
	assert.True(t, ids["INC-P002-152F-134S"], "the 152-F/134-S incident fixture is required")
}

func TestU163_004_IncidentsBlockShipmentRollUpToo(t *testing.T) {
	for _, fx := range loadIncidents(t) {
		if fx.Expect.Disposition != DispositionBlock {
			continue
		}
		t.Run(fx.ID, func(t *testing.T) {
			eng, req := replay(t, fx)
			d, err := eng.CheckShipment(context.Background(), ShipmentRequest{
				Surface: SurfaceAgent, ShipmentID: "131-S", Manifest: []string{fx.TaskID}, Tasks: []Request{req},
			})
			require.NoError(t, err)
			assert.False(t, d.ReviewReady)
			assert.False(t, d.Allowed)
		})
	}
}
