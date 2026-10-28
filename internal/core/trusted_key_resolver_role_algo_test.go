package core_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
)

func TestTrustedKeyResolverRoleScopedAlgo(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	anchors := []config.TrustAnchor{
		{ID: "tok", Role: "token-issuer", Algo: "ecdsa-p256", Fingerprint: "sha256:t1", Status: config.TrustAnchorStatusActive},
		{ID: "att", Role: "attestation-signer", Algo: "ed25519", Fingerprint: "sha256:a1", Status: config.TrustAnchorStatusActive},
	}

	t.Run("role mismatch is rejected", func(t *testing.T) {
		got, err := core.ResolveVerificationKey(anchors, core.KeySelector{ID: "tok", Purpose: "attestation-signer"}, now)
		require.ErrorIs(t, err, core.ErrTrustAnchorRoleMismatch)
		var re *core.TrustAnchorResolveError
		require.ErrorAs(t, err, &re)
		require.Empty(t, got.Anchor.ID)
	})

	t.Run("role match carries pinned algo", func(t *testing.T) {
		got, err := core.ResolveVerificationKey(anchors, core.KeySelector{ID: "att", Purpose: "attestation-signer"}, now)
		require.NoError(t, err)
		require.Equal(t, "att", got.Anchor.ID)
		require.Equal(t, "ed25519", got.Algo)

		byRole, err := core.ResolveVerificationKey(anchors, core.KeySelector{Role: "token-issuer", Purpose: "token-issuer"}, now)
		require.NoError(t, err)
		require.Equal(t, "ecdsa-p256", byRole.Algo)
	})

	t.Run("role selector with different purpose is rejected", func(t *testing.T) {
		_, err := core.ResolveVerificationKey(anchors, core.KeySelector{Role: "token-issuer", Purpose: "attestation-signer"}, now)
		require.ErrorIs(t, err, core.ErrTrustAnchorRoleMismatch)
	})

	t.Run("declared algo must equal pinned algo", func(t *testing.T) {
		key, err := core.ResolveVerificationKey(anchors, core.KeySelector{ID: "att", Purpose: "attestation-signer"}, now)
		require.NoError(t, err)
		require.NoError(t, key.VerifyDeclaredAlgo("ed25519"))
		for _, declared := range []string{"none", "None", "NONE", "ecdsa-p256", "ED25519", "hs256"} {
			require.ErrorIs(t, key.VerifyDeclaredAlgo(declared), core.ErrTrustAnchorAlgoMismatch, declared)
		}
	})
}
