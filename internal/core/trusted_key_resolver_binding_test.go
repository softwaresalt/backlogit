package core_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
)

// TestTrustedKeyResolverExternalRoleValidityBinding covers 168.008-T: the
// external pin carries role and validity and the resolver fails closed on any
// in-workspace divergence that widens them.
func TestTrustedKeyResolverExternalRoleValidityBinding(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	extNB, extNA := now.Add(-time.Hour), now.Add(time.Hour)
	pins := core.NewBoundExternalPinSet(core.PinBinding{Fingerprint: "sha256:k", Role: "attestation-signer", NotBefore: &extNB, NotAfter: &extNA})
	base := config.TrustAnchor{ID: "k", Role: "attestation-signer", Algo: "ed25519", Fingerprint: "sha256:k",
		Status: config.TrustAnchorStatusActive, NotBefore: tptr(extNB), NotAfter: tptr(extNA)}
	sel := core.KeySelector{ID: "k"}

	tests := []struct {
		name   string
		mutate func(a *config.TrustAnchor)
		want   error
	}{
		{"matching binding resolves", func(*config.TrustAnchor) {}, nil},
		{"workspace may narrow window", func(a *config.TrustAnchor) { a.NotAfter = tptr(extNA.Add(-time.Minute)) }, nil},
		{"role widening denied", func(a *config.TrustAnchor) { a.Role = "token-issuer" }, core.ErrTrustAnchorBindingMismatch},
		{"not_after extension denied", func(a *config.TrustAnchor) { a.NotAfter = tptr(extNA.Add(24 * time.Hour)) }, core.ErrTrustAnchorBindingMismatch},
		{"removed not_after denied", func(a *config.TrustAnchor) { a.NotAfter = nil }, core.ErrTrustAnchorBindingMismatch},
		{"earlier not_before denied", func(a *config.TrustAnchor) { a.NotBefore = tptr(extNB.Add(-time.Hour)) }, core.ErrTrustAnchorBindingMismatch},
		{"removed not_before denied", func(a *config.TrustAnchor) { a.NotBefore = nil }, core.ErrTrustAnchorBindingMismatch},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := base
			tc.mutate(&a)
			got, err := core.ResolveVerificationKey([]config.TrustAnchor{a}, sel, now, pins)
			if tc.want == nil {
				require.NoError(t, err)
				require.Equal(t, "k", got.Anchor.ID)
				return
			}
			require.ErrorIs(t, err, tc.want)
		})
	}

	t.Run("externally expired key stays expired despite workspace extension", func(t *testing.T) {
		expired := now.Add(-time.Hour)
		p := core.NewBoundExternalPinSet(core.PinBinding{Fingerprint: "sha256:k", Role: "attestation-signer", NotAfter: &expired})
		a := base
		a.NotAfter = tptr(now.Add(365 * 24 * time.Hour))
		_, err := core.ResolveVerificationKey([]config.TrustAnchor{a}, sel, now, p)
		require.ErrorIs(t, err, core.ErrTrustAnchorBindingMismatch)
	})

	t.Run("pin without binding fails closed", func(t *testing.T) {
		_, err := core.ResolveVerificationKey([]config.TrustAnchor{base}, sel, now, core.NewExternalPinSet("sha256:k"))
		require.ErrorIs(t, err, core.ErrTrustAnchorBindingUnverifiable)
	})
}
