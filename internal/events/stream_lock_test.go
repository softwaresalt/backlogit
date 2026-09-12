package events

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAcquireItemLogFileLockUsesStableAdvisorySidecar(t *testing.T) {
	locksRoot := t.TempDir()
	firstUnlock, err := acquireItemLogFileLock(context.Background(), locksRoot, "T001")
	require.NoError(t, err)
	lockPath, pathErr := ItemLogLockPath(locksRoot, "T001")
	require.NoError(t, pathErr)

	_, err = acquireItemLogFileLock(context.Background(), locksRoot, "T001")
	require.Error(t, err, "a held advisory lock must remain busy without stale reclamation")
	firstUnlock()

	secondUnlock, err := acquireItemLogFileLock(context.Background(), locksRoot, "T001")
	require.NoError(t, err)
	secondUnlock()
	_, statErr := os.Stat(lockPath)
	require.NoError(t, statErr, "the stable advisory sidecar must remain after release")
}
