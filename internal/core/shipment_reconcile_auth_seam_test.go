package core

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/models"
)

// seamNonceProvider is an in-memory external compare-and-consume authority.
type seamNonceProvider struct {
	mu       sync.Mutex
	consumed map[string]int
}

func (p *seamNonceProvider) CompareAndConsumeNonce(_ context.Context, nonce string, _ time.Time) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.consumed == nil {
		p.consumed = map[string]int{}
	}
	if p.consumed[nonce] > 0 {
		return ErrNonceReplayed
	}
	p.consumed[nonce]++
	return nil
}

func (p *seamNonceProvider) count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.consumed)
}

type seamConsumeAuditor struct{}

func (seamConsumeAuditor) RecordNonceConsume(NonceConsumeEvent) error { return nil }

type seamRejectionAuditor struct{ events []TokenRejectionEvent }

func (a *seamRejectionAuditor) RecordTokenRejection(ev TokenRejectionEvent) error {
	a.events = append(a.events, ev)
	return nil
}

// authSeamFixture is a reconcilable archived shipment with an operator key.
type authSeamFixture struct {
	ws         *Workspace
	shipmentID string
	req        ShipmentShippedReconcileRequest
	digest     string
	priv       ed25519.PrivateKey
	key        ResolvedKey
	pub        *VerificationKey
	provider   *seamNonceProvider
	rejections *seamRejectionAuditor
	nonces     *NonceConsumer
}

func newAuthSeamFixture(t *testing.T) authSeamFixture {
	t.Helper()
	ev := setupShipmentReconcileEvidenceFixture(t)
	ev.ws.Config.Reconcile = &config.ReconcileConfig{}
	member := createShipmentReconcileMemberWithStatus(t, ev.ws, models.StatusDone)
	shipment := createArchivedShipmentReconcileFixture(t, ev.ws, []string{member})

	closureRel := ev.writeClosureFile(t, "docs/closure/auth-seam.md", strings.TrimSpace(`---
shipments:
  - `+shipment.ID+`
---

# Closure

### `+shipment.ID+`
Merged as PR #101 (feature), commit `+shortSHA(ev.merge048)+`, and PR #102 (post-merge closure), commit `+shortSHA(ev.closure048)+`.
`)+"\n")
	req := ev.request(shipment.ID, ev.merge048, closureRel)
	digest, err := shipmentReconcileRequestIdentityDigest(req)
	require.NoError(t, err)

	pub, priv, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	anchors := []config.TrustAnchor{{
		ID: "op-1", Role: TokenIssuerRole, Algo: "ed25519", Fingerprint: "sha256:t1",
		Status: config.TrustAnchorStatusActive,
	}}
	key, err := ResolveTokenIssuerKey(anchors, "op-1", time.Now(), NewBoundExternalPinSet(PinBinding{Fingerprint: "sha256:t1", Role: TokenIssuerRole}))
	require.NoError(t, err)

	projection, err := OpenNonceLedger(filepath.Join(t.TempDir(), "nonces.jsonl"))
	require.NoError(t, err)
	provider := &seamNonceProvider{}
	nonces, err := NewNonceConsumer(provider, projection, seamConsumeAuditor{})
	require.NoError(t, err)

	return authSeamFixture{
		ws: ev.ws, shipmentID: shipment.ID, req: req, digest: digest, priv: priv, key: key,
		pub:      &VerificationKey{Algo: "ed25519", Public: pub, Fingerprint: "sha256:t1"},
		provider: provider, rejections: &seamRejectionAuditor{}, nonces: nonces,
	}
}

func (f authSeamFixture) token(shipmentID, digest, nonce string) AuthorizationToken {
	now := time.Now()
	tok := AuthorizationToken{
		Header: AuthorizationTokenHeader{Algo: "ed25519", KeyID: "op-1"},
		Claims: AuthorizationTokenClaims{NotBefore: now.Add(-time.Minute), NotAfter: now.Add(5 * time.Minute), Nonce: nonce},
		Binding: AuthorizationTokenBinding{
			ShipmentID: shipmentID, RequestIdentityDigest: digest,
		},
	}
	tok.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(f.priv, tok.SigningPayload()))
	return tok
}

func (f authSeamFixture) authorized(tok AuthorizationToken) ShipmentShippedReconcileRequest {
	req := f.req
	req.AuthEnforcement = AuthEnforcementAuthenticated
	req.Authorization = &ReconcileAuthorization{
		Token: tok, Key: f.key, PublicKey: f.pub, Nonces: f.nonces, Auditor: f.rejections,
	}
	return req
}

func (f authSeamFixture) logText(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(events.LogPathForItem(WorkspaceLogsRoot(f.ws.RootPath), f.shipmentID))
	if os.IsNotExist(err) {
		return ""
	}
	require.NoError(t, err)
	return string(raw)
}

func (f authSeamFixture) archivedStatus(t *testing.T) string {
	t.Helper()
	art, err := findArtifact(context.Background(), f.ws, f.shipmentID)
	require.NoError(t, err)
	return art.ArchivedStatus
}

func TestReconcileAuthTokenSeam(t *testing.T) {
	ctx := context.Background()

	t.Run("authenticated: verified token is consumed and approver persists in the event", func(t *testing.T) {
		f := newAuthSeamFixture(t)
		res, err := ReconcileShipmentToShipped(ctx, f.ws, f.authorized(f.token(f.shipmentID, f.digest, "00aa")))
		require.NoError(t, err)
		assert.Equal(t, ShipmentReconcileOutcomeReconciled, res.Outcome)
		assert.Equal(t, AuthorizationOutcomeConsumed, res.Authorization)
		assert.Equal(t, 1, f.provider.count(), "nonce consumed via the atomic owner")
		assert.Equal(t, string(ShipmentShipped), f.archivedStatus(t))
		log := f.logText(t)
		assert.Contains(t, log, `"approver":{"algo":"ed25519","key_id":"op-1","nonce":"00aa"}`)
	})

	t.Run("authenticated: missing token fails closed with no write", func(t *testing.T) {
		f := newAuthSeamFixture(t)
		req := f.req
		req.AuthEnforcement = AuthEnforcementAuthenticated
		_, err := ReconcileShipmentToShipped(ctx, f.ws, req)
		require.ErrorIs(t, err, ErrAuthorizationRequired)
		assert.Equal(t, string(ShipmentActive), f.archivedStatus(t))
		assert.NotContains(t, f.logText(t), EventShipmentReconciledShipped)
		assert.Zero(t, f.provider.count())
	})

	t.Run("authenticated: rejected token fails closed with no write and no consume", func(t *testing.T) {
		f := newAuthSeamFixture(t)
		tok := f.token(f.shipmentID, f.digest, "00bb")
		tok.Binding.RequestIdentityDigest = "tampered"
		_, err := ReconcileShipmentToShipped(ctx, f.ws, f.authorized(tok))
		require.ErrorIs(t, err, ErrAuthorizationRejected)
		assert.Equal(t, string(ShipmentActive), f.archivedStatus(t))
		assert.NotContains(t, f.logText(t), EventShipmentReconciledShipped)
		assert.Zero(t, f.provider.count())
		assert.NotEmpty(t, f.rejections.events, "rejection is audited")
	})

	t.Run("authenticated dry run verifies without consuming or writing", func(t *testing.T) {
		f := newAuthSeamFixture(t)
		req := f.authorized(f.token(f.shipmentID, f.digest, "00cc"))
		req.DryRun = true
		res, err := ReconcileShipmentToShipped(ctx, f.ws, req)
		require.NoError(t, err)
		assert.Equal(t, AuthorizationOutcomeVerified, res.Authorization)
		assert.Zero(t, f.provider.count())
		assert.Equal(t, string(ShipmentActive), f.archivedStatus(t))
	})

	t.Run("unconfigured: v1 confirmation path needs no token and records no approver", func(t *testing.T) {
		f := newAuthSeamFixture(t)
		res, err := ReconcileShipmentToShipped(ctx, f.ws, f.req)
		require.NoError(t, err)
		assert.Equal(t, ShipmentReconcileOutcomeReconciled, res.Outcome)
		assert.Equal(t, AuthorizationOutcomeNotRequired, res.Authorization)
		assert.Zero(t, f.provider.count())
		assert.NotContains(t, f.logText(t), `"approver"`)
	})

	t.Run("unknown enforcement mode is denied fail closed", func(t *testing.T) {
		f := newAuthSeamFixture(t)
		req := f.req
		req.AuthEnforcement = AuthEnforcement("bogus")
		_, err := ReconcileShipmentToShipped(ctx, f.ws, req)
		require.ErrorIs(t, err, ErrAuthPolicyDenied)
		assert.Equal(t, string(ShipmentActive), f.archivedStatus(t))
	})
}
