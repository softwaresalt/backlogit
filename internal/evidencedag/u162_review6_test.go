package evidencedag

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/faultline"
	"github.com/softwaresalt/backlogit/internal/redhonesty"
)

// ambiguousRedGreen returns an otherwise valid red_green artifact whose
// payload repeats a key under Unicode folding. The shared S4 decoder keeps
// the folded variant, so the artifact validates unless the duplicate is caught.
func ambiguousRedGreen(t *testing.T, variant string) faultline.EvidenceArtifact {
	t.Helper()
	a := rgReport(t, true).Artifact
	p := string(a.VerifiedEvidence)
	switch variant {
	case "long-s":
		require.Contains(t, p, `"baseline_result":"fail"`)
		p = strings.Replace(p, `"baseline_result":"fail"`, `"baseline_result":"pass","ba`+"\u017f"+`eline_result":"fail"`, 1)
	case "kelvin":
		require.Contains(t, p, `"package":"internal/x"`)
		p = strings.Replace(p, `"package":"internal/x"`, `"package":"internal/x","pac`+"\u212a"+`age":"internal/x"`, 1)
	}
	a.VerifiedEvidence = json.RawMessage(p)
	return a
}

func TestReview6_FoldDuplicateEvidenceRejectedInMemory(t *testing.T) {
	for _, variant := range []string{"long-s", "kelvin"} {
		t.Run(variant, func(t *testing.T) {
			a := ambiguousRedGreen(t, variant)

			_, err := NewEvidenceRecord(ship(), "red", a)
			require.Error(t, err, "NewEvidenceRecord must refuse fold-duplicate payload keys")
			assert.Contains(t, err.Error(), "duplicate key")

			// A record forged around the ambiguous bytes with a matching digest.
			dig, err := artifactDigest(a)
			if err != nil {
				dig = strings.Repeat("0", 64) // the S4 canonicalizer itself refuses this payload
			}
			raw, err := json.Marshal(a)
			require.NoError(t, err)
			forged := EvidenceRecord{SchemaVersion: RecordSchemaVersion, ShipmentID: ship().ID, NodeID: "red",
				HeadCommit: testHead, Digest: dig, Artifact: raw}
			d := decl([]Node{rgNode("red")})
			rep, err := mustEval(t, d).Evaluate(Input{Shipment: ship(), Evidence: map[string]EvidenceRecord{"red": forged}})
			require.NoError(t, err)
			n, _ := rep.Node("red")
			assert.Equal(t, StatusError, n.Status)
			assert.Contains(t, n.Reason, "duplicate key")
		})
	}
}

func TestReview6_FoldDuplicateExecutorOutputFailsClosed(t *testing.T) {
	for _, variant := range []string{"long-s", "kelvin"} {
		t.Run(variant, func(t *testing.T) {
			a := ambiguousRedGreen(t, variant)
			d := decl([]Node{rgNode("red")})
			fx, err := NewFuncExecutor(redhonesty.ValidatorIdentity, func(context.Context, ExecRequest) (faultline.EvidenceArtifact, error) { return a, nil })
			require.NoError(t, err)
			res, err := RunExecutors(context.Background(), d, ship(), map[string]NodeExecutor{"red": fx})
			require.NoError(t, err)
			assert.Empty(t, res.Evidence)
			assert.Contains(t, res.Failures["red"], "duplicate key")
		})
	}
}

func TestReview6_ExecutorsGetPrivateRequestData(t *testing.T) {
	appl := func() *Applicability {
		return &Applicability{Requires: map[string][]string{"surface": {"cli"}}}
	}
	mk := func(id string) Node {
		n := pnode(id, true)
		n.Applicability = appl()
		return n
	}
	d := decl([]Node{mk("a"), mk("b"), mk("c")})
	s := ship()
	callerNodes := []*Applicability{d.Nodes[0].Applicability, d.Nodes[1].Applicability, d.Nodes[2].Applicability}

	release := make(chan struct{})
	done := make(chan struct{})
	mutate := func(req ExecRequest) {
		req.Shipment.Attributes["surface"] = "web"
		req.Shipment.Attributes["extra"] = "x"
		req.Node.Applicability.Always = true
		req.Node.Applicability.Requires["surface"][0] = "web"
		req.Node.Applicability.Requires["added"] = []string{"y"}
	}
	a := func(ctx context.Context, req ExecRequest) (faultline.EvidenceArtifact, error) {
		mutate(req)
		return parityArtifact(testTaskA, testHead, faultline.StatusPass), nil
	}
	// b ignores its context, times out, and mutates its request after release.
	b := func(ctx context.Context, req ExecRequest) (faultline.EvidenceArtifact, error) {
		defer close(done)
		<-release
		mutate(req)
		return faultline.EvidenceArtifact{}, nil
	}
	cc := func(ctx context.Context, req ExecRequest) (faultline.EvidenceArtifact, error) {
		assert.Equal(t, "cli", req.Shipment.Attributes["surface"], "a prior callback must not alter this request")
		assert.Equal(t, []string{"cli"}, req.Node.Applicability.Requires["surface"])
		return parityArtifactFor("c", testTaskA, testHead, faultline.StatusPass), nil
	}
	bind := map[string]NodeExecutor{}
	for id, fn := range map[string]ExecFunc{"a": a, "b": b, "c": cc} {
		x, err := NewFuncExecutor(parityVal, fn)
		require.NoError(t, err)
		bind[id] = x
	}
	res, err := RunExecutorsWith(context.Background(), d, s, bind, ExecOptions{NodeTimeout: 200 * time.Millisecond})
	require.NoError(t, err)
	assert.Contains(t, res.Failures["b"], "timed out")
	close(release)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Log("abandoned executor never ran: its node was skipped")
	}
	time.Sleep(20 * time.Millisecond)

	assert.Equal(t, map[string]string{"surface": "cli"}, s.Attributes)
	for i, n := range d.Nodes {
		assert.Same(t, callerNodes[i], n.Applicability)
		assert.False(t, n.Applicability.Always)
		assert.Equal(t, map[string][]string{"surface": {"cli"}}, n.Applicability.Requires)
	}
}

func TestReview6_ReportAndCacheDoNotAlias(t *testing.T) {
	f := newFixture(t)
	d := decl([]Node{pnode("base", false), pnode("t", true)}, Edge{"t", "base"})
	g, err := NewGate(d, ModeEnforce, f.svc)
	require.NoError(t, err)
	dec, err := g.Check(context.Background(), GateInput{Shipment: ship(), WaiverTokens: [][]byte{signClaims(f.alice, f.claims())}})
	require.NoError(t, err)
	n, _ := dec.Report.Node("t")
	require.NotEmpty(t, n.MaskedUpstreams)
	n.MaskedUpstreams[0] = "corrupted"
	for _, ce := range g.eval.cache {
		for _, m := range ce.result.MaskedUpstreams {
			assert.NotEqual(t, "corrupted", m)
		}
	}
}

func TestReview6_DeclarationCopiesAreIsolated(t *testing.T) {
	d := decl([]Node{pnode("a", true)})
	d.Nodes[0].Applicability = &Applicability{Requires: map[string][]string{"surface": {"cli"}}}
	g, err := NewGate(d, ModeEnforce, nil)
	require.NoError(t, err)
	d.Nodes[0].Applicability.Requires["surface"][0] = "web"
	got := g.Declaration()
	assert.Equal(t, []string{"cli"}, got.Nodes[0].Applicability.Requires["surface"])
	got.Nodes[0].Applicability.Requires["surface"][0] = "web"
	assert.Equal(t, []string{"cli"}, g.Declaration().Nodes[0].Applicability.Requires["surface"])
}
