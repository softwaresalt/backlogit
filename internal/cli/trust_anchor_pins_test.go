package cli

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
)

func TestBuildPinSetBindings(t *testing.T) {
	t.Run("bound entry carries role and window", func(t *testing.T) {
		set, err := buildPinSet([]string{"sha256:a=attestation-signer;not_before=2026-01-01T00:00:00Z;not_after=2027-01-01T00:00:00Z", "sha256:bare"})
		require.NoError(t, err)
		b, ok := set.Binding("sha256:a")
		require.True(t, ok)
		require.Equal(t, "attestation-signer", b.Role)
		require.Equal(t, time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC), b.NotAfter.UTC())
		require.True(t, set.Contains("sha256:bare"))
		_, ok = set.Binding("sha256:bare")
		require.False(t, ok, "bare pin is unbound and resolves fail closed")
	})
	for _, bad := range []string{"sha256:a=", "=role", "sha256:a=r;not_after=nope", "sha256:a=r;bogus=2026-01-01T00:00:00Z"} {
		t.Run("malformed "+bad, func(t *testing.T) {
			_, err := buildPinSet([]string{bad})
			require.ErrorIs(t, err, core.ErrTrustPinSetAbsent)
		})
	}
}
