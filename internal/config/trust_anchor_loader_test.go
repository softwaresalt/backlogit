package config_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
)

// loadWithTrustAnchors writes default config plus the given trust_anchors YAML
// block and loads it through config.Load.
func loadWithTrustAnchors(t *testing.T, block string) (*config.WorkspaceConfig, error) {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, config.WriteDefaults(dir))
	cfgPath := filepath.Join(dir, "config.yaml")
	raw, err := os.ReadFile(cfgPath)
	require.NoError(t, err)
	out := append(raw, []byte("\n"+block+"\n")...)
	require.NoError(t, os.WriteFile(cfgPath, out, 0o644))
	return config.Load(context.Background(), dir)
}

const trustAnchorsValid = `trust_anchors:
  - id: a1
    role: release
    algo: ed25519
    public_key_ref: keys/a1.pub
    fingerprint: sha256:aa
    status: active
    not_before: 2026-01-01T00:00:00Z
  - id: a2
    role: release
    algo: ecdsa-p256
    public_key_ref: keys/a2.pub
    fingerprint: sha256:bb
    status: revoked
`

const trustAnchorsDupID = `trust_anchors:
  - id: a1
    algo: ed25519
    public_key_ref: keys/a1.pub
    status: active
  - id: a1
    algo: ed25519
    public_key_ref: keys/a2.pub
    status: active
`

const trustAnchorsBadAlgo = `trust_anchors:
  - id: a1
    algo: rsa-2048
    public_key_ref: keys/a1.pub
    status: active
`

func TestTrustAnchorStatusValidation(t *testing.T) {
	mk := func(status config.TrustAnchorStatus) []config.TrustAnchor {
		return []config.TrustAnchor{{ID: "a1", Algo: "ed25519", PublicKeyRef: "keys/a1.pub", Status: status}}
	}
	for _, ok := range []config.TrustAnchorStatus{"active", "revoked"} {
		require.NoError(t, config.ValidateTrustAnchors(mk(ok)), string(ok))
	}
	for _, bad := range []config.TrustAnchorStatus{"", "Revoked", "ACTIVE", "revokd", " active"} {
		err := config.ValidateTrustAnchors(mk(bad))
		require.ErrorIs(t, err, config.ErrTrustAnchorInvalidStatus, "status %q", bad)
		var ve *config.TrustAnchorValidationError
		require.True(t, errors.As(err, &ve))
		require.Equal(t, "status", ve.Field)
	}
}

func TestTrustAnchorLoaderValidation(t *testing.T) {
	t.Run("valid config loads into structs", func(t *testing.T) {
		cfg, err := loadWithTrustAnchors(t, trustAnchorsValid)
		require.NoError(t, err)
		require.Len(t, cfg.TrustAnchors, 2)
		require.Equal(t, "a1", cfg.TrustAnchors[0].ID)
		require.Equal(t, "ed25519", cfg.TrustAnchors[0].Algo)
		require.Equal(t, "keys/a1.pub", cfg.TrustAnchors[0].PublicKeyRef)
		require.Equal(t, config.TrustAnchorStatusActive, cfg.TrustAnchors[0].Status)
		require.NotNil(t, cfg.TrustAnchors[0].NotBefore)
		require.Equal(t, config.TrustAnchorStatusRevoked, cfg.TrustAnchors[1].Status)
	})

	t.Run("absent trust_anchors loads", func(t *testing.T) {
		cfg, err := loadWithTrustAnchors(t, "")
		require.NoError(t, err)
		require.Empty(t, cfg.TrustAnchors)
	})

	rejects := []struct {
		name     string
		block    string
		sentinel error
		field    string
		index    int
	}{
		{"duplicate id", trustAnchorsDupID, config.ErrTrustAnchorDuplicateID, "id", 1},
		{"disallowed algo", trustAnchorsBadAlgo, config.ErrTrustAnchorDisallowedAlgo, "algo", 0},
	}
	for _, tc := range rejects {
		t.Run("rejects "+tc.name, func(t *testing.T) {
			_, err := loadWithTrustAnchors(t, tc.block)
			require.Error(t, err)
			require.ErrorIs(t, err, tc.sentinel)
			var ve *config.TrustAnchorValidationError
			require.True(t, errors.As(err, &ve), "want *TrustAnchorValidationError, got %T", err)
			require.Equal(t, tc.field, ve.Field)
			require.Equal(t, tc.index, ve.Index)
		})
	}
}
