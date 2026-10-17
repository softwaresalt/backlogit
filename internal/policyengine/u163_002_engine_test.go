package policyengine

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/evidence"
)

var enforceRegistry = strings.Replace(canonicalRegistryYAML, "mode: report-only", "mode: enforce", 1)

func check(t *testing.T, eng *Engine, req Request) Decision {
	t.Helper()
	d, err := eng.Check(context.Background(), req)
	require.NoError(t, err)
	return d
}

func TestU163_002_SatisfiedGraphPasses(t *testing.T) {
	e := newEnv(t, enforceRegistry)
	d := check(t, e.engine(), e.request(e.good()))
	assert.Equal(t, DispositionPass, d.Disposition, "%+v", d.Errors)
	assert.True(t, d.Allowed)
	assert.True(t, d.ReviewReady)
	assert.Empty(t, d.Errors)
	require.Len(t, d.Gates, 6)
	for i, g := range d.Gates {
		assert.Equal(t, NodeTypes()[i], g.Node, "gates must be in canonical order")
		assert.Equal(t, "verified", g.Status)
	}
}

func TestU163_002_AbsentEvidenceBlocksByDefault(t *testing.T) {
	for _, node := range []NodeType{NodeHarnessScaffold, NodeRedPhaseVerify, NodeGreenPhaseVerify, NodeMergeReady} {
		t.Run(string(node), func(t *testing.T) {
			e := newEnv(t, enforceRegistry)
			recs := e.good()
			delete(recs, node)
			d := check(t, e.engine(), e.request(recs))
			assert.Equal(t, DispositionBlock, d.Disposition)
			assert.False(t, d.Allowed)
			assert.True(t, hasViolation(d, node, CodeEvidenceAbsent), "%+v", d.Errors)
			assert.NotEmpty(t, d.Errors[0].Reason)
		})
	}
}

func TestU163_002_NoEvidenceAtAllBlocksEveryRecordedGate(t *testing.T) {
	e := newEnv(t, enforceRegistry)
	d := check(t, e.engine(), e.request(nil))
	assert.False(t, d.Allowed)
	for _, node := range []NodeType{NodeHarnessScaffold, NodeRedPhaseVerify, NodeGreenPhaseVerify, NodeMergeReady} {
		assert.True(t, hasViolation(d, node, CodeEvidenceAbsent), string(node))
	}
}

func TestU163_002_UnmetUpstreamPropagatesToDependents(t *testing.T) {
	e := newEnv(t, enforceRegistry)
	recs := e.good()
	delete(recs, NodeHarnessScaffold)
	d := check(t, e.engine(), e.request(recs))
	assert.Equal(t, "unmet", gateStatus(d, NodeRedPhaseVerify))
	assert.True(t, hasViolation(d, NodeRedPhaseVerify, CodeUpstreamUnmet), "%+v", d.Errors)
	assert.True(t, hasViolation(d, NodeMergeReady, CodeUpstreamUnmet))
}

func TestU163_002_OutcomeFailBlocks(t *testing.T) {
	e := newEnv(t, enforceRegistry)
	recs := e.good()
	recs[NodeGreenPhaseVerify] = e.rec(NodeGreenPhaseVerify, e.c[3], e.at(3, 30), OutcomeFail, nil)
	d := check(t, e.engine(), e.request(recs))
	assert.False(t, d.Allowed)
	assert.True(t, hasViolation(d, NodeGreenPhaseVerify, CodeOutcomeNotPass))
}

func TestU163_002_DerivedGatesRequireTheDerivationInterface(t *testing.T) {
	cases := map[string]struct {
		mut  func(*env)
		node NodeType
		code string
	}{
		"review incomplete":        {func(e *env) { e.deriv.review.ReviewedCurrent = false }, NodeReviewComplete, CodeReviewIncomplete},
		"unresolved threads":       {func(e *env) { e.deriv.review.UnresolvedThreads = 2 }, NodeReviewComplete, CodeReviewIncomplete},
		"review degraded":          {func(e *env) { e.deriv.review.Status = evidence.StatusDegraded; e.deriv.review.Reason = "gh down" }, NodeReviewComplete, CodeDerivationDegraded},
		"review head mismatch":     {func(e *env) { e.deriv.review.HeadSHA = e.c[3] }, NodeReviewComplete, CodeDerivationMismatch},
		"ci failing":               {func(e *env) { e.deriv.ci.AllPassed = false }, NodeCIPass, CodeCIFailing},
		"ci degraded":              {func(e *env) { e.deriv.ci.Status = evidence.StatusDegraded }, NodeCIPass, CodeDerivationDegraded},
		"ci head mismatch":         {func(e *env) { e.deriv.ci.HeadSHA = e.c[3] }, NodeCIPass, CodeDerivationMismatch},
		"unknown derivation state": {func(e *env) { e.deriv.ci.Status = "weird" }, NodeCIPass, CodeDerivationDegraded},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t, enforceRegistry)
			tc.mut(e)
			d := check(t, e.engine(), e.request(e.good()))
			assert.False(t, d.Allowed)
			assert.True(t, hasViolation(d, tc.node, tc.code), "%+v", d.Errors)
			assert.True(t, hasViolation(d, NodeMergeReady, CodeUpstreamUnmet) || hasViolation(d, NodeMergeReady, CodeUpstreamUnverifiable), "%+v", d.Errors)
		})
	}
}

func TestU163_002_DerivationUnavailableBlocks(t *testing.T) {
	e := newEnv(t, enforceRegistry)
	d := check(t, e.engine(func(o *Options) { o.Derivation = nil }), e.request(e.good()))
	assert.False(t, d.Allowed)
	assert.True(t, hasViolation(d, NodeReviewComplete, CodeDerivationUnavailable))
	assert.True(t, hasViolation(d, NodeCIPass, CodeDerivationUnavailable))

	req := e.request(e.good())
	req.PR = 0
	d = check(t, e.engine(), req)
	assert.False(t, d.Allowed)
	assert.True(t, hasViolation(d, NodeReviewComplete, CodeDerivationUnavailable), "no PR means nothing to derive from")
}

func TestU163_002_SelfAssertedEvidenceForDerivedGatesIsRejected(t *testing.T) {
	e := newEnv(t, enforceRegistry)
	e.deriv.review.ReviewedCurrent = false
	recs := e.good()
	recs[NodeReviewComplete] = e.rec(NodeReviewComplete, e.c[4], e.at(4, 10), OutcomePass, nil)
	d := check(t, e.engine(), e.request(recs))
	assert.False(t, d.Allowed)
	assert.True(t, hasViolation(d, NodeReviewComplete, CodeSelfAsserted), "%+v", d.Errors)
}

func TestU163_002_DerivationCalledOnlyForEnabledDerivedGates(t *testing.T) {
	doc := strings.Replace(enforceRegistry, "{id: review-complete, enabled: true", "{id: review-complete, enabled: false", 1)
	doc = strings.Replace(doc, "{id: ci-pass, enabled: true", "{id: ci-pass, enabled: false", 1)
	e := newEnv(t, doc)
	d := check(t, e.engine(), e.request(e.good()))
	assert.True(t, d.Allowed, "%+v", d.Errors)
	assert.Zero(t, e.deriv.calls)
	assert.Equal(t, "not-applicable", gateStatus(d, NodeReviewComplete))
}

func TestU163_002_AmbiguousAndMalformedEvidenceIsRejected(t *testing.T) {
	cases := map[string]struct {
		mut  func(*env, map[NodeType]Record) []Record
		node NodeType
		code string
	}{
		"duplicate record": {func(e *env, m map[NodeType]Record) []Record {
			return append(e.request(m).Records, m[NodeHarnessScaffold])
		}, NodeHarnessScaffold, CodeEvidenceAmbiguous},
		"conflicting duplicate": {func(e *env, m map[NodeType]Record) []Record {
			return append(e.request(m).Records, e.rec(NodeHarnessScaffold, e.c[1], e.at(1, 40), OutcomePass, nil))
		}, NodeHarnessScaffold, CodeEvidenceAmbiguous},
		"wrong task": {func(e *env, m map[NodeType]Record) []Record {
			r := m[NodeGreenPhaseVerify]
			r.TaskID = "163.003-T"
			m[NodeGreenPhaseVerify] = r
			return e.request(m).Records
		}, NodeGreenPhaseVerify, CodeBindingMismatch},
		"tampered outcome": {func(e *env, m map[NodeType]Record) []Record {
			r := m[NodeGreenPhaseVerify]
			r.Outcome = OutcomePass
			r.CommitSHA = e.c[2]
			m[NodeGreenPhaseVerify] = r
			return e.request(m).Records
		}, NodeGreenPhaseVerify, CodeDigestMismatch},
		"unsupported schema": {func(e *env, m map[NodeType]Record) []Record {
			r := m[NodeGreenPhaseVerify]
			r.SchemaVersion = 2
			m[NodeGreenPhaseVerify] = r
			return e.request(m).Records
		}, NodeGreenPhaseVerify, CodeEvidenceInvalid},
		"bad commit sha": {func(e *env, m map[NodeType]Record) []Record {
			r := m[NodeGreenPhaseVerify]
			r.CommitSHA = "HEAD"
			m[NodeGreenPhaseVerify] = r
			return e.request(m).Records
		}, NodeGreenPhaseVerify, CodeEvidenceInvalid},
		"unknown outcome": {func(e *env, m map[NodeType]Record) []Record {
			r := m[NodeGreenPhaseVerify]
			r.Outcome = "maybe"
			m[NodeGreenPhaseVerify] = r
			return e.request(m).Records
		}, NodeGreenPhaseVerify, CodeEvidenceInvalid},
		"zero timestamp": {func(e *env, m map[NodeType]Record) []Record {
			r := m[NodeGreenPhaseVerify]
			r.RecordedAt = time.Time{}
			m[NodeGreenPhaseVerify] = r
			return e.request(m).Records
		}, NodeGreenPhaseVerify, CodeEvidenceInvalid},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t, enforceRegistry)
			m := e.good()
			req := e.request(m)
			req.Records = tc.mut(e, m)
			d := check(t, e.engine(), req)
			assert.False(t, d.Allowed)
			assert.True(t, hasViolation(d, tc.node, tc.code), "%+v", d.Errors)
			assert.Equal(t, "error", gateStatus(d, tc.node))
		})
	}
}

func TestU163_002_RecordForUnknownNodeBlocksTheWholeRequest(t *testing.T) {
	e := newEnv(t, enforceRegistry)
	req := e.request(e.good())
	bogus := req.Records[0]
	bogus.Node = "deploy-prod"
	req.Records = append(req.Records, bogus)
	d, err := e.engine().Check(context.Background(), req)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidRequest)
	assert.False(t, d.Allowed)
	assert.Equal(t, DispositionBlock, d.Disposition)
}

func TestU163_002_FabricatedEvidenceIsRejected(t *testing.T) {
	cases := map[string]struct {
		mut  func(*env, map[NodeType]Record)
		node NodeType
		code string
	}{
		"sha unknown to git": {func(e *env, m map[NodeType]Record) {
			m[NodeGreenPhaseVerify] = e.rec(NodeGreenPhaseVerify, sha(999), e.at(3, 30), OutcomePass, nil)
		}, NodeGreenPhaseVerify, CodeSHAUnverifiable},
		"sha on a side branch": {func(e *env, m map[NodeType]Record) {
			e.mem.AddCommit(sha(50), e.at(3, 0), e.c[0])
			m[NodeGreenPhaseVerify] = e.rec(NodeGreenPhaseVerify, sha(50), e.at(3, 30), OutcomePass, nil)
		}, NodeGreenPhaseVerify, CodeSHANotInHistory},
		"retroactive timestamp before its own commit": {func(e *env, m map[NodeType]Record) {
			m[NodeGreenPhaseVerify] = e.rec(NodeGreenPhaseVerify, e.c[3], e.at(0, 5), OutcomePass, nil)
		}, NodeGreenPhaseVerify, CodeTimestampRetroactive},
		"timestamp in the future": {func(e *env, m map[NodeType]Record) {
			m[NodeGreenPhaseVerify] = e.rec(NodeGreenPhaseVerify, e.c[3], e.t0.Add(48*time.Hour), OutcomePass, nil)
		}, NodeGreenPhaseVerify, CodeTimestampFuture},
		"red artifact bound to a different commit": {func(e *env, m map[NodeType]Record) {
			m[NodeRedPhaseVerify] = e.rec(NodeRedPhaseVerify, e.c[2], e.at(2, 30), OutcomePass, e.redArtifact(e.c[3]))
		}, NodeRedPhaseVerify, CodeArtifactInvalid},
		"red artifact missing": {func(e *env, m map[NodeType]Record) {
			m[NodeRedPhaseVerify] = e.rec(NodeRedPhaseVerify, e.c[2], e.at(2, 30), OutcomePass, nil)
		}, NodeRedPhaseVerify, CodeArtifactInvalid},
		"red artifact for another task": {func(e *env, m map[NodeType]Record) {
			e.task = "163.009-T"
			art := e.redArtifact(e.c[2])
			e.task = "163.002-T"
			m[NodeRedPhaseVerify] = e.rec(NodeRedPhaseVerify, e.c[2], e.at(2, 30), OutcomePass, art)
		}, NodeRedPhaseVerify, CodeArtifactInvalid},
		"red artifact garbage": {func(e *env, m map[NodeType]Record) {
			m[NodeRedPhaseVerify] = e.rec(NodeRedPhaseVerify, e.c[2], e.at(2, 30), OutcomePass, json.RawMessage(`{"x":1}`))
		}, NodeRedPhaseVerify, CodeArtifactInvalid},
		"artifact on a non-red node": {func(e *env, m map[NodeType]Record) {
			m[NodeGreenPhaseVerify] = e.rec(NodeGreenPhaseVerify, e.c[3], e.at(3, 30), OutcomePass, e.redArtifact(e.c[3]))
		}, NodeGreenPhaseVerify, CodeEvidenceInvalid},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t, enforceRegistry)
			m := e.good()
			tc.mut(e, m)
			d := check(t, e.engine(), e.request(m))
			assert.False(t, d.Allowed)
			assert.True(t, hasViolation(d, tc.node, tc.code), "%+v", d.Errors)
		})
	}
}

func TestU163_002_GitFailuresNeverPass(t *testing.T) {
	e := newEnv(t, enforceRegistry)
	for name, g := range map[string]Git{
		"ancestry error": &flakyGit{Git: e.mem, failAncestor: true},
		"time error":     &flakyGit{Git: e.mem, failTime: true},
	} {
		t.Run(name, func(t *testing.T) {
			d := check(t, e.engine(func(o *Options) { o.Git = g }), e.request(e.good()))
			assert.False(t, d.Allowed)
			assert.NotEmpty(t, d.Errors)
		})
	}
}

type flakyGit struct {
	Git
	failAncestor, failTime bool
}

func (f *flakyGit) IsAncestor(ctx context.Context, a, d string) (bool, error) {
	if f.failAncestor {
		return true, errors.New("git exploded")
	}
	return f.Git.IsAncestor(ctx, a, d)
}

func (f *flakyGit) CommitTime(ctx context.Context, s string) (time.Time, error) {
	if f.failTime {
		return time.Now(), errors.New("git exploded")
	}
	return f.Git.CommitTime(ctx, s)
}

func TestU163_002_OutOfOrderEvidenceBlocks(t *testing.T) {
	cases := map[string]struct {
		mut  func(*env, map[NodeType]Record)
		node NodeType
	}{
		"green commit older than red commit": {func(e *env, m map[NodeType]Record) {
			m[NodeGreenPhaseVerify] = e.rec(NodeGreenPhaseVerify, e.c[1], e.at(3, 30), OutcomePass, nil)
		}, NodeGreenPhaseVerify},
		"red recorded before harness": {func(e *env, m map[NodeType]Record) {
			m[NodeRedPhaseVerify] = e.rec(NodeRedPhaseVerify, e.c[2], e.at(2, 0), OutcomePass, e.redArtifact(e.c[2]))
			m[NodeHarnessScaffold] = e.rec(NodeHarnessScaffold, e.c[1], e.at(2, 50), OutcomePass, nil)
		}, NodeRedPhaseVerify},
		"merge-ready not at the head": {func(e *env, m map[NodeType]Record) {
			m[NodeMergeReady] = e.rec(NodeMergeReady, e.c[3], e.at(4, 30), OutcomePass, nil)
		}, NodeMergeReady},
		"harness after red": {func(e *env, m map[NodeType]Record) {
			m[NodeHarnessScaffold] = e.rec(NodeHarnessScaffold, e.c[3], e.at(3, 10), OutcomePass, nil)
		}, NodeRedPhaseVerify},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t, enforceRegistry)
			m := e.good()
			tc.mut(e, m)
			d := check(t, e.engine(), e.request(m))
			assert.False(t, d.Allowed)
			assert.True(t, hasViolation(d, tc.node, CodeOutOfOrder), "%+v", d.Errors)
		})
	}
}

func TestU163_002_EqualCommitsAndTimestampsAreInOrder(t *testing.T) {
	e := newEnv(t, enforceRegistry)
	m := e.good()
	m[NodeRedPhaseVerify] = e.rec(NodeRedPhaseVerify, e.c[2], e.at(2, 30), OutcomePass, e.redArtifact(e.c[2]))
	m[NodeGreenPhaseVerify] = e.rec(NodeGreenPhaseVerify, e.c[2], e.at(2, 30), OutcomePass, nil)
	d := check(t, e.engine(), e.request(m))
	assert.True(t, d.Allowed, "%+v", d.Errors)
}

func TestU163_002_ShaVerificationPrefersProtectedRemoteRefs(t *testing.T) {
	t.Run("no protected ref resolves", func(t *testing.T) {
		e := newEnv(t, enforceRegistry)
		d := check(t, e.engine(func(o *Options) { o.ProtectedRefs = []string{"refs/remotes/origin/nope"} }), e.request(e.good()))
		assert.False(t, d.Allowed)
		assert.True(t, hasViolation(d, "", CodeNoProtectedRef), "%+v", d.Errors)
	})
	t.Run("head only on a local branch", func(t *testing.T) {
		e := newEnv(t, enforceRegistry)
		e.mem.SetRef("refs/heads/feat/x", e.c[4])
		e.mem.SetRef("refs/remotes/origin/feat/x", e.c[3])
		d := check(t, e.engine(), e.request(e.good()))
		assert.False(t, d.Allowed)
		assert.True(t, hasViolation(d, "", CodeHeadUnpublished), "%+v", d.Errors)
	})
	t.Run("one resolvable protected ref suffices", func(t *testing.T) {
		e := newEnv(t, enforceRegistry)
		d := check(t, e.engine(func(o *Options) {
			o.ProtectedRefs = []string{"refs/remotes/origin/missing", "refs/remotes/origin/feat/x"}
		}), e.request(e.good()))
		assert.True(t, d.Allowed, "%+v", d.Errors)
	})
	t.Run("local refs are never accepted as protected", func(t *testing.T) {
		e := newEnv(t, enforceRegistry)
		for _, ref := range []string{"", "HEAD", "main", "refs/heads/main", "refs/remotes/", "refs/remotes/../x", "refs/remotes/origin/-x", "refs/remotes/origin/a b", "refs/remotes/origin/a\nb"} {
			_, err := NewEngine(e.reg, Options{Git: e.mem, ProtectedRefs: []string{ref}})
			require.Error(t, err, ref)
		}
		_, err := NewEngine(e.reg, Options{Git: e.mem})
		require.Error(t, err, "at least one protected ref is required")
	})
}

func TestU163_002_PerGateEnableAndReportOnlyRollout(t *testing.T) {
	t.Run("disabled gate does not block", func(t *testing.T) {
		doc := strings.Replace(enforceRegistry, "{id: green-phase-verify, enabled: true", "{id: green-phase-verify, enabled: false", 1)
		e := newEnv(t, doc)
		m := e.good()
		delete(m, NodeGreenPhaseVerify)
		d := check(t, e.engine(), e.request(m))
		assert.True(t, d.Allowed, "%+v", d.Errors)
		assert.Equal(t, "not-applicable", gateStatus(d, NodeGreenPhaseVerify))
	})
	t.Run("disabled gate does not hide other gates", func(t *testing.T) {
		doc := strings.Replace(enforceRegistry, "{id: green-phase-verify, enabled: true", "{id: green-phase-verify, enabled: false", 1)
		e := newEnv(t, doc)
		m := e.good()
		delete(m, NodeRedPhaseVerify)
		d := check(t, e.engine(), e.request(m))
		assert.False(t, d.Allowed)
	})
	t.Run("report-only reports but allows", func(t *testing.T) {
		e := newEnv(t, canonicalRegistryYAML)
		m := e.good()
		delete(m, NodeHarnessScaffold)
		d := check(t, e.engine(), e.request(m))
		assert.Equal(t, DispositionReportOnly, d.Disposition)
		assert.True(t, d.Allowed)
		assert.False(t, d.ReviewReady, "the truthful verdict must survive report-only")
		assert.True(t, hasViolation(d, NodeHarnessScaffold, CodeEvidenceAbsent))
		assert.Equal(t, ModeReportOnly, d.Mode)
	})
	t.Run("report-only satisfied graph is a plain pass", func(t *testing.T) {
		e := newEnv(t, canonicalRegistryYAML)
		d := check(t, e.engine(), e.request(e.good()))
		assert.Equal(t, DispositionPass, d.Disposition)
	})
	t.Run("an option may tighten but never relax the mode", func(t *testing.T) {
		e := newEnv(t, canonicalRegistryYAML)
		m := e.good()
		delete(m, NodeHarnessScaffold)
		d := check(t, e.engine(func(o *Options) { o.Mode = ModeEnforce }), e.request(m))
		assert.False(t, d.Allowed)
		assert.Equal(t, ModeEnforce, d.Mode)

		e2 := newEnv(t, enforceRegistry)
		_, err := NewEngine(e2.reg, Options{Git: e2.mem, ProtectedRefs: []string{"refs/remotes/origin/main"}, Mode: ModeReportOnly})
		require.Error(t, err)
		_, err = NewEngine(e2.reg, Options{Git: e2.mem, ProtectedRefs: []string{"refs/remotes/origin/main"}, Mode: "bogus"})
		require.Error(t, err)
	})
}

func TestU163_002_RequestValidationFailsClosed(t *testing.T) {
	e := newEnv(t, canonicalRegistryYAML) // report-only: even so, bad input must block
	eng := e.engine()
	bad := map[string]func(*Request){
		"unknown surface": func(r *Request) { r.Surface = "ci-bot" },
		"empty surface":   func(r *Request) { r.Surface = "" },
		"bad task":        func(r *Request) { r.TaskID = "../x" },
		"bad head":        func(r *Request) { r.HeadCommit = "main" },
		"negative pr":     func(r *Request) { r.PR = -1 },
		"too many records": func(r *Request) {
			r.Records = make([]Record, MaxRecords+1)
		},
	}
	for name, mut := range bad {
		t.Run(name, func(t *testing.T) {
			req := e.request(e.good())
			mut(&req)
			d, err := eng.Check(context.Background(), req)
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrInvalidRequest)
			assert.False(t, d.Allowed, "invalid input is never a rollout relaxation")
			assert.Equal(t, DispositionBlock, d.Disposition)
		})
	}
	t.Run("cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		d, err := eng.Check(ctx, e.request(e.good()))
		require.Error(t, err)
		assert.False(t, d.Allowed)
	})
	t.Run("nil engine", func(t *testing.T) {
		var nilEng *Engine
		d, err := nilEng.Check(context.Background(), e.request(e.good()))
		require.Error(t, err)
		assert.False(t, d.Allowed)
	})
}

func TestU163_002_OutputIsDeterministicAndBounded(t *testing.T) {
	e := newEnv(t, enforceRegistry)
	eng := e.engine()
	req := e.request(map[NodeType]Record{})
	d1 := check(t, eng, req)
	d2 := check(t, eng, req)
	b1, _ := json.Marshal(d1)
	b2, _ := json.Marshal(d2)
	assert.JSONEq(t, string(b1), string(b2))
	assert.LessOrEqual(t, len(d1.Errors), MaxErrors)
	for _, v := range d1.Errors {
		assert.LessOrEqual(t, len(v.Reason), MaxErrorBytes)
	}
	e.deriv.review.Reason = strings.Repeat("é", 5000) + "\x1b[31m"
	e.deriv.review.Status = evidence.StatusDegraded
	d := check(t, eng, e.request(e.good()))
	for _, v := range d.Errors {
		assert.LessOrEqual(t, len(v.Reason), MaxErrorBytes)
		assert.NotContains(t, v.Reason, "\x1b")
	}
}

func TestU163_002_CacheNeverServesAStaleDecision(t *testing.T) {
	e := newEnv(t, enforceRegistry)
	eng := e.engine()
	req := e.request(e.good())
	assert.True(t, check(t, eng, req).Allowed)
	e.deriv.ci.AllPassed = false
	assert.False(t, check(t, eng, req).Allowed)
	e.deriv.ci.AllPassed = true
	assert.True(t, check(t, eng, req).Allowed)
}

func TestU163_002_EmptyEvidenceIsDistinctFromMissingEvidence(t *testing.T) {
	e := newEnv(t, enforceRegistry)
	req := e.request(e.good())
	req.Records = []Record{}
	d := check(t, e.engine(), req)
	assert.False(t, d.Allowed)
	req.Records = nil
	d2 := check(t, e.engine(), req)
	assert.False(t, d2.Allowed)
	assert.Equal(t, len(d.Errors), len(d2.Errors))
}

func TestU163_002_ReviewAndCIBoundToTheHeadAreRequired(t *testing.T) {
	e := newEnv(t, enforceRegistry)
	e.deriv.review.HeadSHA = ""
	d := check(t, e.engine(), e.request(e.good()))
	assert.False(t, d.Allowed)
	assert.True(t, hasViolation(d, NodeReviewComplete, CodeDerivationMismatch))
}
