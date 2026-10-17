package evidencedag

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/faultline"
)

func TestU162_003_TrustParseValid(t *testing.T) {
	a := newSigner("k1")
	tr, err := ParseTrust(trustDoc([]map[string]any{a.keyJSON("alice", "human")}, map[string][]string{"alice": {"parity"}}))
	require.NoError(t, err)
	require.NotNil(t, tr)
	k, ok := tr.keys["k1"]
	require.True(t, ok)
	assert.Equal(t, "alice", k.Principal)
	assert.Equal(t, KeyKindHuman, k.Kind)
	assert.True(t, tr.authorized("alice", FamilyParity))
	assert.False(t, tr.authorized("alice", FamilyDocsAudit))
	assert.False(t, tr.authorized("nobody", FamilyParity), "denied by default")
}

func TestU162_003_TrustParseRefusals(t *testing.T) {
	a := newSigner("k1")
	good := func() map[string]any {
		return map[string]any{"schema_version": 1, "keys": []map[string]any{a.keyJSON("alice", "human")}, "policy": []map[string]any{{"principal": "alice", "families": []string{"parity"}}}}
	}
	enc := func(m map[string]any) []byte { b, _ := json.Marshal(m); return b }
	with := func(fn func(m map[string]any)) []byte { m := good(); fn(m); return enc(m) }
	keyWith := func(fn func(k map[string]any)) []byte {
		return with(func(m map[string]any) {
			k := a.keyJSON("alice", "human")
			fn(k)
			m["keys"] = []map[string]any{k}
		})
	}
	cases := map[string][]byte{
		"version 2":       with(func(m map[string]any) { m["schema_version"] = 2 }),
		"missing version": with(func(m map[string]any) { delete(m, "schema_version") }),
		"unknown field":   with(func(m map[string]any) { m["extra"] = 1 }),
		"duplicate key id": with(func(m map[string]any) {
			m["keys"] = []map[string]any{a.keyJSON("alice", "human"), a.keyJSON("bob", "human")}
		}),
		"bad kind":         keyWith(func(k map[string]any) { k["kind"] = "root" }),
		"short public key": keyWith(func(k map[string]any) { k["public_key"] = base64.StdEncoding.EncodeToString([]byte("short")) }),
		"bad base64":       keyWith(func(k map[string]any) { k["public_key"] = "!!!" }),
		"empty principal":  keyWith(func(k map[string]any) { k["principal"] = "" }),
		"empty key id":     keyWith(func(k map[string]any) { k["key_id"] = "" }),
		"policy without key": with(func(m map[string]any) {
			m["policy"] = []map[string]any{{"principal": "ghost", "families": []string{"parity"}}}
		}),
		"policy unknown family": with(func(m map[string]any) {
			m["policy"] = []map[string]any{{"principal": "alice", "families": []string{"bogus"}}}
		}),
		"policy empty families": with(func(m map[string]any) { m["policy"] = []map[string]any{{"principal": "alice", "families": []string{}}} }),
		"duplicate policy": with(func(m map[string]any) {
			m["policy"] = []map[string]any{{"principal": "alice", "families": []string{"parity"}}, {"principal": "alice", "families": []string{"docs-audit"}}}
		}),
		"duplicate json key": []byte(`{"schema_version":1,"schema_version":1,"keys":[],"policy":[]}`),
		"trailing data":      append(enc(good()), []byte(" {}")...),
		"oversize":           []byte(strings.Repeat(" ", MaxTrustBytes+1)),
		"empty":              nil,
		"keys missing":       with(func(m map[string]any) { delete(m, "keys") }),
		"policy missing":     with(func(m map[string]any) { delete(m, "policy") }),
	}
	for name, doc := range cases {
		t.Run(name, func(t *testing.T) {
			tr, err := ParseTrust(doc)
			require.Error(t, err)
			assert.Nil(t, tr)
			assert.ErrorIs(t, err, ErrTrustInvalid)
		})
	}
	_, err := ParseTrust(with(func(m map[string]any) { m["schema_version"] = 2 }))
	assert.ErrorIs(t, err, ErrUnsupportedVersion)
}

func TestU162_003_LoadTrustLocationAndFileRules(t *testing.T) {
	a := newSigner("k1")
	doc := trustDoc([]map[string]any{a.keyJSON("alice", "human")}, map[string][]string{"alice": {"parity"}})
	cfgDir := t.TempDir()
	evidenceRoot := t.TempDir()
	good := filepath.Join(cfgDir, "trust.json")
	require.NoError(t, os.WriteFile(good, doc, 0o600))
	tr, err := LoadTrust(good, evidenceRoot)
	require.NoError(t, err)
	assert.NotNil(t, tr)

	inside := filepath.Join(evidenceRoot, "sub", "trust.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(inside), 0o755))
	require.NoError(t, os.WriteFile(inside, doc, 0o600))
	_, err = LoadTrust(inside, evidenceRoot)
	assert.ErrorIs(t, err, ErrTrustInvalid, "trust inside agent-writable evidence is refused")
	_, err = LoadTrust(evidenceRoot, evidenceRoot)
	assert.Error(t, err)

	_, err = LoadTrust("trust.json")
	assert.Error(t, err, "relative path refused")
	_, err = LoadTrust(filepath.Join(cfgDir, "missing.json"))
	assert.Error(t, err)
	_, err = LoadTrust(cfgDir)
	assert.Error(t, err, "directory refused")
	huge := filepath.Join(cfgDir, "huge.json")
	require.NoError(t, os.WriteFile(huge, []byte(strings.Repeat(" ", MaxTrustBytes+5)), 0o600))
	_, err = LoadTrust(huge)
	assert.Error(t, err)
	_, err = LoadTrust(good, "")
	assert.Error(t, err, "empty forbidden root is a configuration error, not a wildcard")

	link := filepath.Join(cfgDir, "link.json")
	if err := os.Symlink(good, link); err == nil {
		_, err = LoadTrust(link, evidenceRoot)
		assert.Error(t, err, "symlinked trust file refused")
	}
	linkDir := filepath.Join(t.TempDir(), "linkdir")
	if err := os.Symlink(evidenceRoot, linkDir); err == nil {
		_, err = LoadTrust(filepath.Join(linkDir, "sub", "trust.json"), evidenceRoot)
		assert.Error(t, err, "a symlink that resolves into the evidence root is refused")
	}
}

func TestU162_003_VerifierUsesTrustedPublicKeyOnly(t *testing.T) {
	// A token carrying its own public key material is not a field of the wire
	// format at all: unknown fields are refused.
	f := newFixture(t)
	d := decl([]Node{pnode("t", true)})
	var w waiverToken
	require.NoError(t, json.Unmarshal(signClaims(f.rogue, f.claims()), &w))
	b, _ := json.Marshal(map[string]any{"schema_version": 1, "key_id": "k-alice", "payload": w.Payload, "signature": w.Signature, "public_key": base64.StdEncoding.EncodeToString(f.rogue.pub)})
	_, err := f.svc.Accept(b, ship(), lookupFor(d))
	assert.ErrorIs(t, err, ErrWaiverDenied)
	var _ ed25519.PublicKey = f.rogue.pub
}

func TestU162_003_ComposeContributesPolicyNodesIntoSingleGate(t *testing.T) {
	base := decl([]Node{pnode("base", false), pnode("t", true)}, Edge{"t", "base"})
	policy := pnode("s11-policy", true)
	policy.Family = FamilyPlanTaskSoundness
	c := Contribution{Source: "s11", Nodes: []Node{policy}}
	merged, err := Compose(base, c)
	require.NoError(t, err)
	require.NoError(t, merged.Validate())
	assert.Len(t, merged.Nodes, 3)
	assert.Len(t, base.Nodes, 2, "base is not mutated")

	f := newFixture(t)
	g, err := NewGate(merged, ModeEnforce, f.svc)
	require.NoError(t, err)
	ev := map[string]EvidenceRecord{"base": goodRec(t, "base"), "t": goodRec(t, "t")}
	dec, err := g.Check(t.Context(), GateInput{Shipment: ship(), Evidence: ev})
	require.NoError(t, err)
	assert.False(t, dec.ReviewReady, "a contributed policy terminal node gates the same review-ready decision")
	n, _ := dec.Report.Node("s11-policy")
	assert.NotEqual(t, StatusVerified, n.Status)
}

func TestU162_003_ComposeRefusals(t *testing.T) {
	base := decl([]Node{pnode("base", false), pnode("t", true)}, Edge{"t", "base"})
	other := pnode("p", true)
	cases := map[string][]Contribution{
		"duplicate id":        {{Source: "s11", Nodes: []Node{pnode("t", true)}}},
		"edge from base node": {{Source: "s11", Nodes: []Node{other}, Edges: []Edge{{"t", "p"}}}},
		"edge to undeclared":  {{Source: "s11", Nodes: []Node{other}, Edges: []Edge{{"p", "ghost"}}}},
		"bad source":          {{Source: "", Nodes: []Node{other}}},
		"empty contribution":  {{Source: "s11"}},
		"two with same ids":   {{Source: "a", Nodes: []Node{other}}, {Source: "b", Nodes: []Node{other}}},
		"invalid node":        {{Source: "s11", Nodes: []Node{{ID: "p"}}}},
	}
	for name, cs := range cases {
		t.Run(name, func(t *testing.T) {
			d, err := Compose(base, cs...)
			require.Error(t, err)
			assert.Nil(t, d)
		})
	}
	_, err := Compose(nil, Contribution{Source: "s11", Nodes: []Node{other}})
	assert.Error(t, err)
	d, err := Compose(base)
	require.NoError(t, err)
	assert.Equal(t, base, d)
}

func TestU162_003_ComposeCannotLeaveContributedNodeOrphaned(t *testing.T) {
	base := decl([]Node{pnode("t", true)})
	orphan := pnode("helper", false)
	_, err := Compose(base, Contribution{Source: "s11", Nodes: []Node{orphan}})
	assert.ErrorIs(t, err, ErrInvalidDeclaration, "a non-terminal contributed node nothing depends on is unreachable")
	merged, err := Compose(base, Contribution{Source: "s11", Nodes: []Node{orphan, pnode("pol", true)}, Edges: []Edge{{"pol", "helper"}}})
	require.NoError(t, err)
	assert.Len(t, merged.Nodes, 3)
}

func renderFixture(t *testing.T) (*Declaration, Report) {
	t.Helper()
	d := decl([]Node{pnode("z-term", true), pnode("a-base", false), pnode("m-term", true)}, Edge{"z-term", "a-base"})
	rep, err := mustEval(t, d).Evaluate(Input{Shipment: ship(), Evidence: map[string]EvidenceRecord{"a-base": goodRec(t, "a-base"), "z-term": goodRec(t, "z-term")}})
	require.NoError(t, err)
	return d, rep
}

func TestU162_003_TextReportDeterministicAndBounded(t *testing.T) {
	_, rep := renderFixture(t)
	txt := rep.Text()
	assert.Equal(t, txt, rep.Text())
	assert.Contains(t, txt, "144-S")
	assert.Contains(t, txt, testHead)
	assert.Contains(t, txt, "rollup: blocked")
	section := txt[strings.Index(txt, "nodes:"):]
	ia, im, iz := strings.Index(section, "a-base"), strings.Index(section, "m-term"), strings.Index(section, "z-term")
	assert.True(t, ia >= 0 && ia < im && im < iz, "nodes listed in ID order")
	assert.Contains(t, txt, "m-term")
	assert.LessOrEqual(t, len(txt), MaxReportBytes)

	long := strings.Repeat("x", MaxReportBytes*2)
	out := boundOutput(long)
	assert.LessOrEqual(t, len(out), MaxReportBytes)
	assert.Contains(t, out, "truncated")
	assert.Equal(t, "short", boundOutput("short"))
}

func TestU162_003_DOTOutput(t *testing.T) {
	d, rep := renderFixture(t)
	dot := d.DOT(&rep)
	assert.Equal(t, dot, d.DOT(&rep))
	assert.True(t, strings.HasPrefix(dot, "digraph evidence {"))
	assert.True(t, strings.HasSuffix(strings.TrimSpace(dot), "}"))
	assert.Contains(t, dot, `"z-term" -> "a-base"`)
	assert.Contains(t, dot, "verified")
	assert.Contains(t, dot, "unmet")
	plain := d.DOT(nil)
	assert.NotContains(t, plain, "verified")
	assert.LessOrEqual(t, len(dot), MaxReportBytes)

	hostile := &Declaration{SchemaVersion: 1, Nodes: []Node{{ID: `a"]; evil [x="`, Terminal: true}}}
	out := hostile.DOT(nil)
	assert.NotContains(t, out, `a"]; evil`)
}

func TestU162_003_VersioningRefusal(t *testing.T) {
	assert.Equal(t, 1, SchemaVersion)
	d := decl([]Node{pnode("t", true)})
	d.SchemaVersion = 2
	assert.ErrorIs(t, d.Validate(), ErrUnsupportedVersion)
	_, err := NewEvaluator(d)
	assert.ErrorIs(t, err, ErrUnsupportedVersion)
	_, err = NewGate(d, ModeEnforce, nil)
	assert.ErrorIs(t, err, ErrUnsupportedVersion)
	_ = faultline.EvidenceSchemaVersion
}
