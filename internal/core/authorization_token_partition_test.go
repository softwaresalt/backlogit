package core_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
)

func TestTokenVerifyPositivePartition(t *testing.T) {
	t.Run("valid token from a trusted issuer authorizes", func(t *testing.T) {
		f := newTokenVerifyFixture(t)
		tok := f.sign(f.token())
		key, err := core.ResolveTokenIssuerKey(f.anchors, tok.Header.KeyID, tokenVerifyNow, allPinned(f.anchors))
		require.NoError(t, err)
		in := core.TokenVerifyInput{
			Token:      tok,
			Key:        key,
			PublicKey:  &core.VerificationKey{Algo: "ed25519", Public: f.pub, Fingerprint: "sha256:t1"},
			ShipmentID: "151-S",
			At:         tokenVerifyNow,
		}
		require.NoError(t, core.VerifyAuthorizationToken(in))
	})

	t.Run("self-minted token from an unpinned issuer is rejected typed", func(t *testing.T) {
		f := newTokenVerifyFixture(t)
		tok := f.sign(f.token())
		// The attacker declares their own anchor in the workspace; the
		// external pin set does not pin it.
		_, err := core.ResolveTokenIssuerKey(f.anchors, tok.Header.KeyID, tokenVerifyNow, core.NewExternalPinSet("sha256:other"))
		require.Error(t, err)
		require.ErrorIs(t, err, core.ErrTokenUntrustedIssuer)
		require.ErrorIs(t, err, core.ErrTrustAnchorNotPinned)
		var verr *core.TokenVerifyError
		require.True(t, errors.As(err, &verr))
		require.Equal(t, "header.key_id", verr.Field)
		require.False(t, errors.Is(err, core.ErrTokenExpired))
	})

	t.Run("unknown key id and absent pin set are untrusted issuer", func(t *testing.T) {
		f := newTokenVerifyFixture(t)
		_, err := core.ResolveTokenIssuerKey(f.anchors, "nobody", tokenVerifyNow, allPinned(f.anchors))
		require.ErrorIs(t, err, core.ErrTokenUntrustedIssuer)
		_, err = core.ResolveTokenIssuerKey(f.anchors, "op-1", tokenVerifyNow, nil)
		require.ErrorIs(t, err, core.ErrTokenUntrustedIssuer)
	})

	t.Run("expired token is rejected typed", func(t *testing.T) {
		f := newTokenVerifyFixture(t)
		tok := f.sign(f.token())
		in := f.input(t, tok)
		in.At = tok.Claims.NotAfter.Add(time.Second)
		err := core.VerifyAuthorizationToken(in)
		require.ErrorIs(t, err, core.ErrTokenExpired)
		require.False(t, errors.Is(err, core.ErrTokenUntrustedIssuer))
	})

	t.Run("a forged signature under a trusted key id is not an issuer error", func(t *testing.T) {
		f := newTokenVerifyFixture(t)
		tok := f.sign(f.token())
		tok.Binding.RequestIdentityDigest = "evil"
		err := core.VerifyAuthorizationToken(f.input(t, tok))
		require.ErrorIs(t, err, core.ErrTokenSignatureInvalid)
	})
}
