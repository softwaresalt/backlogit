package evidencedag

import (
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/faultline"
)

func TestHardening_RelativeForbiddenRootsAreCanonicalized(t *testing.T) {
	base := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(base, "evidence"), 0o700))
	require.NoError(t, os.MkdirAll(filepath.Join(base, "safe"), 0o700))
	t.Chdir(base)

	t.Run("ledger", func(t *testing.T) {
		_, err := OpenLedger(filepath.Join(base, "evidence", "w.jsonl"), "evidence")
		assert.Error(t, err, "a relative root is resolved against the working directory")
		_, err = OpenLedger(filepath.Join(base, "evidence", "w.jsonl"), ".")
		assert.Error(t, err, "the working directory itself is a forbidden root")
		_, err = OpenLedger(filepath.Join(base, "safe", "w.jsonl"), "evidence")
		assert.NoError(t, err)
	})
	t.Run("anchor", func(t *testing.T) {
		_, err := NewFileAnchorStore(filepath.Join(base, "evidence", "a.json"), "evidence")
		assert.Error(t, err)
		_, err = NewFileAnchorStore(filepath.Join(base, "evidence", "a.json"), "./evidence/../evidence")
		assert.Error(t, err)
		_, err = NewFileAnchorStore(filepath.Join(base, "safe", "a.json"), "evidence")
		assert.NoError(t, err)
	})
	t.Run("trust", func(t *testing.T) {
		p := filepath.Join(base, "evidence", "trust.json")
		require.NoError(t, os.WriteFile(p, trustDoc([]map[string]any{}, map[string][]string{}), 0o600))
		_, err := LoadTrust(p, "evidence")
		assert.ErrorIs(t, err, ErrTrustInvalid)
		q := filepath.Join(base, "safe", "trust.json")
		require.NoError(t, os.WriteFile(q, trustDoc([]map[string]any{}, map[string][]string{}), 0o600))
		_, err = LoadTrust(q, "evidence")
		assert.NoError(t, err)
	})
	t.Run("not-yet-existing root still excludes its subtree", func(t *testing.T) {
		_, err := OpenLedger(filepath.Join(base, "safe", "w.jsonl"), "safe/not-created-yet/..")
		assert.Error(t, err, "dot-dot resolves to safe, which contains the ledger")
		_, err = OpenLedger(filepath.Join(base, "safe", "w.jsonl"), filepath.Join(base, "future"))
		assert.NoError(t, err, "a non-existent root elsewhere is canonicalized, not an error")
	})
}

func TestHardening_WaiverLifetimeIsOverflowSafe(t *testing.T) {
	cases := []struct {
		name          string
		before, after int64
	}{
		{"int64 extremes overflow the old duration math", math.MinInt64, math.MaxInt64},
		{"huge lifetime", 0, math.MaxInt64},
		{"negative not_before", -1, 100},
		{"beyond the timestamp bound", 1 << 41, 1<<41 + 10},
		{"lifetime just over the cap", 1_799_999_000, 1_799_999_000 + int64(MaxWaiverLifetime/time.Second) + 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			c := f.claims()
			c.NotBefore, c.ExpiresAt = tc.before, tc.after
			d := decl([]Node{pnode("t", true)})
			_, err := f.svc.Accept(signClaims(f.alice, c), ship(), lookupFor(d))
			require.ErrorIs(t, err, ErrWaiverDenied)
			recs, rerr := f.ledger.Records(nil)
			require.NoError(t, rerr)
			assert.Empty(t, recs)
		})
	}
	f := newFixture(t)
	c := f.claims()
	c.ExpiresAt = c.NotBefore + int64(MaxWaiverLifetime/time.Second)
	_, err := f.svc.Accept(signClaims(f.alice, c), ship(), lookupFor(decl([]Node{pnode("t", true)})))
	assert.NoError(t, err, "exactly the maximum lifetime is allowed")
}

func TestHardening_AnchorStoreRejectsForksAndStaysMonotonicUnderConcurrency(t *testing.T) {
	h1, h2 := strings.Repeat("a", 64), strings.Repeat("b", 64)
	t.Run("memory", func(t *testing.T) {
		m := &MemoryAnchorStore{}
		require.NoError(t, m.Store(Anchor{Seq: 3, Hash: h1}))
		require.NoError(t, m.Store(Anchor{Seq: 3, Hash: h1}), "an identical store is idempotent")
		assert.ErrorIs(t, m.Store(Anchor{Seq: 3, Hash: h2}), ErrAnchorFork)
		assert.ErrorIs(t, m.Store(Anchor{Seq: 2, Hash: h1}), ErrAnchorFork)
		got, _, _ := m.Load()
		assert.Equal(t, Anchor{Seq: 3, Hash: h1}, got)
	})
	t.Run("file", func(t *testing.T) {
		dir := t.TempDir()
		s, err := NewFileAnchorStore(filepath.Join(dir, "a.json"), testForbiddenRoot)
		require.NoError(t, err)
		require.NoError(t, s.Store(Anchor{Seq: 3, Hash: h1}))
		require.NoError(t, s.Store(Anchor{Seq: 3, Hash: h1}))
		assert.ErrorIs(t, s.Store(Anchor{Seq: 3, Hash: h2}), ErrAnchorFork)
		got, _, err := s.Load()
		require.NoError(t, err)
		assert.Equal(t, Anchor{Seq: 3, Hash: h1}, got)
	})
	t.Run("concurrent", func(t *testing.T) {
		old := lockWait
		lockWait = 2 * time.Minute
		defer func() { lockWait = old }()
		dir := t.TempDir()
		p := filepath.Join(dir, "a.json")
		const n = 16
		var wg sync.WaitGroup
		for i := 1; i <= n; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				s, err := NewFileAnchorStore(p, testForbiddenRoot) // distinct handles, one file
				if err != nil {
					t.Error(err)
					return
				}
				h := strings.Repeat(string(rune('a'+i%6)), 63) + "0"
				if err := s.Store(Anchor{Seq: i, Hash: h}); err != nil && !errors.Is(err, ErrAnchorFork) {
					t.Errorf("store %d: %v", i, err)
				}
			}(i)
		}
		wg.Wait()
		s, err := NewFileAnchorStore(p, testForbiddenRoot)
		require.NoError(t, err)
		got, ok, err := s.Load()
		require.NoError(t, err)
		require.True(t, ok)
		assert.Equal(t, n, got.Seq, "the highest sequence wins and nothing moved backwards")
		_, statErr := os.Lstat(p + ".lock")
		assert.True(t, errors.Is(statErr, os.ErrNotExist), "the anchor lock is released")
	})
}

func TestHardening_AdvisoryOnlyTerminalsAreNeverReady(t *testing.T) {
	advisory := pnode("t", true)
	advisory.OnUnmet = OnUnmetReport
	d := decl([]Node{advisory})
	f := newFixture(t)
	for _, mode := range []Mode{ModeEnforce, ModeReportOnly} {
		g, err := NewGate(d, mode, f.svc)
		require.NoError(t, err)
		dec, err := g.Check(context.Background(), GateInput{Shipment: ship()})
		require.NoError(t, err)
		assert.False(t, dec.ReviewReady, string(mode))
		assert.Equal(t, RollupUnproven, dec.Rollup.State, "a distinct status, not ready and not blocked")
		assert.Equal(t, 0, dec.Rollup.ProvenTerminals)
		assert.Equal(t, mode == ModeReportOnly, dec.Allowed, string(mode))
		assert.Contains(t, dec.Reason, "prove nothing")
	}

	t.Run("an empty executor run proves nothing", func(t *testing.T) {
		res, err := RunExecutors(context.Background(), d, ship(), nil)
		require.NoError(t, err)
		g, err := NewGate(d, ModeEnforce, f.svc)
		require.NoError(t, err)
		dec, err := g.Check(context.Background(), GateInput{Shipment: ship(), Evidence: res.Evidence, Failures: res.Failures})
		require.NoError(t, err)
		assert.False(t, dec.ReviewReady)
		assert.False(t, dec.Allowed)
	})
	t.Run("one verified terminal alongside advisory is ready", func(t *testing.T) {
		d2 := decl([]Node{advisory, pnode("v", true)})
		g, err := NewGate(d2, ModeEnforce, f.svc)
		require.NoError(t, err)
		dec, err := g.Check(context.Background(), GateInput{Shipment: ship(), Evidence: map[string]EvidenceRecord{"v": goodRec(t, "v")}})
		require.NoError(t, err)
		assert.True(t, dec.ReviewReady)
		assert.Equal(t, 1, dec.Rollup.ProvenTerminals)
	})
	t.Run("a valid waiver is proof", func(t *testing.T) {
		g, err := NewGate(d, ModeEnforce, f.svc)
		require.NoError(t, err)
		dec, err := g.Check(context.Background(), GateInput{Shipment: ship(), WaiverTokens: [][]byte{signClaims(f.alice, f.claims())}})
		require.NoError(t, err)
		assert.True(t, dec.ReviewReady)
	})
}

func TestHardening_EvidenceAndFailureForOneNodeAreMutuallyExclusive(t *testing.T) {
	d := decl([]Node{pnode("t", true)})
	ev := mustEval(t, d)
	in := Input{Shipment: ship(), Evidence: map[string]EvidenceRecord{"t": goodRec(t, "t")}, Failures: map[string]string{"t": "boom"}}
	_, err := ev.Evaluate(in)
	assert.ErrorIs(t, err, ErrInvalidInput)
	g, err := NewGate(d, ModeEnforce, nil)
	require.NoError(t, err)
	_, err = g.Check(context.Background(), GateInput{Shipment: ship(), Evidence: in.Evidence, Failures: in.Failures})
	assert.ErrorIs(t, err, ErrInvalidInput)
}

func TestHardening_OwnExecutorFailureBeatsUpstreamMasking(t *testing.T) {
	d := decl([]Node{pnode("base", false), pnode("t", true)}, Edge{"t", "base"})
	rep, err := mustEval(t, d).Evaluate(Input{Shipment: ship(), Failures: map[string]string{"t": "detector crashed"}})
	require.NoError(t, err)
	n, _ := rep.Node("t")
	assert.Equal(t, StatusError, n.Status)
	assert.Contains(t, n.Reason, "executor failed: detector crashed")
	assert.Equal(t, StatusUnmet, statusOf(t, rep, "base"))
}

func TestHardening_WaiverReportListsMaskedUpstreams(t *testing.T) {
	f := newFixture(t)
	d := decl([]Node{pnode("base", false), pnode("t", true)}, Edge{"t", "base"})
	g, err := NewGate(d, ModeEnforce, f.svc)
	require.NoError(t, err)
	dec, err := g.Check(context.Background(), GateInput{Shipment: ship(), WaiverTokens: [][]byte{signClaims(f.alice, f.claims())}})
	require.NoError(t, err)
	assert.True(t, dec.ReviewReady)
	n, _ := dec.Report.Node("t")
	assert.Equal(t, StatusWaived, n.Status)
	assert.Equal(t, []string{"base"}, n.MaskedUpstreams, "the unmet upstream hidden by the waiver is named")
}

func TestHardening_DoomedWaiverIsNotBurned(t *testing.T) {
	f := newFixture(t)
	d := decl([]Node{pnode("t", true), pnode("u", true)})
	g, err := NewGate(d, ModeEnforce, f.svc)
	require.NoError(t, err)

	forged := rec(t, "t", parityArtifact(testTaskB, testHead, faultline.StatusPass))
	dec, err := g.Check(context.Background(), GateInput{
		Shipment: ship(), Evidence: map[string]EvidenceRecord{"t": forged},
		WaiverTokens: [][]byte{signClaims(f.alice, f.claims())},
	})
	require.NoError(t, err)
	assert.False(t, dec.ReviewReady)
	require.Len(t, dec.Denials, 1)
	assert.Contains(t, dec.Denials[0].Reason, "not consumed")
	recs, err := f.ledger.Records(nil)
	require.NoError(t, err)
	assert.Empty(t, recs, "the ledger is untouched")
	_, ok, err := f.anchors.Load()
	require.NoError(t, err)
	assert.False(t, ok, "the anchor is untouched")

	a := goodRec(t, "t")
	b := a
	b.NodeID = "u"
	c := f.claims()
	dec, err = g.Check(context.Background(), GateInput{
		Shipment: ship(), Evidence: map[string]EvidenceRecord{"t": a, "u": b},
		WaiverTokens: [][]byte{signClaims(f.alice, c)},
	})
	require.NoError(t, err)
	require.Len(t, dec.Denials, 1, "replayed evidence also dooms the node")
	recs, err = f.ledger.Records(nil)
	require.NoError(t, err)
	assert.Empty(t, recs)
}

func TestHardening_NilContextIsInvalidInput(t *testing.T) {
	f := newFixture(t)
	g := f.gate(ModeEnforce, f.svc)
	var nilCtx context.Context
	_, err := g.Check(nilCtx, GateInput{Shipment: ship()})
	assert.ErrorIs(t, err, ErrInvalidInput)
	x := fnExec(t, passFn(testTaskA))
	_, err = x.Execute(nilCtx, ExecRequest{Shipment: ship(), Node: pnode("t", true)})
	assert.ErrorIs(t, err, ErrInvalidInput)
	_, err = (&ParityExecutor{Source: func(context.Context, ExecRequest) (ParityResult, error) { return ParityResult{}, nil }}).Execute(nilCtx, ExecRequest{})
	assert.ErrorIs(t, err, ErrInvalidInput)
	_, err = (&RedHonestyExecutor{Gate: nil}).Execute(nilCtx, ExecRequest{})
	assert.Error(t, err)
}

func TestHardening_ExecutorsAlwaysBoundedAndAbandonedGoroutinesCapped(t *testing.T) {
	newStuck := func(t *testing.T) NodeExecutor {
		release := make(chan struct{})
		t.Cleanup(func() {
			close(release)
			require.Eventually(t, func() bool { return abandonedCount() == 0 }, 5*time.Second, 5*time.Millisecond)
		})
		return fnExec(t, func(context.Context, ExecRequest) (faultline.EvidenceArtifact, error) {
			<-release // ignores ctx on purpose
			return faultline.EvidenceArtifact{}, errors.New("released")
		})
	}

	t.Run("negative timeout is invalid", func(t *testing.T) {
		_, err := RunExecutorsWith(context.Background(), decl([]Node{pnode("a", true)}), ship(), nil, ExecOptions{NodeTimeout: -time.Second})
		assert.ErrorIs(t, err, ErrInvalidInput)
	})
	t.Run("parent cancellation frees a context-ignoring executor with no explicit timeout", func(t *testing.T) {
		stuck := newStuck(t)
		ctx, cancel := context.WithCancel(context.Background())
		go func() { time.Sleep(50 * time.Millisecond); cancel() }()
		res, err := RunExecutors(ctx, decl([]Node{pnode("a", true)}), ship(), map[string]NodeExecutor{"a": stuck})
		require.NoError(t, err)
		assert.Contains(t, res.Failures["a"], "interrupted")
		assert.Equal(t, 1, abandonedCount())
	})
	t.Run("the abandoned cap stops further executors", func(t *testing.T) {
		stuck := newStuck(t)
		old := maxAbandonedExecutors
		maxAbandonedExecutors = 2
		defer func() { maxAbandonedExecutors = old }()
		d3 := decl([]Node{pnode("a", true), pnode("b", true), pnode("c", true)})
		res, err := RunExecutorsWith(context.Background(), d3, ship(),
			map[string]NodeExecutor{"a": stuck, "b": stuck, "c": stuck}, ExecOptions{NodeTimeout: 30 * time.Millisecond})
		require.ErrorIs(t, err, ErrExecutorsAbandoned)
		assert.Contains(t, res.Failures["a"], "timed out")
		assert.Contains(t, res.Failures["b"], "timed out")
		assert.NotContains(t, res.Failures, "c")
	})
}
