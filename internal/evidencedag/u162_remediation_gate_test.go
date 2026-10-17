package evidencedag

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRemediation_WaiverCopyCannotBeRetargeted(t *testing.T) {
	f := newFixture(t)
	d := decl([]Node{pnode("t", true), pnode("u", true)})
	w, err := f.svc.Accept(signClaims(f.alice, f.claims()), ship(), lookupFor(d))
	require.NoError(t, err)
	ev := mustEval(t, d)
	run := func(m map[string]Waiver) Report {
		rep, err := ev.Evaluate(Input{Shipment: ship(), Waivers: m})
		require.NoError(t, err)
		return rep
	}
	assert.Equal(t, StatusWaived, statusOf(t, run(map[string]Waiver{"t": w}), "t"))

	toOther := w
	toOther.nodeID = "u"
	assert.Equal(t, StatusError, statusOf(t, run(map[string]Waiver{"u": toOther}), "u"), "a copy mutated to another node is rejected")
	assert.Equal(t, StatusError, statusOf(t, run(map[string]Waiver{"u": w}), "u"), "a waiver filed under another node is rejected")

	toShip := w
	toShip.shipmentID = "145-S"
	assert.Equal(t, StatusError, statusOf(t, run(map[string]Waiver{"t": toShip}), "t"))
	toPrincipal := w
	toPrincipal.principal = "root"
	assert.Equal(t, StatusError, statusOf(t, run(map[string]Waiver{"t": toPrincipal}), "t"), "the seal covers the principal too")
	assert.Equal(t, StatusError, statusOf(t, run(map[string]Waiver{"t": {}}), "t"), "a zero waiver is never valid")
	assert.Equal(t, "t", w.NodeID())
	assert.Equal(t, "144-S", w.ShipmentID())
	assert.Equal(t, testHead, w.HeadCommit())
	assert.Equal(t, "alice", w.Principal())
	assert.NotEmpty(t, w.ID())
}

func TestRemediation_BlockingSemanticsCannotBeLaundered(t *testing.T) {
	t.Run("report terminal over blocking non-terminal", func(t *testing.T) {
		term := pnode("term", true)
		term.OnUnmet = OnUnmetReport
		d := decl([]Node{pnode("base", false), term}, Edge{"term", "base"})
		rep := evalOne(t, d, nil)
		n, _ := rep.Node("term")
		assert.Equal(t, StatusUnmet, n.Status)
		assert.Equal(t, OnUnmetBlock, n.OnUnmet, "the blocking upstream propagates")
		ru := rep.Rollup()
		assert.Equal(t, RollupBlocked, ru.State)
		assert.Equal(t, []string{"term"}, ru.Unmet)
		assert.Empty(t, ru.Advisory)
	})
	t.Run("report terminal over report upstream stays advisory", func(t *testing.T) {
		up := pnode("up", true)
		up.OnUnmet = OnUnmetReport
		term := pnode("term", true)
		term.OnUnmet = OnUnmetReport
		d := decl([]Node{up, term, pnode("req", true)}, Edge{"term", "up"})
		rep := evalOne(t, d, map[string]EvidenceRecord{"req": goodRec(t, "req")})
		ru := rep.Rollup()
		assert.Equal(t, RollupReady, ru.State)
		assert.Equal(t, []string{"term", "up"}, ru.Advisory)
	})
	t.Run("report terminal over blocking terminal upstream", func(t *testing.T) {
		term := pnode("term", true)
		term.OnUnmet = OnUnmetReport
		d := decl([]Node{pnode("up", true), term}, Edge{"term", "up"})
		rep := evalOne(t, d, nil)
		ru := rep.Rollup()
		assert.Equal(t, RollupBlocked, ru.State)
		assert.ElementsMatch(t, []string{"up", "term"}, ru.Unmet)
	})
	t.Run("non-terminal report is rejected", func(t *testing.T) {
		n := pnode("base", false)
		n.OnUnmet = OnUnmetReport
		d := decl([]Node{n, pnode("t", true)}, Edge{"t", "base"})
		err := d.Validate()
		require.ErrorIs(t, err, ErrInvalidDeclaration)
		assert.Contains(t, err.Error(), "on_unmet")
	})
	t.Run("composed report terminal cannot weaken a base blocker", func(t *testing.T) {
		base := decl([]Node{pnode("b", false), pnode("bt", true)}, Edge{"bt", "b"})
		c := pnode("c", true)
		c.OnUnmet = OnUnmetReport
		merged, err := Compose(base, Contribution{Source: "s11", Nodes: []Node{c}, Edges: []Edge{{"c", "b"}}})
		require.NoError(t, err)
		rep := evalOne(t, merged, nil)
		ru := rep.Rollup()
		assert.Equal(t, RollupBlocked, ru.State)
		assert.ElementsMatch(t, []string{"bt", "c"}, ru.Unmet)
		assert.Empty(t, ru.Advisory)
	})
}

func TestRemediation_ComposeRecordsSourceAndRejectsDuplicates(t *testing.T) {
	base := decl([]Node{pnode("bt", true)})
	merged, err := Compose(base, Contribution{Source: "s11", Nodes: []Node{pnode("c1", true)}}, Contribution{Source: "s12", Nodes: []Node{pnode("c2", true)}})
	require.NoError(t, err)
	src := map[string]string{}
	for _, n := range merged.Nodes {
		src[n.ID] = n.Source
	}
	assert.Equal(t, map[string]string{"bt": "", "c1": "s11", "c2": "s12"}, src)
	assert.Empty(t, base.Nodes[0].Source)

	_, err = Compose(base, Contribution{Source: "s11", Nodes: []Node{pnode("c1", true)}}, Contribution{Source: "s11", Nodes: []Node{pnode("c2", true)}})
	assert.ErrorIs(t, err, ErrInvalidDeclaration, "the same source twice is rejected")
	preset := pnode("c1", true)
	preset.Source = "other"
	_, err = Compose(base, Contribution{Source: "s11", Nodes: []Node{preset}})
	assert.ErrorIs(t, err, ErrInvalidDeclaration)
}

func TestRemediation_WaiverTokensNotBurnedBeforeTheyCanTakeEffect(t *testing.T) {
	newGate := func(f *fixture, nodes []Node) *Gate {
		g, err := NewGate(decl(nodes), ModeEnforce, f.svc)
		require.NoError(t, err)
		return g
	}
	empty := func(f *fixture) {
		recs, err := f.ledger.Records(nil)
		require.NoError(t, err)
		assert.Empty(t, recs, "no nonce was consumed")
		_, ok, _ := f.anchors.Load()
		assert.False(t, ok)
	}
	t.Run("non-applicable node", func(t *testing.T) {
		f := newFixture(t)
		n := pnode("t", true)
		n.Applicability = &Applicability{Requires: map[string][]string{"surface": {"mcp"}}}
		g := newGate(f, []Node{n, pnode("other", true)})
		dec, err := g.Check(context.Background(), GateInput{Shipment: ship(), WaiverTokens: [][]byte{signClaims(f.alice, f.claims())}})
		require.NoError(t, err)
		require.Len(t, dec.Denials, 1)
		empty(f)
	})
	t.Run("invalid input", func(t *testing.T) {
		f := newFixture(t)
		g := newGate(f, []Node{pnode("t", true)})
		_, err := g.Check(context.Background(), GateInput{Shipment: ship(), Evidence: map[string]EvidenceRecord{"ghost": {}}, WaiverTokens: [][]byte{signClaims(f.alice, f.claims())}})
		assert.ErrorIs(t, err, ErrInvalidInput)
		_, err = g.Check(context.Background(), GateInput{Shipment: ship(), Failures: map[string]string{"ghost": "x"}, WaiverTokens: [][]byte{signClaims(f.alice, f.claims())}})
		assert.ErrorIs(t, err, ErrInvalidInput)
		empty(f)
	})
	t.Run("second token for one node", func(t *testing.T) {
		f := newFixture(t)
		g := newGate(f, []Node{pnode("t", true)})
		dec, err := g.Check(context.Background(), GateInput{Shipment: ship(), WaiverTokens: [][]byte{signClaims(f.alice, f.claims()), signClaims(f.alice, f.claims())}})
		require.NoError(t, err)
		assert.True(t, dec.ReviewReady)
		require.Len(t, dec.Denials, 1)
		assert.Equal(t, 1, dec.Denials[0].Index)
		recs, err := f.ledger.Records(nil)
		require.NoError(t, err)
		assert.Len(t, recs, 1, "only the token that took effect was consumed")
	})
	t.Run("not-applicable kind is denied until implemented", func(t *testing.T) {
		f := newFixture(t)
		c := f.claims()
		c.Kind = "not-applicable"
		d := decl([]Node{pnode("t", true)})
		_, err := f.svc.Accept(signClaims(f.alice, c), ship(), lookupFor(d))
		require.ErrorIs(t, err, ErrWaiverDenied)
		assert.Contains(t, err.Error(), "not implemented")
		empty(f)
	})
	t.Run("clock is re-read under the ledger lock", func(t *testing.T) {
		f := newFixture(t)
		var calls atomic.Int32
		svc, err := NewWaiverService(f.trust, f.ledger, f.anchors, func() time.Time {
			if calls.Add(1) == 1 {
				return f.now
			}
			return f.now.Add(2 * time.Hour) // the claims expire after one hour
		})
		require.NoError(t, err)
		d := decl([]Node{pnode("t", true)})
		_, err = svc.Accept(signClaims(f.alice, f.claims()), ship(), lookupFor(d))
		require.ErrorIs(t, err, ErrWaiverDenied)
		assert.Contains(t, err.Error(), "expired")
		empty(f)
	})
}

func TestRemediation_EvaluatorInputAndCacheHardening(t *testing.T) {
	d := decl([]Node{pnode("t", true)})
	ev := mustEval(t, d)
	_, err := ev.Evaluate(Input{Shipment: ship(), Failures: map[string]string{"t": ""}})
	assert.ErrorIs(t, err, ErrInvalidInput, "a failure without a reason is rejected")
	_, err = ev.Evaluate(Input{Shipment: ship(), Failures: map[string]string{"ghost": "x"}})
	assert.ErrorIs(t, err, ErrInvalidInput, "an undeclared failure id is rejected like evidence and waivers")

	rep, err := ev.Evaluate(Input{Shipment: ship(), Failures: map[string]string{"t": "boom"}})
	require.NoError(t, err)
	assert.Equal(t, StatusError, statusOf(t, rep, "t"))
	rep, err = ev.Evaluate(Input{Shipment: ship()})
	require.NoError(t, err)
	assert.Equal(t, StatusUnmet, statusOf(t, rep, "t"), "the presence of a failure is part of the cache key")

	other := ship()
	other.HeadCommit = otherHead
	_, err = ev.Evaluate(Input{Shipment: other})
	require.NoError(t, err)
	rep, err = ev.Evaluate(Input{Shipment: ship()})
	require.NoError(t, err)
	assert.Equal(t, 1, rep.CacheHits, "alternating shipments do not evict each other")
}

func TestRemediation_ReportJSONShapeIsSnakeCaseAndNonNil(t *testing.T) {
	d := decl([]Node{pnode("t", true)})
	rep := evalOne(t, d, map[string]EvidenceRecord{"t": goodRec(t, "t")})
	b, err := json.Marshal(rep)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(b, &m))
	for _, k := range []string{"shipment_id", "head_commit", "nodes", "computed", "cache_hits"} {
		assert.Contains(t, m, k)
	}
	ru, err := json.Marshal(rep.Rollup())
	require.NoError(t, err)
	var rm map[string]any
	require.NoError(t, json.Unmarshal(ru, &rm))
	for _, k := range []string{"state", "applicable_terminals", "unmet", "unverifiable", "advisory", "reason"} {
		assert.Contains(t, rm, k)
	}
	assert.Equal(t, []any{}, rm["unmet"], "slices are non-nil so consumers see [] not null")
	node := m["nodes"].([]any)[0].(map[string]any)
	for _, k := range []string{"id", "family", "terminal", "on_unmet", "status", "reason", "key"} {
		assert.Contains(t, node, k)
	}
}
