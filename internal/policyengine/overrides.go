package policyengine

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/softwaresalt/backlogit/internal/evidencedag"
)

const (
	// maxOverrideTokens caps the override tokens in one request.
	maxOverrideTokens = 8
	// maxTokenPayload bounds the claims a token peek will decode.
	maxTokenPayload = 8 << 10
	// CodeOverrideNotPermitted marks a waived gate that is not overridable.
	CodeOverrideNotPermitted = "override_not_permitted"
)

// OverrideEntry is one verified override record: the authenticated principal
// (taken from the verified signing key, never from the request), the operator's
// justification and the specific gate waived.
type OverrideEntry struct {
	Seq           int    `json:"seq"`
	AcceptedAt    int64  `json:"accepted_at"`
	TaskID        string `json:"task_id"`
	Gate          string `json:"gate"`
	HeadCommit    string `json:"head_commit"`
	Principal     string `json:"principal"`
	KeyID         string `json:"key_id"`
	Justification string `json:"justification"`
	Nonce         string `json:"nonce"`
	PrevHash      string `json:"prev_hash"`
	Hash          string `json:"hash"`
}

// OverrideLog is the verified override history.
type OverrideLog struct {
	Records int                `json:"records"`
	Head    evidencedag.Anchor `json:"head"`
	Entries []OverrideEntry    `json:"entries"`
}

// ErrAnchorMissing reports that no external anchor has ever been stored for
// the override ledger, so the ledger cannot be verified against anything.
var ErrAnchorMissing = errors.New("policyengine: the override ledger has no external anchor")

// VerifyOption configures VerifyOverrideLog.
type VerifyOption func(*verifyOptions)

type verifyOptions struct{ genesis bool }

// AllowGenesis declares that a missing anchor is an expected genesis state, so
// the ledger is verified as empty. Without it a missing anchor is an error:
// an operator who deleted the anchor must not get a silent pass.
func AllowGenesis() VerifyOption { return func(o *verifyOptions) { o.genesis = true } }

// VerifyOverrideLog verifies the append-only override ledger against its
// external anchor and returns its entries. The ledger must match the anchor
// exactly: an edited, reordered, extended, truncated, emptied or deleted
// ledger fails with evidencedag.ErrLedgerTampered. A missing anchor fails with
// ErrAnchorMissing unless AllowGenesis is passed, in which case the ledger
// must be empty. Verification takes the ledger's lock file, so the caller
// needs write access to the ledger directory.
func VerifyOverrideLog(l *evidencedag.Ledger, anchors evidencedag.AnchorStore, opts ...VerifyOption) (OverrideLog, error) {
	var vo verifyOptions
	for _, o := range opts {
		o(&vo)
	}
	if l == nil {
		return OverrideLog{}, errors.New("policyengine: an override ledger is required")
	}
	if anchors == nil {
		return OverrideLog{}, evidencedag.ErrAnchorRequired
	}
	a, ok, err := anchors.Load()
	if err != nil {
		return OverrideLog{}, fmt.Errorf("policyengine: load override anchor: %w", err)
	}
	if !ok {
		if !vo.genesis {
			return OverrideLog{}, ErrAnchorMissing
		}
		a = evidencedag.Anchor{}
	}
	recs, err := l.Records(&a)
	if err != nil {
		return OverrideLog{}, fmt.Errorf("policyengine: verify override log: %w", err)
	}
	out := OverrideLog{Records: len(recs), Head: a, Entries: make([]OverrideEntry, 0, len(recs))}
	for _, r := range recs {
		out.Entries = append(out.Entries, OverrideEntry{
			Seq: r.Seq, AcceptedAt: r.AcceptedAt, TaskID: r.ShipmentID, Gate: r.NodeID, HeadCommit: r.HeadCommit,
			Principal: r.Principal, KeyID: r.KeyID, Justification: r.Reason, Nonce: r.Nonce,
			PrevHash: r.PrevHash, Hash: r.Hash,
		})
	}
	return out, nil
}

// CheckBundle decodes an evidence bundle and evaluates it for a surface. It is
// the shared adapter for the MCP and agent surfaces: it adds no gate logic and
// the bundle channel can carry neither interactivity nor override tokens.
func (e *Engine) CheckBundle(ctx context.Context, s Surface, raw []byte) (Decision, error) {
	mode := Mode("")
	if e != nil {
		mode = e.mode
	}
	b, err := DecodeBundle(raw)
	if err != nil {
		return blocked(Request{Surface: s}, mode, Violation{Code: CodeInvalidRequest, Reason: clip(err.Error(), MaxErrorBytes)}, err)
	}
	return e.Check(ctx, b.Request(s))
}

// overridePlan is the outcome of screening a request's override tokens before
// the core gate sees them.
type overridePlan struct {
	// tokens are the tokens the core gate may verify and consume.
	tokens [][]byte
	// index maps a position in tokens back to the request position.
	index   []int
	denials []OverrideDenial
}

// prepareOverrides screens override tokens. A token reaches the core
// verification (and so may be consumed) only when the context is interactive
// (or non-interactive overrides are explicitly enabled), a trust anchor is
// configured and the named gate is overridable. The core then authenticates it
// against the out-of-band trust anchor; nothing here trusts any field of the
// token.
func (e *Engine) prepareOverrides(req Request) overridePlan {
	var plan overridePlan
	if len(req.OverrideTokens) == 0 {
		return plan
	}
	denyAll := func(reason string) overridePlan {
		for i := range req.OverrideTokens {
			plan.denials = append(plan.denials, OverrideDenial{Index: i, Reason: reason})
		}
		return plan
	}
	interactive := req.Surface == SurfaceCLI && req.Interactive
	if !interactive && !e.opts.AllowNonInteractiveOverrides {
		return denyAll("override denied: non-interactive context")
	}
	if e.opts.Waivers == nil {
		return denyAll("overrides are not enabled: no trust anchor and ledger are configured")
	}
	for i, tok := range req.OverrideTokens {
		node, ok := peekTokenGate(tok)
		if !ok {
			plan.denials = append(plan.denials, OverrideDenial{Index: i, Reason: "override token is malformed"})
			continue
		}
		g, known := e.reg.Gate(NodeType(node))
		if !known || !g.Overridable {
			plan.denials = append(plan.denials, OverrideDenial{Index: i, Reason: fmt.Sprintf("gate %s is not overridable", clip(node, 64))})
			continue
		}
		plan.tokens = append(plan.tokens, tok)
		plan.index = append(plan.index, i)
	}
	return plan
}

// peekTokenGate reads the gate a token names without verifying it. The result
// is used only to deny: a token it clears is still fully verified by the core,
// which refuses duplicate and unknown keys, so the two decodes cannot differ.
func peekTokenGate(raw []byte) (string, bool) {
	if len(raw) == 0 || len(raw) > evidencedag.MaxWaiverTokenBytes {
		return "", false
	}
	var tok struct {
		Payload string `json:"payload"`
	}
	if json.Unmarshal(raw, &tok) != nil {
		return "", false
	}
	payload, err := base64.StdEncoding.DecodeString(tok.Payload)
	if err != nil || len(payload) == 0 || len(payload) > maxTokenPayload {
		return "", false
	}
	var claims struct {
		NodeID string `json:"node_id"`
	}
	if json.Unmarshal(payload, &claims) != nil || claims.NodeID == "" {
		return "", false
	}
	return claims.NodeID, true
}

func (p overridePlan) original(i int) int {
	if i >= 0 && i < len(p.index) {
		return p.index[i]
	}
	return i
}
