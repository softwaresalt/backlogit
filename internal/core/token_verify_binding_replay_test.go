package core

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
)

// bindingReplayFixture is a workspace-free authorization fixture for the
// 170.014-T binding/replay partition: {wrong-shipment, wrong-request-identity,
// replay}. The valid/self-minted/expired cases belong to 170.006-T.
type bindingReplayFixture struct {
	seam authSeamFixtureLite
}

type authSeamFixtureLite struct {
	priv     ed25519.PrivateKey
	auth     ReconcileAuthorization
	provider *seamNonceProvider
}

func newBindingReplayFixture(t *testing.T) bindingReplayFixture {
	t.Helper()
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
	return bindingReplayFixture{seam: authSeamFixtureLite{
		priv:     priv,
		provider: provider,
		auth: ReconcileAuthorization{
			Key:       key,
			PublicKey: &VerificationKey{Algo: "ed25519", Public: pub, Fingerprint: "sha256:t1"},
			Nonces:    nonces,
			Auditor:   &seamRejectionAuditor{},
		},
	}}
}

func (f bindingReplayFixture) authorization(shipmentID, digest, nonce string) ReconcileAuthorization {
	now := time.Now()
	tok := AuthorizationToken{
		Header:  AuthorizationTokenHeader{Algo: "ed25519", KeyID: "op-1"},
		Claims:  AuthorizationTokenClaims{NotBefore: now.Add(-time.Minute), NotAfter: now.Add(5 * time.Minute), Nonce: nonce},
		Binding: AuthorizationTokenBinding{ShipmentID: shipmentID, RequestIdentityDigest: digest},
	}
	tok.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(f.seam.priv, tok.SigningPayload()))
	auth := f.seam.auth
	auth.Token = tok
	return auth
}

func TestTokenVerifyBindingReplayPartition(t *testing.T) {
	ctx := context.Background()
	const shipmentID, digest = "151-S", "digest-abc"
	distinct := []error{ErrApprovalWrongShipment, ErrApprovalWrongRequestIdentity, ErrApprovalReplay}

	assertOnly := func(t *testing.T, err, want error) {
		t.Helper()
		require.ErrorIs(t, err, ErrAuthorizationRejected, "fail closed")
		require.ErrorIs(t, err, want)
		for _, other := range distinct {
			if other != want {
				require.False(t, errors.Is(err, other), "error must not also match %v", other)
			}
		}
		var typed *ApprovalRejectionError
		require.ErrorAs(t, err, &typed)
	}

	t.Run("wrong shipment is rejected with a typed wrong-shipment error", func(t *testing.T) {
		f := newBindingReplayFixture(t)
		_, err := AuthorizeApproval(ctx, f.authorization("152-S", digest, "01"), shipmentID, digest, false)
		assertOnly(t, err, ErrApprovalWrongShipment)
		require.Zero(t, f.seam.provider.count(), "nonce not consumed by a rejected token")
	})

	t.Run("wrong request identity is rejected with a typed wrong-request-identity error", func(t *testing.T) {
		f := newBindingReplayFixture(t)
		_, err := AuthorizeApproval(ctx, f.authorization(shipmentID, "other-digest", "02"), shipmentID, digest, false)
		assertOnly(t, err, ErrApprovalWrongRequestIdentity)
		require.Zero(t, f.seam.provider.count())
	})

	t.Run("replayed token is rejected with a typed replay error", func(t *testing.T) {
		f := newBindingReplayFixture(t)
		auth := f.authorization(shipmentID, digest, "03")
		outcome, err := AuthorizeApproval(ctx, auth, shipmentID, digest, false)
		require.NoError(t, err)
		require.Equal(t, AuthorizationOutcomeConsumed, outcome)
		_, err = AuthorizeApproval(ctx, auth, shipmentID, digest, false)
		assertOnly(t, err, ErrApprovalReplay)
	})
}
