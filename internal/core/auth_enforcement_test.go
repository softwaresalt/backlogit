package core_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
)

type fakeRequiredAuth struct {
	enabled bool
	err     error
}

func (f fakeRequiredAuth) RequiredAuthEnabled(context.Context) (bool, error) {
	return f.enabled, f.err
}

func writePolicy(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "auth-policy.yaml")
	if content != "" {
		require.NoError(t, os.WriteFile(p, []byte(content), 0o600))
	}
	return p
}

func TestAuthEnforcementRequiredAuthFailClosed(t *testing.T) {
	ctx := context.Background()

	t.Run("externally required with missing policy denies", func(t *testing.T) {
		_, err := core.ResolveAuthEnforcement(ctx, fakeRequiredAuth{enabled: true}, writePolicy(t, ""))
		require.ErrorIs(t, err, core.ErrAuthPolicyDenied)
		require.ErrorIs(t, err, core.ErrAuthPolicyMissing)
	})

	t.Run("externally required with malformed policy denies", func(t *testing.T) {
		for _, content := range []string{"required: [unterminated\n", "required: maybe\n", "unknown: 1\n", "- a\n"} {
			_, err := core.ResolveAuthEnforcement(ctx, fakeRequiredAuth{enabled: true}, writePolicy(t, content))
			require.ErrorIs(t, err, core.ErrAuthPolicyDenied, content)
			require.ErrorIs(t, err, core.ErrAuthPolicyMalformed, content)
		}
	})

	t.Run("externally required cannot be downgraded by policy false", func(t *testing.T) {
		got, err := core.ResolveAuthEnforcement(ctx, fakeRequiredAuth{enabled: true}, writePolicy(t, "required: false\n"))
		require.NoError(t, err)
		require.Equal(t, core.AuthEnforcementAuthenticated, got)
	})

	t.Run("externally required with valid policy is authenticated", func(t *testing.T) {
		got, err := core.ResolveAuthEnforcement(ctx, fakeRequiredAuth{enabled: true}, writePolicy(t, "required: true\n"))
		require.NoError(t, err)
		require.Equal(t, core.AuthEnforcementAuthenticated, got)
	})

	t.Run("unassertable external state denies", func(t *testing.T) {
		_, err := core.ResolveAuthEnforcement(ctx, nil, writePolicy(t, "required: true\n"))
		require.ErrorIs(t, err, core.ErrRequiredAuthUnassertable)
		_, err = core.ResolveAuthEnforcement(ctx, fakeRequiredAuth{err: errors.New("pin root down")}, writePolicy(t, "required: true\n"))
		require.ErrorIs(t, err, core.ErrRequiredAuthUnassertable)
		require.ErrorIs(t, err, core.ErrAuthPolicyDenied)
	})

	t.Run("not externally required keeps v1 behavior", func(t *testing.T) {
		got, err := core.ResolveAuthEnforcement(ctx, fakeRequiredAuth{}, writePolicy(t, ""))
		require.NoError(t, err)
		require.Equal(t, core.AuthEnforcementConfirmation, got)
		got, err = core.ResolveAuthEnforcement(ctx, fakeRequiredAuth{}, writePolicy(t, "required: true\n"))
		require.NoError(t, err)
		require.Equal(t, core.AuthEnforcementAuthenticated, got)
	})
}
