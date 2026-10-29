package main

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
)

func TestTrustAnchorVerifyKeyExitCode(t *testing.T) {
	t.Run("trusted key exits zero and writes nothing", func(t *testing.T) {
		root := trustAnchorWorkspace(t)
		t.Setenv("BACKLOGIT_TRUST_PINS", "sha256:live0001")
		before := snapshotTree(t, root)
		out, err := runTrustAnchorCLI(t, root, "trust-anchor", "verify-key", "live-signer")
		require.NoError(t, err, out)
		require.Contains(t, out, "live-signer")
		require.Equal(t, before, snapshotTree(t, root))
	})

	t.Run("untrusted or unknown keys exit non-zero and write nothing", func(t *testing.T) {
		root := trustAnchorWorkspace(t)
		t.Setenv("BACKLOGIT_TRUST_PINS", "sha256:live0001,sha256:old00001")
		before := snapshotTree(t, root)

		_, err := runTrustAnchorCLI(t, root, "trust-anchor", "verify-key", "future-signer") // not pinned and not yet valid
		require.Error(t, err)
		_, err = runTrustAnchorCLI(t, root, "trust-anchor", "verify-key", "old-signer") // pinned but expired
		require.ErrorIs(t, err, core.ErrTrustAnchorExpired)
		_, err = runTrustAnchorCLI(t, root, "trust-anchor", "verify-key", "missing")
		require.ErrorIs(t, err, core.ErrTrustAnchorNotFound)
		require.Equal(t, before, snapshotTree(t, root))
	})

	t.Run("absent external pin set fails closed", func(t *testing.T) {
		root := trustAnchorWorkspace(t)
		t.Setenv("BACKLOGIT_TRUST_PINS", "")
		t.Setenv("BACKLOGIT_TRUST_PINS_FILE", "")
		_, err := runTrustAnchorCLI(t, root, "trust-anchor", "verify-key", "live-signer")
		require.ErrorIs(t, err, core.ErrTrustPinSetAbsent)
	})
}
