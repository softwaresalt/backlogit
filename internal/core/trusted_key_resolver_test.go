package core_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
)

func tptr(t time.Time) *time.Time { return &t }

func TestTrustedKeyResolver(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	anchors := []config.TrustAnchor{
		{ID: "live", Role: "attestation-signer", Algo: "ed25519", Fingerprint: "sha256:aa", Status: config.TrustAnchorStatusActive,
			NotBefore: tptr(now.Add(-time.Hour)), NotAfter: tptr(now.Add(time.Hour))},
		{ID: "future", Role: "future-role", Algo: "ed25519", Fingerprint: "sha256:bb", Status: config.TrustAnchorStatusActive,
			NotBefore: tptr(now.Add(time.Hour))},
		{ID: "old", Role: "old-role", Algo: "ecdsa-p256", Fingerprint: "sha256:cc", Status: config.TrustAnchorStatusActive,
			NotAfter: tptr(now.Add(-time.Hour))},
		{ID: "open", Role: "open-role", Algo: "ed25519", Fingerprint: "sha256:dd", Status: config.TrustAnchorStatusActive},
		{ID: "edge", Role: "edge-role", Algo: "ed25519", Fingerprint: "sha256:ee", Status: config.TrustAnchorStatusActive,
			NotBefore: tptr(now), NotAfter: tptr(now)},
	}

	tests := []struct {
		name    string
		sel     core.KeySelector
		wantID  string
		wantErr error
	}{
		{"active by id inside window", core.KeySelector{ID: "live"}, "live", nil},
		{"active by role inside window", core.KeySelector{Role: "attestation-signer"}, "live", nil},
		{"no window bounds is valid", core.KeySelector{ID: "open"}, "open", nil},
		{"bounds are inclusive", core.KeySelector{ID: "edge"}, "edge", nil},
		{"not yet valid by id", core.KeySelector{ID: "future"}, "", core.ErrTrustAnchorNotYetValid},
		{"not yet valid by role", core.KeySelector{Role: "future-role"}, "", core.ErrTrustAnchorNotYetValid},
		{"expired by id", core.KeySelector{ID: "old"}, "", core.ErrTrustAnchorExpired},
		{"expired by role", core.KeySelector{Role: "old-role"}, "", core.ErrTrustAnchorExpired},
		{"unknown id", core.KeySelector{ID: "nope"}, "", core.ErrTrustAnchorNotFound},
		{"unknown role", core.KeySelector{Role: "nope"}, "", core.ErrTrustAnchorNotFound},
		{"empty selector", core.KeySelector{}, "", core.ErrKeySelectorInvalid},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := core.ResolveVerificationKey(anchors, tc.sel, now)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				var re *core.TrustAnchorResolveError
				require.ErrorAs(t, err, &re)
				require.Empty(t, got.Anchor.ID)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.wantID, got.Anchor.ID)
		})
	}

	t.Run("resolution is a pure function of its inputs", func(t *testing.T) {
		a, errA := core.ResolveVerificationKey(anchors, core.KeySelector{ID: "live"}, now)
		b, errB := core.ResolveVerificationKey(anchors, core.KeySelector{ID: "live"}, now)
		require.NoError(t, errA)
		require.NoError(t, errB)
		require.Equal(t, a, b)
		require.Equal(t, "live", a.Anchor.ID)
		// Window moves with the supplied time, not the wall clock.
		_, err := core.ResolveVerificationKey(anchors, core.KeySelector{ID: "live"}, now.Add(2*time.Hour))
		require.ErrorIs(t, err, core.ErrTrustAnchorExpired)
	})
}
