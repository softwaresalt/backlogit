package policyengine

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/evidence"
	"github.com/softwaresalt/backlogit/internal/redhonesty"
)

func sha(n int) string { return fmt.Sprintf("%040x", n) }

type fakeDeriv struct {
	review evidence.ReviewResult
	ci     evidence.CIResult
	calls  atomic.Int64
}

func (f *fakeDeriv) ReviewEvidence(_ context.Context, _ int) evidence.ReviewResult {
	f.calls.Add(1)
	return f.review
}

func (f *fakeDeriv) CIEvidence(_ context.Context, _ int) evidence.CIResult {
	f.calls.Add(1)
	return f.ci
}

// env is a satisfied-by-default scenario: harness c1 -> red/green proof c2 ->
// green c3 -> head c4, published on a remote-tracking ref.
type env struct {
	t     *testing.T
	mem   *MemGit
	reg   *Registry
	deriv *fakeDeriv
	task  string
	t0    time.Time
	c     [5]string
}

func newEnv(t *testing.T, registry string) *env {
	t.Helper()
	reg, err := LoadRegistry([]byte(registry))
	require.NoError(t, err)
	e := &env{t: t, mem: NewMemGit(), reg: reg, task: "163.002-T", t0: time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)}
	for i := range e.c {
		e.c[i] = sha(i + 1)
	}
	e.mem.AddCommit(e.c[0], e.t0)
	for i := 1; i < len(e.c); i++ {
		e.mem.AddCommit(e.c[i], e.t0.Add(time.Duration(i)*time.Minute), e.c[i-1])
	}
	e.mem.SetRef("refs/remotes/origin/main", e.c[0])
	e.mem.SetRef("refs/remotes/origin/feat/x", e.c[4])
	e.deriv = &fakeDeriv{
		review: evidence.ReviewResult{Status: evidence.StatusVerified, PR: 7, HeadSHA: e.c[4], ReviewedCurrent: true},
		ci:     evidence.CIResult{Status: evidence.StatusVerified, PR: 7, HeadSHA: e.c[4], AllPassed: true},
	}
	return e
}

func (e *env) engine(mut ...func(*Options)) *Engine {
	e.t.Helper()
	opts := Options{
		Git: e.mem, Derivation: e.deriv, Now: func() time.Time { return e.t0.Add(time.Hour) },
		ProtectedRefs: []string{"refs/remotes/origin/main", "refs/remotes/origin/feat/x"},
	}
	for _, m := range mut {
		m(&opts)
	}
	eng, err := NewEngine(e.reg, opts)
	require.NoError(e.t, err)
	return eng
}

func (e *env) at(i int, sec int) time.Time {
	return e.t0.Add(time.Duration(i)*time.Minute + time.Duration(sec)*time.Second)
}

func (e *env) redArtifact(commit string) json.RawMessage {
	e.t.Helper()
	a, err := redhonesty.BuildArtifact(e.task, e.c[0], commit, []redhonesty.Verdict{{
		Test:     redhonesty.NewTest{Package: ".", File: "x_test.go", Name: "TestX"},
		Baseline: redhonesty.ResultFail, Head: redhonesty.ResultPass,
	}})
	require.NoError(e.t, err)
	raw, err := json.Marshal(a)
	require.NoError(e.t, err)
	return raw
}

func (e *env) rec(node NodeType, commit string, at time.Time, outcome string, artifact json.RawMessage) Record {
	e.t.Helper()
	r, err := NewRecord(e.task, node, commit, at, outcome, artifact)
	require.NoError(e.t, err)
	return r
}

// good returns the satisfied evidence set, keyed by node.
func (e *env) good() map[NodeType]Record {
	return map[NodeType]Record{
		NodeHarnessScaffold:  e.rec(NodeHarnessScaffold, e.c[1], e.at(1, 30), OutcomePass, nil),
		NodeRedPhaseVerify:   e.rec(NodeRedPhaseVerify, e.c[2], e.at(2, 30), OutcomePass, e.redArtifact(e.c[2])),
		NodeGreenPhaseVerify: e.rec(NodeGreenPhaseVerify, e.c[3], e.at(3, 30), OutcomePass, nil),
		NodeMergeReady:       e.rec(NodeMergeReady, e.c[4], e.at(4, 30), OutcomePass, nil),
	}
}

func (e *env) request(recs map[NodeType]Record) Request {
	var out []Record
	for _, nt := range NodeTypes() {
		if r, ok := recs[nt]; ok {
			out = append(out, r)
		}
	}
	return Request{Surface: SurfaceCLI, TaskID: e.task, HeadCommit: e.c[4], PR: 7, Records: out}
}

func hasViolation(d Decision, node NodeType, code string) bool {
	for _, v := range d.Errors {
		if v.Node == string(node) && v.Code == code {
			return true
		}
	}
	return false
}

func gateStatus(d Decision, node NodeType) string {
	for _, g := range d.Gates {
		if g.Node == node {
			return g.Status
		}
	}
	return ""
}
