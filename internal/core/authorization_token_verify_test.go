package core_test

import (
	"crypto/ed25519"
	"encoding/base64"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
)

var tokenVerifyNow = time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

type tokenVerifyFixture struct {
	pub     ed25519.PublicKey
	priv    ed25519.PrivateKey
	anchors []config.TrustAnchor
}

func newTokenVerifyFixture(t *testing.T) tokenVerifyFixture {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	return tokenVerifyFixture{
		pub:  pub,
		priv: priv,
		anchors: []config.TrustAnchor{{
			ID: "op-1", Role: "token-issuer", Algo: "ed25519", Fingerprint: "sha256:t1",
			Status: config.TrustAnchorStatusActive,
		}},
	}
}

func (f tokenVerifyFixture) token() core.AuthorizationToken {
	return core.AuthorizationToken{
		Header: core.AuthorizationTokenHeader{Algo: "ed25519", KeyID: "op-1"},
		Claims: core.AuthorizationTokenClaims{
			NotBefore: tokenVerifyNow.Add(-time.Minute),
			NotAfter:  tokenVerifyNow.Add(4 * time.Minute),
			Nonce:     "00ff",
		},
		Binding: core.AuthorizationTokenBinding{ShipmentID: "151-S", RequestIdentityDigest: "abc123"},
	}
}

func (f tokenVerifyFixture) sign(tok core.AuthorizationToken) core.AuthorizationToken {
	tok.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(f.priv, tok.SigningPayload()))
	return tok
}

func (f tokenVerifyFixture) input(t *testing.T, tok core.AuthorizationToken) core.TokenVerifyInput {
	t.Helper()
	key, err := core.ResolveVerificationKey(f.anchors, core.KeySelector{ID: "op-1", Purpose: "token-issuer"}, tokenVerifyNow, allPinned(f.anchors))
	require.NoError(t, err)
	return core.TokenVerifyInput{
		Token:      tok,
		Key:        key,
		PublicKey:  &core.VerificationKey{Algo: "ed25519", Public: f.pub, Fingerprint: "sha256:t1"},
		ShipmentID: "151-S",
		At:         tokenVerifyNow,
	}
}

func TestTokenVerify(t *testing.T) {
	t.Run("valid signature verifies via the resolved key", func(t *testing.T) {
		f := newTokenVerifyFixture(t)
		require.NoError(t, core.VerifyAuthorizationToken(f.input(t, f.sign(f.token()))))
	})

	t.Run("signature failures fail closed", func(t *testing.T) {
		f := newTokenVerifyFixture(t)
		_, otherPriv, err := ed25519.GenerateKey(nil)
		require.NoError(t, err)
		wrongKey := f.token()
		wrongKey.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(otherPriv, wrongKey.SigningPayload()))
		tampered := f.sign(f.token())
		tampered.Binding.RequestIdentityDigest = "evil"
		badKeyID := f.token()
		badKeyID.Header.KeyID = "other"
		badKeyID = f.sign(badKeyID)
		cases := map[string]core.AuthorizationToken{
			"wrong key":           wrongKey,
			"tampered claims":     tampered,
			"empty signature":     f.token(),
			"not base64":          func() core.AuthorizationToken { x := f.token(); x.Signature = "!!"; return x }(),
			"key id not resolved": badKeyID,
		}
		for name, tok := range cases {
			t.Run(name, func(t *testing.T) {
				err := core.VerifyAuthorizationToken(f.input(t, tok))
				require.ErrorIs(t, err, core.ErrTokenSignatureInvalid)
				var ve *core.TokenVerifyError
				require.ErrorAs(t, err, &ve)
			})
		}
	})

	t.Run("missing public key fails closed", func(t *testing.T) {
		f := newTokenVerifyFixture(t)
		in := f.input(t, f.sign(f.token()))
		in.PublicKey = nil
		require.ErrorIs(t, core.VerifyAuthorizationToken(in), core.ErrTokenSignatureInvalid)
	})

	t.Run("wrong shipment fails closed", func(t *testing.T) {
		f := newTokenVerifyFixture(t)
		in := f.input(t, f.sign(f.token()))
		in.ShipmentID = "152-S"
		err := core.VerifyAuthorizationToken(in)
		require.ErrorIs(t, err, core.ErrTokenWrongShipment)
		var ve *core.TokenVerifyError
		require.ErrorAs(t, err, &ve)
	})

	t.Run("validity window", func(t *testing.T) {
		f := newTokenVerifyFixture(t)
		cases := []struct {
			name   string
			mutate func(*core.AuthorizationToken)
			at     time.Time
			want   error
		}{
			{"not yet valid", func(x *core.AuthorizationToken) { x.Claims.NotBefore = tokenVerifyNow.Add(time.Minute) }, tokenVerifyNow, core.ErrTokenNotYetValid},
			{"expired", func(x *core.AuthorizationToken) { x.Claims.NotAfter = tokenVerifyNow.Add(-time.Second) }, tokenVerifyNow, core.ErrTokenExpired},
			{"zero not_before", func(x *core.AuthorizationToken) { x.Claims.NotBefore = time.Time{} }, tokenVerifyNow, core.ErrTokenInvalidWindow},
			{"zero not_after", func(x *core.AuthorizationToken) { x.Claims.NotAfter = time.Time{} }, tokenVerifyNow, core.ErrTokenInvalidWindow},
			{"inverted window", func(x *core.AuthorizationToken) { x.Claims.NotAfter = x.Claims.NotBefore }, tokenVerifyNow, core.ErrTokenInvalidWindow},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				tok := f.token()
				tc.mutate(&tok)
				in := f.input(t, f.sign(tok))
				in.At = tc.at
				err := core.VerifyAuthorizationToken(in)
				require.ErrorIs(t, err, tc.want)
				var ve *core.TokenVerifyError
				require.ErrorAs(t, err, &ve)
			})
		}
		t.Run("bounds are inclusive", func(t *testing.T) {
			tok := f.sign(f.token())
			for _, at := range []time.Time{tok.Claims.NotBefore, tok.Claims.NotAfter} {
				in := f.input(t, tok)
				in.At = at
				require.NoError(t, core.VerifyAuthorizationToken(in))
			}
		})
	})
}
