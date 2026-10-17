package evidencedag

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/faultline"
)

type testSigner struct {
	id   string
	priv ed25519.PrivateKey
	pub  ed25519.PublicKey
}

func newSigner(id string) testSigner {
	seed := sha256.Sum256([]byte("seed:" + id))
	priv := ed25519.NewKeyFromSeed(seed[:])
	return testSigner{id: id, priv: priv, pub: priv.Public().(ed25519.PublicKey)}
}

func (s testSigner) keyJSON(principal string, kind string) map[string]any {
	return map[string]any{"key_id": s.id, "principal": principal, "kind": kind, "public_key": base64.StdEncoding.EncodeToString(s.pub)}
}

func trustDoc(keys []map[string]any, policy map[string][]string) []byte {
	pol := []map[string]any{}
	for p, fams := range policy {
		pol = append(pol, map[string]any{"principal": p, "families": fams})
	}
	b, _ := json.Marshal(map[string]any{"schema_version": 1, "keys": keys, "policy": pol})
	return b
}

func signClaims(s testSigner, c WaiverClaims) []byte {
	cj, _ := json.Marshal(c)
	sig := ed25519.Sign(s.priv, append([]byte(waiverDomain), cj...))
	tok, _ := json.Marshal(waiverToken{
		SchemaVersion: 1, KeyID: s.id,
		Payload:   base64.StdEncoding.EncodeToString(cj),
		Signature: base64.StdEncoding.EncodeToString(sig),
	})
	return tok
}

type fixture struct {
	t      *testing.T
	now    time.Time
	alice  testSigner
	bot    testSigner
	carol  testSigner
	rogue  testSigner
	trust  *Trust
	ledger *Ledger
	svc    *WaiverService
	dir    string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{t: t, now: time.Unix(1_800_000_000, 0), alice: newSigner("k-alice"), bot: newSigner("k-bot"), carol: newSigner("k-carol"), rogue: newSigner("k-rogue"), dir: t.TempDir()}
	doc := trustDoc(
		[]map[string]any{f.alice.keyJSON("alice", "human"), f.bot.keyJSON("bot", "automation"), f.carol.keyJSON("carol", "human")},
		map[string][]string{"alice": {"parity", "red-test-honesty"}, "bot": {"parity"}, "carol": {"docs-audit"}},
	)
	tr, err := ParseTrust(doc)
	require.NoError(t, err)
	f.trust = tr
	f.ledger, err = OpenLedger(filepath.Join(f.dir, "waivers.jsonl"))
	require.NoError(t, err)
	f.svc, err = NewWaiverService(tr, f.ledger, func() time.Time { return f.now })
	require.NoError(t, err)
	return f
}

var nonceCounter int

func (f *fixture) claims() WaiverClaims {
	nonceCounter++
	return WaiverClaims{
		Nonce: fmt.Sprintf("%032x", nonceCounter), ShipmentID: "144-S", NodeID: "t", HeadCommit: testHead,
		Principal: "alice", Kind: "waiver", Reason: "genuinely not applicable: docs-only change",
		NotBefore: f.now.Unix() - 60, ExpiresAt: f.now.Unix() + 3600,
	}
}

func (f *fixture) gate(mode Mode, svc *WaiverService) *Gate {
	f.t.Helper()
	g, err := NewGate(decl([]Node{pnode("t", true)}), mode, svc)
	require.NoError(f.t, err)
	return g
}

func lookupFor(d *Declaration) func(string) (Node, bool) {
	return func(id string) (Node, bool) {
		for _, n := range d.Nodes {
			if n.ID == id {
				return n, true
			}
		}
		return Node{}, false
	}
}

func TestU162_003_UnmetTerminalCannotBeReviewReady(t *testing.T) {
	f := newFixture(t)
	g := f.gate(ModeEnforce, f.svc)
	dec, err := g.Check(context.Background(), GateInput{Shipment: ship()})
	require.NoError(t, err)
	assert.False(t, dec.ReviewReady)
	assert.False(t, dec.Allowed)
	assert.Equal(t, RollupBlocked, dec.Rollup.State)
	assert.Equal(t, []string{"t"}, dec.Rollup.Unmet)
}

func TestU162_003_VerifiedTerminalIsReviewReady(t *testing.T) {
	f := newFixture(t)
	g := f.gate(ModeEnforce, f.svc)
	dec, err := g.Check(context.Background(), GateInput{Shipment: ship(), Evidence: map[string]EvidenceRecord{"t": goodRec(t, "t")}})
	require.NoError(t, err)
	assert.True(t, dec.ReviewReady)
	assert.True(t, dec.Allowed)
	assert.Equal(t, ModeEnforce, dec.Mode)
}

func TestU162_003_EmptyApplicableTerminalSetNeverReady(t *testing.T) {
	f := newFixture(t)
	n := pnode("t", true)
	n.Applicability = &Applicability{Requires: map[string][]string{"surface": {"mcp"}}}
	g, err := NewGate(decl([]Node{n}), ModeEnforce, f.svc)
	require.NoError(t, err)
	dec, err := g.Check(context.Background(), GateInput{Shipment: ship()})
	require.NoError(t, err)
	assert.False(t, dec.ReviewReady)
	assert.False(t, dec.Allowed)
	assert.Equal(t, RollupUnverifiable, dec.Rollup.State)
}

func TestU162_003_ReportOnlyModeReportsButNeverHidesVerdict(t *testing.T) {
	f := newFixture(t)
	g := f.gate(ModeReportOnly, f.svc)
	dec, err := g.Check(context.Background(), GateInput{Shipment: ship()})
	require.NoError(t, err)
	assert.False(t, dec.ReviewReady, "the truthful verdict is preserved")
	assert.True(t, dec.Allowed, "report-only never blocks")
	assert.Equal(t, RollupBlocked, dec.Rollup.State)
	assert.Contains(t, dec.Reason, "report-only")
}

func TestU162_003_GateConstructionAndContext(t *testing.T) {
	f := newFixture(t)
	d := decl([]Node{pnode("t", true)})
	for _, m := range []Mode{"", "off", "ENFORCE"} {
		_, err := NewGate(d, m, nil)
		assert.Error(t, err, string(m))
	}
	_, err := NewGate(decl([]Node{pnode("a", false)}), ModeEnforce, nil)
	assert.ErrorIs(t, err, ErrInvalidDeclaration)
	g := f.gate(ModeEnforce, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = g.Check(ctx, GateInput{Shipment: ship()})
	assert.ErrorIs(t, err, context.Canceled)
	_, err = g.Check(context.Background(), GateInput{Shipment: Shipment{}})
	assert.ErrorIs(t, err, ErrInvalidInput)
	var nilGate *Gate
	_, err = nilGate.Check(context.Background(), GateInput{Shipment: ship()})
	assert.Error(t, err)
}

func TestU162_003_ValidWaiverRecordsAuthorizedAuthenticatedAuthority(t *testing.T) {
	f := newFixture(t)
	g := f.gate(ModeEnforce, f.svc)
	tok := signClaims(f.alice, f.claims())
	dec, err := g.Check(context.Background(), GateInput{Shipment: ship(), WaiverTokens: [][]byte{tok}})
	require.NoError(t, err)
	assert.Empty(t, dec.Denials)
	assert.True(t, dec.ReviewReady)
	n, _ := dec.Report.Node("t")
	assert.Equal(t, StatusWaived, n.Status)
	assert.Equal(t, "alice", n.Waiver)

	recs, err := f.ledger.Records(nil)
	require.NoError(t, err)
	require.Len(t, recs, 1)
	assert.Equal(t, "alice", recs[0].Principal)
	assert.Equal(t, "k-alice", recs[0].KeyID)
	assert.Equal(t, "parity", recs[0].Family)
	assert.Equal(t, "t", recs[0].NodeID)
	assert.Equal(t, "144-S", recs[0].ShipmentID)

	// Re-checking with the identical token is idempotent: no second record.
	dec, err = g.Check(context.Background(), GateInput{Shipment: ship(), WaiverTokens: [][]byte{tok}})
	require.NoError(t, err)
	assert.True(t, dec.ReviewReady)
	recs, err = f.ledger.Records(nil)
	require.NoError(t, err)
	assert.Len(t, recs, 1)
}

func TestU162_003_WaiverDenialPaths(t *testing.T) {
	type tc struct {
		name string
		mk   func(f *fixture) []byte
		svc  bool // false: gate built without a waiver service
	}
	mod := func(fn func(c *WaiverClaims)) func(f *fixture) []byte {
		return func(f *fixture) []byte { c := f.claims(); fn(&c); return signClaims(f.alice, c) }
	}
	cases := []tc{
		{"unsigned", func(f *fixture) []byte {
			var w waiverToken
			_ = json.Unmarshal(signClaims(f.alice, f.claims()), &w)
			w.Signature = ""
			b, _ := json.Marshal(w)
			return b
		}, true},
		{"tampered payload", func(f *fixture) []byte {
			var w waiverToken
			_ = json.Unmarshal(signClaims(f.alice, f.claims()), &w)
			c := f.claims()
			c.NodeID = "t"
			c.Reason = "tampered"
			cj, _ := json.Marshal(c)
			w.Payload = base64.StdEncoding.EncodeToString(cj)
			b, _ := json.Marshal(w)
			return b
		}, true},
		{"unknown key id", func(f *fixture) []byte { return signClaims(f.rogue, f.claims()) }, true},
		{"known key id wrong private key", func(f *fixture) []byte {
			s := f.rogue
			s.id = "k-alice"
			return signClaims(s, f.claims())
		}, true},
		{"expired", mod(func(c *WaiverClaims) { c.ExpiresAt = c.NotBefore + 30 }), true},
		{"not yet valid", mod(func(c *WaiverClaims) { c.NotBefore += 7200; c.ExpiresAt += 7200 }), true},
		{"over-long lifetime", mod(func(c *WaiverClaims) { c.ExpiresAt = c.NotBefore + int64(MaxWaiverLifetime/time.Second) + 1 }), true},
		{"inverted window", mod(func(c *WaiverClaims) { c.ExpiresAt = c.NotBefore - 1 }), true},
		{"wrong shipment", mod(func(c *WaiverClaims) { c.ShipmentID = "999-S" }), true},
		{"wrong node", mod(func(c *WaiverClaims) { c.NodeID = "other" }), true},
		{"wrong head commit", mod(func(c *WaiverClaims) { c.HeadCommit = otherHead }), true},
		{"self-asserted principal", mod(func(c *WaiverClaims) { c.Principal = "carol" }), true},
		{"empty principal", mod(func(c *WaiverClaims) { c.Principal = "" }), true},
		{"missing reason", mod(func(c *WaiverClaims) { c.Reason = "" }), true},
		{"bad kind", mod(func(c *WaiverClaims) { c.Kind = "root" }), true},
		{"short nonce", mod(func(c *WaiverClaims) { c.Nonce = "abc" }), true},
		{"unauthorized family", func(f *fixture) []byte {
			c := f.claims()
			c.Principal = "carol"
			return signClaims(f.carol, c)
		}, true},
		{"agent automation key", func(f *fixture) []byte {
			c := f.claims()
			c.Principal = "bot"
			return signClaims(f.bot, c)
		}, true},
		{"unsupported token version", func(f *fixture) []byte {
			var w waiverToken
			_ = json.Unmarshal(signClaims(f.alice, f.claims()), &w)
			w.SchemaVersion = 2
			b, _ := json.Marshal(w)
			return b
		}, true},
		{"oversize token", func(f *fixture) []byte { return []byte(strings.Repeat("x", MaxWaiverTokenBytes+1)) }, true},
		{"garbage", func(f *fixture) []byte { return []byte("not json") }, true},
		{"empty", func(f *fixture) []byte { return nil }, true},
		{"unknown token field", func(f *fixture) []byte {
			b := signClaims(f.alice, f.claims())
			return []byte(strings.Replace(string(b), `"schema_version":1`, `"schema_version":1,"admin":true`, 1))
		}, true},
		{"waivers disabled", func(f *fixture) []byte { return signClaims(f.alice, f.claims()) }, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t)
			var svc *WaiverService
			if c.svc {
				svc = f.svc
			}
			g := f.gate(ModeEnforce, svc)
			dec, err := g.Check(context.Background(), GateInput{Shipment: ship(), WaiverTokens: [][]byte{c.mk(f)}})
			require.NoError(t, err)
			assert.False(t, dec.ReviewReady, "a denied waiver must never satisfy the gate")
			assert.False(t, dec.Allowed)
			require.Len(t, dec.Denials, 1)
			assert.Equal(t, 0, dec.Denials[0].Index)
			assert.NotEmpty(t, dec.Denials[0].Reason)
			assert.LessOrEqual(t, len(dec.Denials[0].Reason), MaxReasonBytes)
			recs, err := f.ledger.Records(nil)
			require.NoError(t, err)
			assert.Empty(t, recs, "nothing is recorded for a denied waiver")
		})
	}
}

func TestU162_003_AcceptReturnsDeniedSentinels(t *testing.T) {
	f := newFixture(t)
	d := decl([]Node{pnode("t", true)})
	_, err := f.svc.Accept(signClaims(f.rogue, f.claims()), ship(), lookupFor(d))
	assert.ErrorIs(t, err, ErrWaiverDenied)
	w, err := f.svc.Accept(signClaims(f.alice, f.claims()), ship(), lookupFor(d))
	require.NoError(t, err)
	assert.Equal(t, "alice", w.Principal)
	assert.Equal(t, "t", w.NodeID)
	assert.True(t, w.verified)
}

func TestU162_003_ReplayedNonceDenied(t *testing.T) {
	f := newFixture(t)
	d := decl([]Node{pnode("t", true), pnode("u", true)})
	c1 := f.claims()
	_, err := f.svc.Accept(signClaims(f.alice, c1), ship(), lookupFor(d))
	require.NoError(t, err)

	c2 := c1
	c2.NodeID = "u"
	_, err = f.svc.Accept(signClaims(f.alice, c2), ship(), lookupFor(d))
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNonceReplayed)
	assert.ErrorIs(t, err, ErrWaiverDenied)

	c3 := c1
	c3.Reason = "a different token body with the same nonce"
	_, err = f.svc.Accept(signClaims(f.alice, c3), ship(), lookupFor(d))
	assert.ErrorIs(t, err, ErrNonceReplayed)

	recs, err := f.ledger.Records(nil)
	require.NoError(t, err)
	assert.Len(t, recs, 1)
}

func TestU162_003_GateDeniesDuplicateTokensAndUndeclaredNodes(t *testing.T) {
	f := newFixture(t)
	g := f.gate(ModeEnforce, f.svc)
	ghost := f.claims()
	ghost.NodeID = "ghost"
	dec, err := g.Check(context.Background(), GateInput{Shipment: ship(), WaiverTokens: [][]byte{
		signClaims(f.alice, f.claims()), signClaims(f.alice, f.claims()), signClaims(f.alice, ghost),
	}})
	require.NoError(t, err)
	assert.True(t, dec.ReviewReady)
	require.Len(t, dec.Denials, 2)
	assert.Equal(t, 1, dec.Denials[0].Index)
	assert.Equal(t, 2, dec.Denials[1].Index)

	tooMany := make([][]byte, MaxNodes+1)
	_, err = g.Check(context.Background(), GateInput{Shipment: ship(), WaiverTokens: tooMany})
	assert.ErrorIs(t, err, ErrInvalidInput)
}

func TestU162_003_WaiverDoesNotBypassAuthenticityOfOtherNodes(t *testing.T) {
	f := newFixture(t)
	d := decl([]Node{pnode("a", true), pnode("t", true)})
	g, err := NewGate(d, ModeEnforce, f.svc)
	require.NoError(t, err)
	forged := rec(t, "a", parityArtifact(testTaskB, testHead, faultline.StatusPass))
	dec, err := g.Check(context.Background(), GateInput{
		Shipment: ship(), Evidence: map[string]EvidenceRecord{"a": forged},
		WaiverTokens: [][]byte{signClaims(f.alice, f.claims())},
	})
	require.NoError(t, err)
	assert.False(t, dec.ReviewReady, "waiving t must not launder forged evidence for a")
}

func TestU162_003_NoInteractiveShortcut(t *testing.T) {
	// The package exposes no issuance, TTY or environment path that can mint a
	// verified Waiver: the only constructor is WaiverService.Accept.
	f := newFixture(t)
	g := f.gate(ModeEnforce, f.svc)
	t.Setenv("BACKLOGIT_WAIVE", "1")
	t.Setenv("CI", "")
	dec, err := g.Check(context.Background(), GateInput{Shipment: ship()})
	require.NoError(t, err)
	assert.False(t, dec.ReviewReady)
	_, err = os.Stat(filepath.Join(f.dir, "waivers.jsonl"))
	assert.True(t, os.IsNotExist(err))
}
