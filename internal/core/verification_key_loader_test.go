package core_test

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
)

func pemBlock(t *testing.T, typ string, der []byte) string {
	t.Helper()
	return string(pem.EncodeToMemory(&pem.Block{Type: typ, Bytes: der}))
}

func edKeyPEM(t *testing.T) (pub, priv, fp string) {
	t.Helper()
	pk, sk, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	der, err := x509.MarshalPKIXPublicKey(pk)
	require.NoError(t, err)
	skDER, err := x509.MarshalPKCS8PrivateKey(sk)
	require.NoError(t, err)
	sum := sha256.Sum256(der)
	return pemBlock(t, "PUBLIC KEY", der), pemBlock(t, "PRIVATE KEY", skDER), "sha256:" + hex.EncodeToString(sum[:])
}

func ecKeyPEM(t *testing.T, curve elliptic.Curve) (pub, priv string) {
	t.Helper()
	sk, err := ecdsa.GenerateKey(curve, rand.Reader)
	require.NoError(t, err)
	der, err := x509.MarshalPKIXPublicKey(&sk.PublicKey)
	require.NoError(t, err)
	skDER, err := x509.MarshalECPrivateKey(sk)
	require.NoError(t, err)
	return pemBlock(t, "PUBLIC KEY", der), pemBlock(t, "EC PRIVATE KEY", skDER)
}

func TestVerificationKeyLoader(t *testing.T) {
	root := t.TempDir()
	edPub, edPriv, edFP := edKeyPEM(t)
	ecPub, ecPriv := ecKeyPEM(t, elliptic.P256())
	p384Pub, _ := ecKeyPEM(t, elliptic.P384())

	t.Run("valid public key parses per algo", func(t *testing.T) {
		k, err := core.LoadVerificationKey(root, config.TrustAnchor{ID: "e", Algo: "ed25519", PublicKeyRef: edPub})
		require.NoError(t, err)
		require.NotNil(t, k)
		require.Equal(t, "ed25519", k.Algo)
		require.IsType(t, ed25519.PublicKey{}, k.Public)
		require.Equal(t, edFP, k.Fingerprint)

		k, err = core.LoadVerificationKey(root, config.TrustAnchor{ID: "c", Algo: "ecdsa-p256", PublicKeyRef: ecPub})
		require.NoError(t, err)
		require.NotNil(t, k)
		require.IsType(t, &ecdsa.PublicKey{}, k.Public)
	})

	t.Run("valid public key from contained file", func(t *testing.T) {
		require.NoError(t, os.MkdirAll(filepath.Join(root, "keys"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(root, "keys", "a.pem"), []byte(edPub), 0o600))
		k, err := core.LoadVerificationKey(root, config.TrustAnchor{ID: "f", Algo: "ed25519", PublicKeyRef: "keys/a.pem"})
		require.NoError(t, err)
		require.NotNil(t, k)
		require.Equal(t, edFP, k.Fingerprint)
	})

	t.Run("private key rejected with typed error", func(t *testing.T) {
		for name, tc := range map[string]config.TrustAnchor{
			"ed25519 pkcs8": {ID: "p1", Algo: "ed25519", PublicKeyRef: edPriv},
			"ec private":    {ID: "p2", Algo: "ecdsa-p256", PublicKeyRef: ecPriv},
		} {
			k, err := core.LoadVerificationKey(root, tc)
			require.ErrorIs(t, err, core.ErrVerificationKeyPrivate, name)
			require.Nil(t, k, name)
			var vke *core.VerificationKeyError
			require.ErrorAs(t, err, &vke, name)
			require.Equal(t, tc.ID, vke.AnchorID)
		}
		// Private DER hidden inside a PUBLIC KEY block is still private.
		blk, _ := pem.Decode([]byte(edPriv))
		disguised := pemBlock(t, "PUBLIC KEY", blk.Bytes)
		_, err := core.LoadVerificationKey(root, config.TrustAnchor{ID: "p3", Algo: "ed25519", PublicKeyRef: disguised})
		require.ErrorIs(t, err, core.ErrVerificationKeyPrivate)
		// Private key referenced through a file is rejected too.
		require.NoError(t, os.WriteFile(filepath.Join(root, "keys", "priv.pem"), []byte(edPriv), 0o600))
		_, err = core.LoadVerificationKey(root, config.TrustAnchor{ID: "p4", Algo: "ed25519", PublicKeyRef: "keys/priv.pem"})
		require.ErrorIs(t, err, core.ErrVerificationKeyPrivate)
	})

	t.Run("malformed material rejected with typed error", func(t *testing.T) {
		garbage := pemBlock(t, "PUBLIC KEY", []byte("not der"))
		require.NoError(t, os.WriteFile(filepath.Join(root, "keys", "bad.pem"), []byte("garbage"), 0o600))
		for name, tc := range map[string]struct {
			a   config.TrustAnchor
			err error
		}{
			"bad der":            {config.TrustAnchor{ID: "m1", Algo: "ed25519", PublicKeyRef: garbage}, core.ErrVerificationKeyMalformed},
			"file not pem":       {config.TrustAnchor{ID: "m2", Algo: "ed25519", PublicKeyRef: "keys/bad.pem"}, core.ErrVerificationKeyMalformed},
			"algo mismatch":      {config.TrustAnchor{ID: "m3", Algo: "ecdsa-p256", PublicKeyRef: edPub}, core.ErrVerificationKeyAlgoMismatch},
			"wrong curve":        {config.TrustAnchor{ID: "m4", Algo: "ecdsa-p256", PublicKeyRef: p384Pub}, core.ErrVerificationKeyAlgoMismatch},
			"unsupported algo":   {config.TrustAnchor{ID: "m5", Algo: "none", PublicKeyRef: edPub}, core.ErrVerificationKeyMalformed},
			"escaping path":      {config.TrustAnchor{ID: "m6", Algo: "ed25519", PublicKeyRef: "../outside.pem"}, core.ErrVerificationKeyRefUnsafe},
			"missing file":       {config.TrustAnchor{ID: "m7", Algo: "ed25519", PublicKeyRef: "keys/missing.pem"}, core.ErrVerificationKeyUnreadable},
			"trailing pem bytes": {config.TrustAnchor{ID: "m8", Algo: "ed25519", PublicKeyRef: edPub + "junk"}, core.ErrVerificationKeyMalformed},
		} {
			k, err := core.LoadVerificationKey(root, tc.a)
			require.ErrorIs(t, err, tc.err, name)
			require.Nil(t, k, name)
		}
	})

	t.Run("symlink file rejected", func(t *testing.T) {
		link := filepath.Join(root, "keys", "link.pem")
		if err := os.Symlink(filepath.Join(root, "keys", "a.pem"), link); err != nil {
			t.Skipf("symlink creation unavailable: %v", err)
		}
		k, err := core.LoadVerificationKey(root, config.TrustAnchor{ID: "s1", Algo: "ed25519", PublicKeyRef: "keys/link.pem"})
		require.ErrorIs(t, err, core.ErrVerificationKeyRefUnsafe)
		require.Nil(t, k)
	})
}
