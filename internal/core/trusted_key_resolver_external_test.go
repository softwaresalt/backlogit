package core_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
)

// TestTrustedKeyResolverExternalPinAuthority covers 168.007-T: the external
// pin set is authoritative; in-workspace status is advisory and can only
// restrict, never grant, trust.
func TestTrustedKeyResolverExternalPinAuthority(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	anchors := []config.TrustAnchor{
		{ID: "pinned", Role: "attestation-signer", Algo: "ed25519", Fingerprint: "sha256:p", Status: config.TrustAnchorStatusActive},
		{ID: "depinned", Role: "attestation-signer", Algo: "ed25519", Fingerprint: "sha256:d", Status: config.TrustAnchorStatusActive},
		{ID: "revoked-pinned", Role: "revoked-role", Algo: "ed25519", Fingerprint: "sha256:rp", Status: config.TrustAnchorStatusRevoked},
		{ID: "revoked-depinned", Role: "rd-role", Algo: "ed25519", Fingerprint: "sha256:rd", Status: config.TrustAnchorStatusRevoked},
	}
	pins := core.NewBoundExternalPinSet(
		core.PinBinding{Fingerprint: "sha256:p", Role: "attestation-signer"},
		core.PinBinding{Fingerprint: "sha256:rp", Role: "revoked-role"})

	t.Run("absent pin set fails closed for every selector", func(t *testing.T) {
		for _, sel := range []core.KeySelector{{ID: "pinned"}, {Role: "attestation-signer"}, {ID: "nope"}, {}} {
			_, err := core.ResolveVerificationKey(anchors, sel, now, nil)
			require.ErrorIs(t, err, core.ErrTrustPinSetAbsent)
		}
	})

	t.Run("de-pinned active key is not trusted", func(t *testing.T) {
		_, err := core.ResolveVerificationKey(anchors, core.KeySelector{ID: "depinned"}, now, pins)
		require.ErrorIs(t, err, core.ErrTrustAnchorNotPinned)
		var re *core.TrustAnchorResolveError
		require.ErrorAs(t, err, &re)
	})

	t.Run("pinned active key resolves", func(t *testing.T) {
		got, err := core.ResolveVerificationKey(anchors, core.KeySelector{ID: "pinned"}, now, pins)
		require.NoError(t, err)
		require.Equal(t, "pinned", got.Anchor.ID)
	})

	t.Run("revoked status stays revoked even when still pinned", func(t *testing.T) {
		_, err := core.ResolveVerificationKey(anchors, core.KeySelector{ID: "revoked-pinned"}, now, pins)
		require.ErrorIs(t, err, core.ErrTrustAnchorRevoked)
	})

	t.Run("revoked and de-pinned is never trusted", func(t *testing.T) {
		_, err := core.ResolveVerificationKey(anchors, core.KeySelector{ID: "revoked-depinned"}, now, pins)
		require.Error(t, err)
	})

	t.Run("role selection ignores de-pinned siblings", func(t *testing.T) {
		got, err := core.ResolveVerificationKey(anchors, core.KeySelector{Role: "attestation-signer"}, now, pins)
		require.NoError(t, err)
		require.Equal(t, "pinned", got.Anchor.ID)
	})

	t.Run("role with only de-pinned anchors is not trusted", func(t *testing.T) {
		_, err := core.ResolveVerificationKey(anchors, core.KeySelector{Role: "rd-role"}, now, core.NewBoundExternalPinSet(core.PinBinding{Fingerprint: "sha256:p", Role: "attestation-signer"}))
		require.Error(t, err)
	})

	t.Run("revocation via de-pin is monotonic", func(t *testing.T) {
		// Removing a fingerprint from the pin set revokes it; flipping the
		// workspace status back to active cannot restore trust.
		shrunk := core.NewBoundExternalPinSet(core.PinBinding{Fingerprint: "sha256:rp", Role: "revoked-role"})
		flipped := append([]config.TrustAnchor(nil), anchors...)
		flipped[0].Status = config.TrustAnchorStatusActive
		_, err := core.ResolveVerificationKey(flipped, core.KeySelector{ID: "pinned"}, now, shrunk)
		require.ErrorIs(t, err, core.ErrTrustAnchorNotPinned)
	})
}
