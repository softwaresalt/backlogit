package evidencedag

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/faultline"
	"github.com/softwaresalt/backlogit/internal/redhonesty"
)

func evalOne(t *testing.T, d *Declaration, ev map[string]EvidenceRecord) Report {
	t.Helper()
	rep, err := mustEval(t, d).Evaluate(Input{Shipment: ship(), Evidence: ev})
	require.NoError(t, err)
	return rep
}

func TestU162_002_PinnedProducingCommit(t *testing.T) {
	n := pnode("t", true)
	n.ProducingCommit = otherHead
	d := decl([]Node{n})
	// Evidence produced at the pinned commit, presented for head testHead.
	r := rec(t, "t", parityArtifact(testTaskA, otherHead, faultline.StatusPass))
	assert.Equal(t, StatusVerified, statusOf(t, evalOne(t, d, map[string]EvidenceRecord{"t": r}), "t"))
	r2 := rec(t, "t", parityArtifact(testTaskA, testHead, faultline.StatusPass))
	assert.Equal(t, StatusError, statusOf(t, evalOne(t, d, map[string]EvidenceRecord{"t": r2}), "t"))
}

func TestU162_002_AuthenticityPrecedesApplicability(t *testing.T) {
	n := pnode("m", true)
	n.Applicability = &Applicability{Requires: map[string][]string{"surface": {"mcp"}}}
	d := decl([]Node{n, pnode("t", true)})
	forged := rec(t, "m", parityArtifact(testTaskB, testHead, faultline.StatusPass))
	rep := evalOne(t, d, map[string]EvidenceRecord{"m": forged, "t": goodRec(t, "t")})
	assert.Equal(t, StatusError, statusOf(t, rep, "m"), "forged evidence on a non-applicable node is an integrity failure, not not-applicable")
	assert.Equal(t, RollupUnverifiable, rep.Rollup().State)

	ok := goodRec(t, "m")
	rep = evalOne(t, d, map[string]EvidenceRecord{"m": ok, "t": goodRec(t, "t")})
	assert.Equal(t, StatusNotApplicable, statusOf(t, rep, "m"))
}

func TestU162_002_DigestReuseAcrossNodesRejected(t *testing.T) {
	d := decl([]Node{pnode("a", true), pnode("b", true)})
	a := goodRec(t, "a")
	b := a
	b.NodeID = "b"
	rep := evalOne(t, d, map[string]EvidenceRecord{"a": a, "b": b})
	assert.Equal(t, StatusError, statusOf(t, rep, "a"))
	assert.Equal(t, StatusError, statusOf(t, rep, "b"))

	distinct := rec(t, "b", func() faultline.EvidenceArtifact {
		x := parityArtifact(testTaskA, testHead, faultline.StatusPass)
		x.VerifiedEvidence = json.RawMessage(`{"scenario_id":"s9","dimension":"d1","surfaces":["cli"],"divergent_fields":[],"expected_divergence":false,"tracked_defect":"","detail":""}`)
		return x
	}())
	rep = evalOne(t, d, map[string]EvidenceRecord{"a": a, "b": distinct})
	assert.Equal(t, RollupReady, rep.Rollup().State)
}

func fnExec(t *testing.T, fn ExecFunc) NodeExecutor {
	t.Helper()
	x, err := NewFuncExecutor(parityVal, fn)
	require.NoError(t, err)
	return x
}

func passFn(task string) ExecFunc {
	return func(_ context.Context, req ExecRequest) (faultline.EvidenceArtifact, error) {
		return parityArtifactFor(req.Node.ID, task, req.Shipment.HeadCommit, faultline.StatusPass), nil
	}
}

func TestU162_002_RunExecutorsProducesVerifiedEvidence(t *testing.T) {
	d := decl([]Node{pnode("base", false), pnode("t", true)}, Edge{"t", "base"})
	calls := 0
	x := fnExec(t, func(ctx context.Context, req ExecRequest) (faultline.EvidenceArtifact, error) {
		calls++
		return passFn(testTaskA)(ctx, req)
	})
	res, err := RunExecutors(context.Background(), d, ship(), map[string]NodeExecutor{"base": x, "t": x})
	require.NoError(t, err)
	assert.Equal(t, 2, calls)
	assert.Empty(t, res.Failures)
	rep, err := mustEval(t, d).Evaluate(Input{Shipment: ship(), Evidence: res.Evidence, Failures: res.Failures})
	require.NoError(t, err)
	assert.Equal(t, RollupReady, rep.Rollup().State)
}

func TestU162_002_TerminalSatisfiedOnlyByVerifiedExecutorEvidence(t *testing.T) {
	d := decl([]Node{pnode("t", true)})
	failing := fnExec(t, func(_ context.Context, req ExecRequest) (faultline.EvidenceArtifact, error) {
		return parityArtifact(testTaskA, req.Shipment.HeadCommit, faultline.StatusFail), nil
	})
	res, err := RunExecutors(context.Background(), d, ship(), map[string]NodeExecutor{"t": failing})
	require.NoError(t, err)
	rep, err := mustEval(t, d).Evaluate(Input{Shipment: ship(), Evidence: res.Evidence, Failures: res.Failures})
	require.NoError(t, err)
	assert.Equal(t, StatusUnmet, statusOf(t, rep, "t"))

	// No executor bound: no evidence, unmet.
	res, err = RunExecutors(context.Background(), d, ship(), nil)
	require.NoError(t, err)
	assert.Empty(t, res.Evidence)
	rep, err = mustEval(t, d).Evaluate(Input{Shipment: ship(), Evidence: res.Evidence, Failures: res.Failures})
	require.NoError(t, err)
	assert.Equal(t, StatusUnmet, statusOf(t, rep, "t"))

	// An executor that mints evidence for another task is rejected by authenticity.
	forger := fnExec(t, passFn(testTaskB))
	res, err = RunExecutors(context.Background(), d, ship(), map[string]NodeExecutor{"t": forger})
	require.NoError(t, err)
	rep, err = mustEval(t, d).Evaluate(Input{Shipment: ship(), Evidence: res.Evidence, Failures: res.Failures})
	require.NoError(t, err)
	assert.Equal(t, StatusError, statusOf(t, rep, "t"))
}

func TestU162_002_ExecutorFailuresFailClosed(t *testing.T) {
	d := decl([]Node{pnode("a", true), pnode("b", true), pnode("c", true), pnode("d", true)})
	bindings := map[string]NodeExecutor{
		"a": fnExec(t, func(context.Context, ExecRequest) (faultline.EvidenceArtifact, error) {
			return faultline.EvidenceArtifact{}, errors.New(strings.Repeat("boom ", 500))
		}),
		"b": fnExec(t, func(context.Context, ExecRequest) (faultline.EvidenceArtifact, error) { panic("kaput") }),
		"c": fnExec(t, func(context.Context, ExecRequest) (faultline.EvidenceArtifact, error) {
			return faultline.EvidenceArtifact{ProducingTask: "x"}, nil
		}),
	}
	wrongID, err := NewFuncExecutor("someone/else@v1", passFn(testTaskA))
	require.NoError(t, err)
	bindings["d"] = wrongID
	res, err := RunExecutors(context.Background(), d, ship(), bindings)
	require.NoError(t, err)
	assert.Empty(t, res.Evidence)
	require.Len(t, res.Failures, 4)
	for id, reason := range res.Failures {
		assert.LessOrEqual(t, len(reason), MaxReasonBytes, id)
	}
	rep, err := mustEval(t, d).Evaluate(Input{Shipment: ship(), Evidence: res.Evidence, Failures: res.Failures})
	require.NoError(t, err)
	for _, id := range []string{"a", "b", "c", "d"} {
		assert.Equal(t, StatusError, statusOf(t, rep, id))
	}
	assert.Equal(t, RollupUnverifiable, rep.Rollup().State)
}

func TestU162_002_RunExecutorsScopeAndCancellation(t *testing.T) {
	n := pnode("m", true)
	n.Applicability = &Applicability{Requires: map[string][]string{"surface": {"mcp"}}}
	d := decl([]Node{n, pnode("t", true)})
	ran := map[string]bool{}
	x := fnExec(t, func(ctx context.Context, req ExecRequest) (faultline.EvidenceArtifact, error) {
		ran[req.Node.ID] = true
		return passFn(testTaskA)(ctx, req)
	})
	res, err := RunExecutors(context.Background(), d, ship(), map[string]NodeExecutor{"m": x, "t": x})
	require.NoError(t, err)
	assert.False(t, ran["m"], "non-applicable nodes are not executed")
	assert.True(t, ran["t"])
	assert.Contains(t, res.Evidence, "t")

	_, err = RunExecutors(context.Background(), d, ship(), map[string]NodeExecutor{"ghost": x})
	assert.ErrorIs(t, err, ErrInvalidInput)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = RunExecutors(ctx, d, ship(), map[string]NodeExecutor{"t": x})
	assert.ErrorIs(t, err, context.Canceled)

	_, err = NewFuncExecutor("", passFn(testTaskA))
	assert.Error(t, err)
	_, err = NewFuncExecutor(parityVal, nil)
	assert.Error(t, err)
}

type fakeChecker struct {
	req redhonesty.Request
	rep redhonesty.Report
	err error
}

func (f *fakeChecker) Check(_ context.Context, req redhonesty.Request) (redhonesty.Report, error) {
	f.req = req
	return f.rep, f.err
}

func rgNode(id string) Node {
	n := pnode(id, true)
	n.Family = FamilyRedTestHonesty
	n.Validator = redhonesty.ValidatorIdentity
	n.ProducingTask = testTaskB
	return n
}

func rgReport(t *testing.T, honest bool) redhonesty.Report {
	t.Helper()
	v := redhonesty.Verdict{
		Test:     redhonesty.NewTest{Package: "internal/x", Name: "TestX"},
		Baseline: redhonesty.ResultFail, Head: redhonesty.ResultPass,
	}
	if !honest {
		v.Baseline = redhonesty.ResultPass
		v.Rejections = []redhonesty.RejectionClass{redhonesty.RejectAlreadyPasses}
	}
	a, err := redhonesty.BuildArtifact(testTaskB, otherHead, testHead, []redhonesty.Verdict{v})
	require.NoError(t, err)
	return redhonesty.Report{BaseSHA: otherHead, HeadSHA: testHead, Artifact: a, Honest: honest}
}

func TestU162_002_RedHonestyAdapter(t *testing.T) {
	d := decl([]Node{rgNode("red")})
	chk := &fakeChecker{rep: rgReport(t, true)}
	x := &RedHonestyExecutor{Gate: chk, BaseRef: "origin/main"}
	assert.Equal(t, redhonesty.ValidatorIdentity, x.Identity())
	res, err := RunExecutors(context.Background(), d, ship(), map[string]NodeExecutor{"red": x})
	require.NoError(t, err)
	assert.Equal(t, redhonesty.Request{TaskID: testTaskB, BaseRef: "origin/main", HeadRef: testHead}, chk.req)
	rep, err := mustEval(t, d).Evaluate(Input{Shipment: ship(), Evidence: res.Evidence, Failures: res.Failures})
	require.NoError(t, err)
	assert.Equal(t, StatusVerified, statusOf(t, rep, "red"))

	chk.rep = rgReport(t, false)
	res, err = RunExecutors(context.Background(), d, ship(), map[string]NodeExecutor{"red": x})
	require.NoError(t, err)
	rep, err = mustEval(t, d).Evaluate(Input{Shipment: ship(), Evidence: res.Evidence, Failures: res.Failures})
	require.NoError(t, err)
	assert.Equal(t, StatusUnmet, statusOf(t, rep, "red"), "dishonest red test must not satisfy the node")

	chk.err = errors.New("git exploded")
	res, err = RunExecutors(context.Background(), d, ship(), map[string]NodeExecutor{"red": x})
	require.NoError(t, err)
	assert.Contains(t, res.Failures, "red")

	nilGate := &RedHonestyExecutor{}
	_, err = nilGate.Execute(context.Background(), ExecRequest{Shipment: ship(), Node: rgNode("red")})
	assert.Error(t, err)
}

func TestU162_002_ParityAdapter(t *testing.T) {
	d := decl([]Node{pnode("par", true)})
	x := &ParityExecutor{Source: func(context.Context, ExecRequest) (ParityResult, error) {
		return ParityResult{Evidence: faultline.ParityEvidence{ScenarioID: "s1", Dimension: "d", Surfaces: []string{"cli", "mcp"}, DivergentFields: []string{}}, Status: faultline.StatusPass}, nil
	}}
	assert.Equal(t, ParityValidatorIdentity, x.Identity())
	res, err := RunExecutors(context.Background(), d, ship(), map[string]NodeExecutor{"par": x})
	require.NoError(t, err)
	rep, err := mustEval(t, d).Evaluate(Input{Shipment: ship(), Evidence: res.Evidence, Failures: res.Failures})
	require.NoError(t, err)
	assert.Equal(t, StatusVerified, statusOf(t, rep, "par"))

	bad := &ParityExecutor{Source: func(context.Context, ExecRequest) (ParityResult, error) {
		return ParityResult{Evidence: faultline.ParityEvidence{ScenarioID: "s1", Dimension: "d", Surfaces: []string{"cli"}, DivergentFields: []string{"x"}}, Status: faultline.StatusPass}, nil
	}}
	res, err = RunExecutors(context.Background(), d, ship(), map[string]NodeExecutor{"par": bad})
	require.NoError(t, err)
	assert.Contains(t, res.Failures, "par", "non-conformant parity payload is refused at the contract")

	_, err = (&ParityExecutor{}).Execute(context.Background(), ExecRequest{Shipment: ship(), Node: pnode("par", true)})
	assert.Error(t, err)
}

func writeRecord(t *testing.T, path string, r EvidenceRecord) {
	t.Helper()
	b, err := json.Marshal(r)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, b, 0o644))
}

func TestU162_002_LoadEvidenceFileRefusesSymlinks(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	good := goodRec(t, "t")
	writeRecord(t, filepath.Join(outside, "real.json"), good)
	writeRecord(t, filepath.Join(root, "inner", "real.json"), good)
	if err := os.Symlink(filepath.Join(outside, "real.json"), filepath.Join(root, "link.json")); err != nil {
		t.Skipf("SKIPPED symlink refusal checks: os.Symlink is unavailable here (%v); on Windows enable Developer Mode or run elevated", err)
	}
	require.NoError(t, os.Symlink(outside, filepath.Join(root, "linkdir")))
	require.NoError(t, os.Symlink(filepath.Join(root, "inner", "real.json"), filepath.Join(root, "inlink.json")))
	require.NoError(t, os.Symlink(filepath.Join(root, "inner"), filepath.Join(root, "indir")))
	for _, p := range []string{"link.json", "linkdir/real.json", "inlink.json", "indir/real.json"} {
		_, err := LoadEvidenceFile(root, p)
		require.Error(t, err, p)
		assert.Contains(t, err.Error(), "symlink or reparse point", p)
	}
	if runtime.GOOS != "windows" {
		fifo := filepath.Join(root, "pipe.json")
		if err := mkfifo(fifo); err == nil {
			_, err := LoadEvidenceFile(root, "pipe.json")
			assert.Error(t, err)
		}
	}
}
