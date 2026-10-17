package policyengine

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/evidencedag"
)

// waiverDomain mirrors the signature domain of the S10 waiver tokens, which are
// minted out of band by a human-held key; the production code has no issuance
// API.
const waiverDomain = "backlogit/evidencedag/waiver/v1\n"

type overrideEnv struct {
	t       *testing.T
	e       *env
	humanK  ed25519.PrivateKey
	autoK   ed25519.PrivateKey
	rogueK  ed25519.PrivateKey
	ledger  *evidencedag.Ledger
	lpath   string
	anchors *evidencedag.MemoryAnchorStore
	svc     *evidencedag.WaiverService
	nonce   int
}

func newOverrideEnv(t *testing.T, e *env) *overrideEnv {
	t.Helper()
	gen := func() (ed25519.PublicKey, ed25519.PrivateKey) {
		pub, priv, err := ed25519.GenerateKey(rand.Reader)
		require.NoError(t, err)
		return pub, priv
	}
	humanPub, humanPriv := gen()
	autoPub, autoPriv := gen()
	_, roguePriv := gen()
	b64 := base64.StdEncoding.EncodeToString
	trustDoc := fmt.Sprintf(`{"schema_version":1,"keys":[
{"key_id":"alice-key","principal":"alice","kind":"human","public_key":%q},
{"key_id":"bot-key","principal":"bot","kind":"automation","public_key":%q}],
"policy":[{"principal":"alice","families":["workflow-policy"]},{"principal":"bot","families":["workflow-policy"]}]}`,
		b64(humanPub), b64(autoPub))
	trust, err := evidencedag.ParseTrust([]byte(trustDoc))
	require.NoError(t, err)

	dir := t.TempDir()
	lpath := filepath.Join(dir, "overrides.jsonl")
	forbidden := t.TempDir()
	ledger, err := evidencedag.OpenLedger(lpath, forbidden)
	require.NoError(t, err)
	anchors := &evidencedag.MemoryAnchorStore{}
	clock := func() time.Time { return e.t0.Add(time.Hour) }
	svc, err := evidencedag.NewWaiverService(trust, ledger, anchors, clock)
	require.NoError(t, err)
	return &overrideEnv{t: t, e: e, humanK: humanPriv, autoK: autoPriv, rogueK: roguePriv, ledger: ledger, lpath: lpath, anchors: anchors, svc: svc}
}

func (o *overrideEnv) engine(mut ...func(*Options)) *Engine {
	return o.e.engine(append([]func(*Options){func(op *Options) { op.Waivers = o.svc }}, mut...)...)
}

// token mints a signed override token. Fields are overridable by mut.
func (o *overrideEnv) token(priv ed25519.PrivateKey, keyID string, node NodeType, mut ...func(*evidencedag.WaiverClaims)) []byte {
	o.t.Helper()
	o.nonce++
	now := o.e.t0.Add(time.Hour).Unix()
	c := evidencedag.WaiverClaims{
		Nonce: fmt.Sprintf("%032x", o.nonce), ShipmentID: o.e.task, NodeID: string(node), HeadCommit: o.e.c[4],
		Principal: "alice", Kind: "waiver", Reason: "P-002.1 harness-exempt: generated code", NotBefore: now - 60, ExpiresAt: now + 600,
	}
	for _, m := range mut {
		m(&c)
	}
	payload, err := json.Marshal(c)
	require.NoError(o.t, err)
	sig := ed25519.Sign(priv, append([]byte(waiverDomain), payload...))
	raw, err := json.Marshal(map[string]any{
		"schema_version": 1, "key_id": keyID,
		"payload": base64.StdEncoding.EncodeToString(payload), "signature": base64.StdEncoding.EncodeToString(sig),
	})
	require.NoError(o.t, err)
	return raw
}

func (o *overrideEnv) human(node NodeType, mut ...func(*evidencedag.WaiverClaims)) []byte {
	return o.token(o.humanK, "alice-key", node, mut...)
}

func overrideRequest(e *env, recs map[NodeType]Record, interactive bool, tokens ...[]byte) Request {
	r := e.request(recs)
	r.Interactive = interactive
	r.OverrideTokens = tokens
	return r
}

func TestU163_003_AuthenticatedOperatorOverrideWaivesOnlyTheNamedGate(t *testing.T) {
	e := newEnv(t, enforceRegistry)
	o := newOverrideEnv(t, e)
	recs := e.good()
	delete(recs, NodeRedPhaseVerify)

	d := check(t, o.engine(), overrideRequest(e, recs, true, o.human(NodeRedPhaseVerify)))
	assert.Equal(t, DispositionPass, d.Disposition, "%+v %+v", d.Errors, d.OverrideDenials)
	assert.Equal(t, "waived", gateStatus(d, NodeRedPhaseVerify))
	for _, g := range d.Gates {
		if g.Node == NodeRedPhaseVerify {
			assert.Equal(t, "alice", g.Waiver)
		}
	}

	log, err := VerifyOverrideLog(o.ledger, o.anchors)
	require.NoError(t, err)
	require.Len(t, log.Entries, 1)
	en := log.Entries[0]
	assert.Equal(t, "alice", en.Principal)
	assert.Equal(t, string(NodeRedPhaseVerify), en.Gate)
	assert.Equal(t, e.task, en.TaskID)
	assert.Contains(t, en.Justification, "P-002.1")
	assert.Equal(t, e.c[4], en.HeadCommit)
	assert.Equal(t, 1, log.Records)
}

func TestU163_003_OverrideDoesNotHideOtherFailures(t *testing.T) {
	e := newEnv(t, enforceRegistry)
	o := newOverrideEnv(t, e)
	recs := e.good()
	delete(recs, NodeHarnessScaffold)
	delete(recs, NodeRedPhaseVerify)
	d := check(t, o.engine(), overrideRequest(e, recs, true, o.human(NodeRedPhaseVerify)))
	assert.False(t, d.Allowed, "waiving red must not hide the missing harness: %+v", d.Errors)
	assert.True(t, hasViolation(d, NodeHarnessScaffold, CodeEvidenceAbsent))
	assert.False(t, d.ReviewReady)
}

func TestU163_003_NonOverridableGatesAreDeniedWithoutConsumingTheToken(t *testing.T) {
	for _, node := range []NodeType{NodeGreenPhaseVerify, NodeReviewComplete, NodeCIPass, NodeMergeReady} {
		t.Run(string(node), func(t *testing.T) {
			e := newEnv(t, enforceRegistry)
			o := newOverrideEnv(t, e)
			recs := e.good()
			delete(recs, NodeGreenPhaseVerify)
			delete(recs, NodeMergeReady)
			d := check(t, o.engine(), overrideRequest(e, recs, true, o.human(node)))
			assert.False(t, d.Allowed)
			require.Len(t, d.OverrideDenials, 1)
			assert.Contains(t, d.OverrideDenials[0].Reason, "not overridable")
			assert.Equal(t, 0, d.OverrideDenials[0].Index)
			log, err := VerifyOverrideLog(o.ledger, o.anchors)
			require.NoError(t, err)
			assert.Zero(t, log.Records, "a denied token must not be recorded")
		})
	}
}

func TestU163_003_OverridesFromNonInteractiveContextsAreDeniedByDefault(t *testing.T) {
	cases := map[string]struct {
		surface     Surface
		interactive bool
		allow       bool
		want        bool
	}{
		"cli interactive":                 {SurfaceCLI, true, false, true},
		"cli non-interactive":             {SurfaceCLI, false, false, false},
		"mcp even if flagged interactive": {SurfaceMCP, true, false, false},
		"agent even if flagged":           {SurfaceAgent, true, false, false},
		"mcp with explicit opt-in":        {SurfaceMCP, false, true, true},
		"agent with explicit opt-in":      {SurfaceAgent, false, true, true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t, enforceRegistry)
			o := newOverrideEnv(t, e)
			recs := e.good()
			delete(recs, NodeRedPhaseVerify)
			req := overrideRequest(e, recs, tc.interactive, o.human(NodeRedPhaseVerify))
			req.Surface = tc.surface
			d := check(t, o.engine(func(op *Options) { op.AllowNonInteractiveOverrides = tc.allow }), req)
			assert.Equal(t, tc.want, d.Allowed, "%+v %+v", d.Errors, d.OverrideDenials)
			if !tc.want {
				require.Len(t, d.OverrideDenials, 1)
				assert.Contains(t, d.OverrideDenials[0].Reason, "non-interactive")
				log, err := VerifyOverrideLog(o.ledger, o.anchors)
				require.NoError(t, err)
				assert.Zero(t, log.Records)
			}
		})
	}
}

func TestU163_003_SelfAssertedOrUnverifiableIdentityIsDenied(t *testing.T) {
	cases := map[string]func(o *overrideEnv) []byte{
		"principal claimed by a different key": func(o *overrideEnv) []byte {
			return o.token(o.rogueK, "alice-key", NodeRedPhaseVerify) // signed by the wrong key
		},
		"unknown key id": func(o *overrideEnv) []byte {
			return o.token(o.humanK, "mallory-key", NodeRedPhaseVerify)
		},
		"automation key": func(o *overrideEnv) []byte {
			return o.token(o.autoK, "bot-key", NodeRedPhaseVerify, func(c *evidencedag.WaiverClaims) { c.Principal = "bot" })
		},
		"claimed principal differs from the key": func(o *overrideEnv) []byte {
			return o.human(NodeRedPhaseVerify, func(c *evidencedag.WaiverClaims) { c.Principal = "operator" })
		},
		"expired": func(o *overrideEnv) []byte {
			return o.human(NodeRedPhaseVerify, func(c *evidencedag.WaiverClaims) {
				c.NotBefore -= 7200
				c.ExpiresAt -= 7200
			})
		},
		"not yet valid": func(o *overrideEnv) []byte {
			return o.human(NodeRedPhaseVerify, func(c *evidencedag.WaiverClaims) {
				c.NotBefore += 3600
				c.ExpiresAt += 3600
			})
		},
		"bound to another head": func(o *overrideEnv) []byte {
			return o.human(NodeRedPhaseVerify, func(c *evidencedag.WaiverClaims) { c.HeadCommit = sha(77) })
		},
		"bound to another task": func(o *overrideEnv) []byte {
			return o.human(NodeRedPhaseVerify, func(c *evidencedag.WaiverClaims) { c.ShipmentID = "163.009-T" })
		},
		"empty justification": func(o *overrideEnv) []byte {
			return o.human(NodeRedPhaseVerify, func(c *evidencedag.WaiverClaims) { c.Reason = "" })
		},
		"garbage":    func(*overrideEnv) []byte { return []byte("not a token") },
		"empty json": func(*overrideEnv) []byte { return []byte("{}") },
		"unsigned operator field": func(*overrideEnv) []byte {
			return []byte(`{"operator":"alice","principal":"alice","node":"red-phase-verify"}`)
		},
	}
	for name, mk := range cases {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t, enforceRegistry)
			o := newOverrideEnv(t, e)
			recs := e.good()
			delete(recs, NodeRedPhaseVerify)
			d := check(t, o.engine(), overrideRequest(e, recs, true, mk(o)))
			assert.False(t, d.Allowed, "%+v", d.Errors)
			require.NotEmpty(t, d.OverrideDenials)
			log, err := VerifyOverrideLog(o.ledger, o.anchors)
			require.NoError(t, err)
			assert.Zero(t, log.Records)
		})
	}
}

func TestU163_003_TokenReuseIsIdempotentAndNonceReplayIsDenied(t *testing.T) {
	e := newEnv(t, enforceRegistry)
	o := newOverrideEnv(t, e)
	recs := e.good()
	delete(recs, NodeRedPhaseVerify)
	tok := o.human(NodeRedPhaseVerify)
	eng := o.engine()
	assert.True(t, check(t, eng, overrideRequest(e, recs, true, tok)).Allowed)

	// The identical token is an idempotent retry bound to the same task, gate
	// and head commit; it adds no second record.
	assert.True(t, check(t, eng, overrideRequest(e, recs, true, tok)).Allowed)
	log, err := VerifyOverrideLog(o.ledger, o.anchors)
	require.NoError(t, err)
	assert.Equal(t, 1, log.Records)

	// A different token reusing the consumed nonce is a replay and is denied.
	nonce := log.Entries[0].Nonce
	forged := o.human(NodeRedPhaseVerify, func(c *evidencedag.WaiverClaims) {
		c.Nonce = nonce
		c.Reason = "a different justification"
	})
	d := check(t, eng, overrideRequest(e, recs, true, forged))
	assert.False(t, d.Allowed)
	assert.NotEmpty(t, d.OverrideDenials)
}

func TestU163_003_OverridesRequireATrustAnchor(t *testing.T) {
	e := newEnv(t, enforceRegistry)
	o := newOverrideEnv(t, e)
	recs := e.good()
	delete(recs, NodeRedPhaseVerify)
	d := check(t, e.engine(), overrideRequest(e, recs, true, o.human(NodeRedPhaseVerify)))
	assert.False(t, d.Allowed)
	require.Len(t, d.OverrideDenials, 1)
	assert.Contains(t, d.OverrideDenials[0].Reason, "not enabled")
}

func TestU163_003_TokenForADisabledGateIsDenied(t *testing.T) {
	doc := strings.Replace(enforceRegistry, "{id: red-phase-verify, enabled: true", "{id: red-phase-verify, enabled: false", 1)
	e := newEnv(t, doc)
	o := newOverrideEnv(t, e)
	d := check(t, o.engine(), overrideRequest(e, e.good(), true, o.human(NodeRedPhaseVerify)))
	assert.NotEmpty(t, d.OverrideDenials)
	log, err := VerifyOverrideLog(o.ledger, o.anchors)
	require.NoError(t, err)
	assert.Zero(t, log.Records)
}

func TestU163_003_OverrideTokenCountIsBounded(t *testing.T) {
	e := newEnv(t, enforceRegistry)
	o := newOverrideEnv(t, e)
	toks := make([][]byte, maxOverrideTokens+1)
	for i := range toks {
		toks[i] = []byte("x")
	}
	d, err := o.engine().Check(context.Background(), overrideRequest(e, e.good(), true, toks...))
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidRequest)
	assert.False(t, d.Allowed)
}

func TestU163_003_OverrideDenialIndicesRefertoTheRequestOrder(t *testing.T) {
	e := newEnv(t, enforceRegistry)
	o := newOverrideEnv(t, e)
	recs := e.good()
	delete(recs, NodeRedPhaseVerify)
	d := check(t, o.engine(), overrideRequest(e, recs, true,
		o.human(NodeCIPass), // 0: not overridable
		o.human(NodeRedPhaseVerify, func(c *evidencedag.WaiverClaims) { c.HeadCommit = sha(77) }), // 1: wrong head
		o.human(NodeRedPhaseVerify), // 2: valid
	))
	assert.True(t, d.Allowed, "%+v %+v", d.Errors, d.OverrideDenials)
	var idx []int
	for _, dn := range d.OverrideDenials {
		idx = append(idx, dn.Index)
	}
	assert.ElementsMatch(t, []int{0, 1}, idx)
}

func TestU163_003_OverrideLogIsTamperEvident(t *testing.T) {
	setup := func(t *testing.T) *overrideEnv {
		e := newEnv(t, enforceRegistry)
		o := newOverrideEnv(t, e)
		recs := e.good()
		delete(recs, NodeRedPhaseVerify)
		eng := o.engine()
		require.True(t, check(t, eng, overrideRequest(e, recs, true, o.human(NodeRedPhaseVerify))).Allowed)
		recs2 := e.good()
		delete(recs2, NodeHarnessScaffold)
		require.True(t, check(t, eng, overrideRequest(e, recs2, true, o.human(NodeHarnessScaffold))).Allowed)
		log, err := VerifyOverrideLog(o.ledger, o.anchors)
		require.NoError(t, err)
		require.Equal(t, 2, log.Records)
		assert.Equal(t, log.Entries[1].PrevHash, log.Entries[0].Hash, "records must be hash chained")
		return o
	}
	read := func(t *testing.T, o *overrideEnv) []string {
		raw, err := os.ReadFile(o.lpath)
		require.NoError(t, err)
		return strings.Split(strings.TrimRight(string(raw), "\r\n"), "\n")
	}
	write := func(t *testing.T, o *overrideEnv, lines []string) {
		require.NoError(t, os.WriteFile(o.lpath, []byte(strings.Join(lines, "\n")+"\n"), 0o600))
	}
	cases := map[string]func(t *testing.T, o *overrideEnv){
		"edited justification": func(t *testing.T, o *overrideEnv) {
			lines := read(t, o)
			lines[0] = strings.Replace(lines[0], "P-002.1", "P-999.9", 1)
			write(t, o, lines)
		},
		"edited principal": func(t *testing.T, o *overrideEnv) {
			lines := read(t, o)
			lines[1] = strings.Replace(lines[1], `"alice"`, `"eve"`, 1)
			write(t, o, lines)
		},
		"truncated tail": func(t *testing.T, o *overrideEnv) { write(t, o, read(t, o)[:1]) },
		"emptied":        func(t *testing.T, o *overrideEnv) { write(t, o, nil) },
		"deleted":        func(t *testing.T, o *overrideEnv) { require.NoError(t, os.Remove(o.lpath)) },
		"reordered": func(t *testing.T, o *overrideEnv) {
			lines := read(t, o)
			write(t, o, []string{lines[1], lines[0]})
		},
		"appended forgery": func(t *testing.T, o *overrideEnv) {
			lines := read(t, o)
			write(t, o, append(lines, lines[0]))
		},
	}
	for name, tamper := range cases {
		t.Run(name, func(t *testing.T) {
			o := setup(t)
			tamper(t, o)
			_, err := VerifyOverrideLog(o.ledger, o.anchors)
			require.Error(t, err)
			assert.ErrorIs(t, err, evidencedag.ErrLedgerTampered)
		})
	}
}

func TestU163_003_VerifyOverrideLogEmptyAndUnanchored(t *testing.T) {
	e := newEnv(t, enforceRegistry)
	o := newOverrideEnv(t, e)
	log, err := VerifyOverrideLog(o.ledger, o.anchors)
	require.NoError(t, err)
	assert.Zero(t, log.Records)
	assert.NotNil(t, log.Entries, "an empty log is an empty list, not nil")

	_, err = VerifyOverrideLog(nil, o.anchors)
	assert.Error(t, err)
	_, err = VerifyOverrideLog(o.ledger, nil)
	assert.Error(t, err)

	// A ledger that exists while the anchor says genesis is tampering, not "no anchor".
	require.NoError(t, os.WriteFile(o.lpath, []byte("{}\n"), 0o600))
	_, err = VerifyOverrideLog(o.ledger, o.anchors)
	assert.Error(t, err)
}

var identityFieldPattern = regexp.MustCompile(`(?i)principal|operator|identity|actor|user|approver|author`)

func TestU163_003_NoSurfaceInputCarriesASelfAssertedIdentity(t *testing.T) {
	for _, typ := range []reflect.Type{reflect.TypeOf(Request{}), reflect.TypeOf(Bundle{}), reflect.TypeOf(Record{})} {
		for i := 0; i < typ.NumField(); i++ {
			assert.False(t, identityFieldPattern.MatchString(typ.Field(i).Name), "%s.%s must not carry an identity", typ.Name(), typ.Field(i).Name)
			assert.False(t, identityFieldPattern.MatchString(typ.Field(i).Tag.Get("json")), "%s.%s json tag", typ.Name(), typ.Field(i).Name)
		}
	}
}

func decisionKey(d Decision) string {
	d.Surface = ""
	b, _ := json.Marshal(d)
	return string(b)
}

func TestU163_003_SurfaceParityNoSurfaceBypassesAGate(t *testing.T) {
	scenarios := map[string]func(*env, map[NodeType]Record){
		"satisfied":      func(*env, map[NodeType]Record) {},
		"absent harness": func(e *env, m map[NodeType]Record) { delete(m, NodeHarnessScaffold) },
		"absent red":     func(e *env, m map[NodeType]Record) { delete(m, NodeRedPhaseVerify) },
		"fabricated sha": func(e *env, m map[NodeType]Record) {
			m[NodeGreenPhaseVerify] = e.rec(NodeGreenPhaseVerify, sha(999), e.at(3, 30), OutcomePass, nil)
		},
		"retroactive": func(e *env, m map[NodeType]Record) {
			m[NodeGreenPhaseVerify] = e.rec(NodeGreenPhaseVerify, e.c[3], e.at(0, 1), OutcomePass, nil)
		},
		"out of order": func(e *env, m map[NodeType]Record) {
			m[NodeGreenPhaseVerify] = e.rec(NodeGreenPhaseVerify, e.c[1], e.at(3, 30), OutcomePass, nil)
		},
		"ci failing":      func(e *env, m map[NodeType]Record) { e.deriv.ci.AllPassed = false },
		"review degraded": func(e *env, m map[NodeType]Record) { e.deriv.review.Status = "degraded" },
	}
	for name, mut := range scenarios {
		t.Run(name, func(t *testing.T) {
			for _, mode := range []string{enforceRegistry, canonicalRegistryYAML} {
				e := newEnv(t, mode)
				m := e.good()
				mut(e, m)
				eng := e.engine()
				req := e.request(m)
				raw, err := json.Marshal(Bundle{SchemaVersion: EvidenceSchemaVersion, TaskID: req.TaskID, HeadCommit: req.HeadCommit, PR: req.PR, Records: req.Records})
				require.NoError(t, err)

				var keys []string
				for _, s := range Surfaces() {
					d, err := eng.CheckBundle(context.Background(), s, raw)
					require.NoError(t, err)
					assert.Equal(t, s, d.Surface)
					keys = append(keys, decisionKey(d))

					direct := req
					direct.Surface = s
					dd := check(t, eng, direct)
					assert.Equal(t, decisionKey(dd), decisionKey(d), "CheckBundle must be a thin adapter over Check")
				}
				for _, k := range keys[1:] {
					assert.Equal(t, keys[0], k, "surfaces must agree on every gate")
				}
			}
		})
	}
}

func TestU163_003_CheckBundleRejectsAmbiguousInputOnEverySurface(t *testing.T) {
	e := newEnv(t, enforceRegistry)
	eng := e.engine()
	for _, s := range Surfaces() {
		d, err := eng.CheckBundle(context.Background(), s, []byte(`{"schema_version":1,"operator":"alice"}`))
		require.Error(t, err)
		assert.False(t, d.Allowed)
		assert.Equal(t, DispositionBlock, d.Disposition)
	}
	d, err := eng.CheckBundle(context.Background(), "bot", []byte(`{}`))
	require.Error(t, err)
	assert.False(t, d.Allowed)
}

func TestU163_003_BundleChannelsNeverCarryInteractivityOrTokens(t *testing.T) {
	e := newEnv(t, enforceRegistry)
	o := newOverrideEnv(t, e)
	recs := e.good()
	delete(recs, NodeRedPhaseVerify)
	req := e.request(recs)
	raw, err := json.Marshal(Bundle{SchemaVersion: EvidenceSchemaVersion, TaskID: req.TaskID, HeadCommit: req.HeadCommit, PR: req.PR, Records: req.Records})
	require.NoError(t, err)
	d, err := o.engine().CheckBundle(context.Background(), SurfaceCLI, raw)
	require.NoError(t, err)
	assert.False(t, d.Allowed, "the bundle channel can never express an override")
}
