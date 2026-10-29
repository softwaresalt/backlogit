package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
)

func TestReconcileAuthTokenFlag(t *testing.T) {
	baseArgs := []string{
		"shipment", "reconcile-shipped", "048-S", "--dry-run",
		"--reason", "preview", "--actor", "operator", "--idempotency-key", "idem-1",
		"--merge-sha", "0123456789abcdef0123456789abcdef01234567", "--closure-evidence", "docs/closure/048-S.md",
	}

	t.Run("flag is registered and documented", func(t *testing.T) {
		root := trustAnchorWorkspace(t)
		out, err := runTrustAnchorCLI(t, root, "shipment", "reconcile-shipped", "--help")
		require.NoError(t, err, out)
		require.Contains(t, out, "--authorization-token")
	})

	t.Run("unreadable token file fails before any write", func(t *testing.T) {
		root := trustAnchorWorkspace(t)
		before := snapshotTree(t, root)
		out, err := runTrustAnchorCLI(t, root, append(baseArgs, "--authorization-token", filepath.Join(root, "missing.json"))...)
		require.Error(t, err, out)
		require.ErrorIs(t, err, os.ErrNotExist)
		require.Equal(t, before, snapshotTree(t, root))
	})

	t.Run("malformed token fails closed with a typed parse error", func(t *testing.T) {
		root := trustAnchorWorkspace(t)
		path := filepath.Join(root, "token.json")
		require.NoError(t, os.WriteFile(path, []byte(`{"header":`), 0o600))
		before := snapshotTree(t, root)
		_, err := runTrustAnchorCLI(t, root, append(baseArgs, "--authorization-token", path)...)
		require.ErrorIs(t, err, core.ErrTokenMalformed)
		require.Equal(t, before, snapshotTree(t, root))
	})
}
