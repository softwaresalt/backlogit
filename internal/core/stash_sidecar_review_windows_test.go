//go:build windows

package core

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestReviewStashRejectsJunctionComponent(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(root, "outside")
	require.NoError(t, os.Mkdir(outside, 0o755))
	target := filepath.Join(outside, "stash.jsonl.lock")
	before := []byte("junction target must remain unchanged")
	require.NoError(t, os.WriteFile(target, before, 0o600))
	old := time.Now().Add(-2 * staleLockTTL)
	require.NoError(t, os.Chtimes(target, old, old))
	link := filepath.Join(root, "junction")
	require.NoError(t, createReconcileTestJunction(t, link, outside))
	lockPath := filepath.Join(link, "stash.jsonl.lock")
	file, err := acquireStashSidecarFile(lockPath)
	if file != nil {
		require.NoError(t, file.Close())
	}
	require.Error(t, err)
	file, busy, err := openStashLockHandle(lockPath)
	if file != nil {
		require.NoError(t, file.Close())
	}
	require.False(t, busy)
	require.Error(t, err)
	after, err := os.ReadFile(target)
	require.NoError(t, err)
	require.Equal(t, before, after)
}
