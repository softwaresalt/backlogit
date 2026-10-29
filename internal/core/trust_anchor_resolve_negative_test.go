package core_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
)

// These negative tests are the persistent RED harness (168.010-T). Each one
// reproduces a vulnerable behavior of the pre-hardening resolver/mutation core
// with a PRESENT external pin set and asserts the HARDENED outcome. They fail
// until 168.007/168.008/168.009 land.

type negState struct{ anchors []config.TrustAnchor }

func (s *negState) LoadAnchors(context.Context) ([]config.TrustAnchor, error) {
	return append([]config.TrustAnchor(nil), s.anchors...), nil
}
func (s *negState) WriteConfig(_ context.Context, a []config.TrustAnchor) error {
	s.anchors = append([]config.TrustAnchor(nil), a...)
	return nil
}
func (s *negState) WriteStatus(_ context.Context, c core.TrustAnchorStatusChange) error {
	for i := range s.anchors {
		if s.anchors[i].ID == c.ID {
			s.anchors[i].Status = c.Status
		}
	}
	return nil
}

type negAudit struct{}

func (negAudit) RecordAudit(context.Context, core.TrustAnchorAuditEvent) error { return nil }

type negPins struct{ set *core.ExternalPinSet }

func (p negPins) ExternalPinSet(context.Context) (*core.ExternalPinSet, error) { return p.set, nil }

// A key that was revoked / de-pinned externally is flipped back to active by a
// workspace-local config edit. The resolver must not treat it as trusted.
func TestTrustAnchorResolveNegative_WorkspaceEditFlipsRevokedToActive(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	pins := core.NewExternalPinSet("sha256:other") // present; the flipped key is NOT pinned
	require.NoError(t, core.VerifyTrustAnchorPins(context.Background(),
		[]config.TrustAnchor{{ID: "other", Fingerprint: "sha256:other"}}, negPins{set: pins}))

	// Originally revoked; attacker edits status to active in workspace config.
	flipped := []config.TrustAnchor{{ID: "revived", Role: "attestation-signer", Algo: "ed25519",
		Fingerprint: "sha256:revived", Status: config.TrustAnchorStatusActive}}
	require.False(t, pins.Contains("sha256:revived"))

	_, err := core.ResolveVerificationKey(flipped, core.KeySelector{ID: "revived", Purpose: "attestation-signer"}, now, pins)
	require.Error(t, err, "a de-pinned key re-activated by a workspace-local edit must not resolve as trusted")
}

// A workspace-local edit widens the role and extends the validity window of an
// anchor. The resolver must not accept the in-workspace widening.
func TestTrustAnchorResolveNegative_WorkspaceRoleWideningAndValidityExtension(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	// Externally pinned as an attestation-signer that expired a month ago.
	// The edit re-labels it token-issuer and pushes not_after a year out.
	edited := []config.TrustAnchor{{ID: "signer", Role: "token-issuer", Algo: "ed25519",
		Fingerprint: "sha256:signer", Status: config.TrustAnchorStatusActive,
		NotAfter: tptr(now.Add(365 * 24 * time.Hour))}}
	pins := core.NewExternalPinSet("sha256:signer")
	require.True(t, pins.Contains("sha256:signer"))

	_, err := core.ResolveVerificationKey(edited, core.KeySelector{ID: "signer", Purpose: "token-issuer"}, now, pins)
	require.Error(t, err, "in-workspace role widening / validity extension must not resolve as trusted")
}

// An advisory-only revoke flips workspace status but nothing authoritative
// (the external pin set) revokes the key. It must not report success.
func TestTrustAnchorResolveNegative_AdvisoryRevokeWithoutExternalRevocation(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	target := config.TrustAnchor{ID: "victim", Role: "attestation-signer", Algo: "ed25519", PublicKeyRef: "keys/v.pem",
		Fingerprint: "sha256:victim", Status: config.TrustAnchorStatusActive}
	pins := core.NewExternalPinSet("sha256:victim") // external authority still pins the key

	state := &negState{anchors: []config.TrustAnchor{target}}
	m := &core.TrustAnchorMutator{State: state, Audit: negAudit{}, Pins: negPins{set: pins},
		Approver: core.NewEd25519ApproverVerifier(pub), Now: func() time.Time { return now }}
	ev := core.ApprovalEvidence{Action: core.TrustAnchorActionRevoke, AnchorID: "victim", Approver: "op",
		Expiry: now.Add(time.Hour), Nonce: "n"}
	ev.Signature = ed25519.Sign(priv, ev.SigningPayload())

	_, err = m.Apply(context.Background(), core.TrustAnchorMutation{Action: core.TrustAnchorActionRevoke, AnchorID: "victim", Approval: &ev})
	require.Error(t, err, "revoke must not report success while the external pin set still authorizes the fingerprint")
}
