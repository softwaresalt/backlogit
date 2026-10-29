package core_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
)

// fakeRevocation is an in-memory, non-filesystem RevocationAuthority.
type fakeRevocation struct {
	tombstones []string
	recordErr  error
}

func (f *fakeRevocation) RecordRevocation(_ context.Context, rec core.RevocationRecord) error {
	if f.recordErr != nil {
		return f.recordErr
	}
	f.tombstones = append(f.tombstones, rec.Fingerprint)
	return nil
}

func (f *fakeRevocation) Tombstones(context.Context) ([]string, error) {
	return append([]string(nil), f.tombstones...), nil
}

// TestTrustAnchorAuthoritativeRevoke covers 168.009-T.
func TestTrustAnchorAuthoritativeRevoke(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	victim := config.TrustAnchor{ID: "victim", Role: "attestation-signer", Algo: "ed25519", PublicKeyRef: "keys/v.pem",
		Fingerprint: "sha256:victim", Status: config.TrustAnchorStatusActive}
	req := func() core.TrustAnchorMutation {
		ev := core.ApprovalEvidence{Action: core.TrustAnchorActionRevoke, AnchorID: "victim", Approver: "op", Expiry: now.Add(time.Hour), Nonce: "n"}
		ev.Signature = ed25519.Sign(priv, ev.SigningPayload())
		return core.TrustAnchorMutation{Action: core.TrustAnchorActionRevoke, AnchorID: "victim", Approval: &ev}
	}
	setup := func(auth core.RevocationAuthority) (*mutFixture, *core.TrustAnchorMutator) {
		f := &mutFixture{anchors: []config.TrustAnchor{victim}}
		return f, &core.TrustAnchorMutator{State: f, Audit: f, Revocation: auth,
			Pins:     pinProv{set: core.NewExternalPinSet("sha256:victim")},
			Approver: core.NewEd25519ApproverVerifier(pub), Now: func() time.Time { return now }}
	}
	pins := core.NewBoundExternalPinSet(core.PinBinding{Fingerprint: "sha256:victim", Role: "attestation-signer"})

	t.Run("no authority: fails closed, surfaces external operation, no workspace change", func(t *testing.T) {
		f, m := setup(nil)
		out, err := m.Apply(context.Background(), req())
		require.ErrorIs(t, err, core.ErrRevocationAuthorityRequired)
		require.Contains(t, err.Error(), "sha256:victim")
		require.False(t, out.StatusWritten || out.RevocationRecorded)
		require.Equal(t, []string{"audit:denied"}, f.log, "no status flip, only a denial audit")
		require.Equal(t, config.TrustAnchorStatusActive, f.anchors[0].Status)
		// The key is not resolved as revoked: nothing authoritative changed.
		_, rerr := core.ResolveVerificationKey(f.anchors, core.KeySelector{ID: "victim"}, now, pins)
		require.NoError(t, rerr)
	})

	t.Run("authority failure is not success and writes no projection", func(t *testing.T) {
		auth := &fakeRevocation{recordErr: errors.New("authority unreachable")}
		f, m := setup(auth)
		out, err := m.Apply(context.Background(), req())
		require.ErrorIs(t, err, core.ErrWriteNotApplied)
		require.ErrorIs(t, err, auth.recordErr)
		require.False(t, out.StatusWritten || out.RevocationRecorded)
		require.Equal(t, config.TrustAnchorStatusActive, f.anchors[0].Status)
		require.NotContains(t, f.log, "status:victim=revoked")
	})

	t.Run("authority records then projection is contained; resolver denies monotonic", func(t *testing.T) {
		auth := &fakeRevocation{}
		f, m := setup(auth)
		out, err := m.Apply(context.Background(), req())
		require.NoError(t, err)
		require.True(t, out.RevocationRecorded && out.StatusWritten)
		require.Equal(t, []string{"sha256:victim"}, auth.tombstones)

		// An attacker flips the workspace status back to active; the external
		// tombstone still denies the key.
		f.anchors[0].Status = config.TrustAnchorStatusActive
		revoked, err := core.ApplyRevocations(context.Background(), pins, auth)
		require.NoError(t, err)
		_, rerr := core.ResolveVerificationKey(f.anchors, core.KeySelector{ID: "victim"}, now, revoked)
		require.ErrorIs(t, rerr, core.ErrTrustAnchorRevoked)
		// The original pin set value is not mutated.
		require.False(t, pins.Tombstoned("sha256:victim"))
	})
}
