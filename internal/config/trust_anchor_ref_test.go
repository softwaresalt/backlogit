package config_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
)

func inlinePublicKeyPEM(t *testing.T) string {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	der, err := x509.MarshalPKIXPublicKey(pub)
	require.NoError(t, err)
	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
}

func anchorWithRef(ref string) []config.TrustAnchor {
	return []config.TrustAnchor{{
		ID: "a1", Algo: config.TrustAnchorAlgoEd25519, PublicKeyRef: ref,
		Status: config.TrustAnchorStatusActive,
	}}
}

func TestTrustAnchorRefStructuralContract(t *testing.T) {
	validPEM := inlinePublicKeyPEM(t)
	privPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte{1, 2, 3}}))

	t.Run("accepts inline PEM and contained relative path", func(t *testing.T) {
		for name, ref := range map[string]string{
			"inline pem":    validPEM,
			"relative path": "keys/a1.pub",
			"nested dir":    "keys/release/a1.pem",
			"dot prefix":    "./keys/a1.pub",
		} {
			t.Run(name, func(t *testing.T) {
				require.NoError(t, config.ValidateTrustAnchors(anchorWithRef(ref)))
			})
		}
	})

	rejects := []struct {
		name     string
		ref      string
		sentinel error
	}{
		{"empty", "", config.ErrTrustAnchorRefEmpty},
		{"whitespace only", "  \t ", config.ErrTrustAnchorRefEmpty},
		{"truncated pem", "-----BEGIN PUBLIC KEY-----\nAAAA\n", config.ErrTrustAnchorRefMalformed},
		{"pem bad base64", "-----BEGIN PUBLIC KEY-----\n!!!!\n-----END PUBLIC KEY-----\n", config.ErrTrustAnchorRefMalformed},
		{"private key pem", privPEM, config.ErrTrustAnchorRefMalformed},
		{"pem with trailing data", validPEM + "extra", config.ErrTrustAnchorRefMalformed},
		{"embedded newline path", "keys/a\n1.pub", config.ErrTrustAnchorRefMalformed},
		{"trailing separator", "keys/", config.ErrTrustAnchorRefMalformed},
		{"dot only", ".", config.ErrTrustAnchorRefMalformed},
		{"absolute posix", "/etc/keys/a1.pub", config.ErrTrustAnchorRefDisallowedScheme},
		{"absolute backslash", `\keys\a1.pub`, config.ErrTrustAnchorRefDisallowedScheme},
		{"absolute drive", `C:\keys\a1.pub`, config.ErrTrustAnchorRefDisallowedScheme},
		{"parent traversal", "../keys/a1.pub", config.ErrTrustAnchorRefDisallowedScheme},
		{"embedded traversal", "keys/../../a1.pub", config.ErrTrustAnchorRefDisallowedScheme},
		{"backslash traversal", `keys\..\..\a1.pub`, config.ErrTrustAnchorRefDisallowedScheme},
		{"https url", "https://example.com/a1.pub", config.ErrTrustAnchorRefDisallowedScheme},
		{"file url", "file:///etc/a1.pub", config.ErrTrustAnchorRefDisallowedScheme},
		{"env scheme", "env:KEY", config.ErrTrustAnchorRefDisallowedScheme},
	}
	for _, tc := range rejects {
		t.Run("rejects "+tc.name, func(t *testing.T) {
			err := config.ValidateTrustAnchors(anchorWithRef(tc.ref))
			require.Error(t, err)
			require.ErrorIs(t, err, tc.sentinel)
			var ve *config.TrustAnchorValidationError
			require.True(t, errors.As(err, &ve), "want *TrustAnchorValidationError, got %T", err)
			require.Equal(t, "public_key_ref", ve.Field)
		})
	}
}
