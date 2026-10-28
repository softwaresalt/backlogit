package core_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
)

func TestTrustedKeyResolverRevocationAmbiguity(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	anchors := []config.TrustAnchor{
		{ID: "revoked-one", Role: "revoked-role", Algo: "ed25519", Fingerprint: "sha256:r1", Status: config.TrustAnchorStatusRevoked},
		{ID: "dup-a", Role: "dup-role", Algo: "ed25519", Fingerprint: "sha256:da", Status: config.TrustAnchorStatusActive},
		{ID: "dup-b", Role: "dup-role", Algo: "ed25519", Fingerprint: "sha256:db", Status: config.TrustAnchorStatusActive},
		{ID: "rot-old", Role: "rot-role", Algo: "ed25519", Fingerprint: "sha256:o1", Status: config.TrustAnchorStatusRevoked},
		{ID: "rot-new", Role: "rot-role", Algo: "ed25519", Fingerprint: "sha256:n1", Status: config.TrustAnchorStatusActive},
		{ID: "win-a", Role: "win-role", Algo: "ed25519", Fingerprint: "sha256:wa", Status: config.TrustAnchorStatusActive,
			NotAfter: tptr(now.Add(-time.Hour))},
		{ID: "win-b", Role: "win-role", Algo: "ed25519", Fingerprint: "sha256:wb", Status: config.TrustAnchorStatusActive},
	}

	tests := []struct {
		name    string
		sel     core.KeySelector
		wantID  string
		wantErr error
	}{
		{"revoked by id", core.KeySelector{ID: "revoked-one"}, "", core.ErrTrustAnchorRevoked},
		{"revoked by role", core.KeySelector{Role: "revoked-role"}, "", core.ErrTrustAnchorRevoked},
		{"ambiguous active role", core.KeySelector{Role: "dup-role"}, "", core.ErrTrustAnchorAmbiguous},
		{"revoked sibling ignored by role", core.KeySelector{Role: "rot-role"}, "rot-new", nil},
		{"out-of-window sibling does not make role ambiguous", core.KeySelector{Role: "win-role"}, "win-b", nil},
		{"id selects one of an ambiguous role", core.KeySelector{ID: "dup-a"}, "dup-a", nil},
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
}
