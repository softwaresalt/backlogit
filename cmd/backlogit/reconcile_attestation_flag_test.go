package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
)

func TestReconcileAttestationFlag(t *testing.T) {
	baseArgs := []string{
		"shipment", "reconcile-shipped", "048-S", "--dry-run",
		"--reason", "preview", "--actor", "operator", "--idempotency-key", "idem-1",
		"--merge-sha", "0123456789abcdef0123456789abcdef01234567", "--closure-evidence", "docs/closure/048-S.md",
	}

	t.Run("flag is registered and documented", func(t *testing.T) {
		root := trustAnchorWorkspace(t)
		out, err := runTrustAnchorCLI(t, root, "shipment", "reconcile-shipped", "--help")
		require.NoError(t, err, out)
		require.Contains(t, out, "--attestation")
	})

	t.Run("unreadable attestation file fails before any write", func(t *testing.T) {
		root := trustAnchorWorkspace(t)
		before := snapshotTree(t, root)
		out, err := runTrustAnchorCLI(t, root, append(baseArgs, "--attestation", filepath.Join(root, "missing.json"))...)
		require.Error(t, err, out)
		require.ErrorIs(t, err, os.ErrNotExist)
		require.Equal(t, before, snapshotTree(t, root))
	})

	t.Run("malformed attestation fails closed with a typed parse error", func(t *testing.T) {
		root := trustAnchorWorkspace(t)
		path := filepath.Join(root, "att.json")
		require.NoError(t, os.WriteFile(path, []byte(`{"payloadType":`), 0o600))
		before := snapshotTree(t, root)
		_, err := runTrustAnchorCLI(t, root, append(baseArgs, "--attestation", path)...)
		require.ErrorIs(t, err, core.ErrAttestationMalformed)
		require.Equal(t, before, snapshotTree(t, root))
	})

	t.Run("absent external pin set fails closed instead of skipping verification", func(t *testing.T) {
		root := trustAnchorWorkspace(t)
		t.Setenv("BACKLOGIT_TRUST_PINS", "")
		t.Setenv("BACKLOGIT_TRUST_PINS_FILE", "")
		path := filepath.Join(root, "att.json")
		require.NoError(t, os.WriteFile(path, []byte(`{"payloadType":"t","payload":"e30=","signatures":[{"keyid":"live-signer","sig":"c2ln","sig_algo":"ed25519"}]}`), 0o600))
		before := snapshotTree(t, root)
		_, err := runTrustAnchorCLI(t, root, append(baseArgs, "--attestation", path)...)
		require.ErrorIs(t, err, core.ErrTrustPinSetAbsent)
		require.Equal(t, before, snapshotTree(t, root))
	})
}
