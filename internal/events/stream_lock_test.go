package events

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAcquireItemLogFileLockUsesStableAdvisorySidecar(t *testing.T) {
	logsDir := t.TempDir()
	firstUnlock, err := acquireItemLogFileLock(context.Background(), logsDir, "T001")
	require.NoError(t, err)
	lockPath := itemLogLockSidecarPath(logsDir, "T001")

	_, err = acquireItemLogFileLock(context.Background(), logsDir, "T001")
	require.Error(t, err, "a held advisory lock must remain busy without stale reclamation")
	firstUnlock()

	secondUnlock, err := acquireItemLogFileLock(context.Background(), logsDir, "T001")
	require.NoError(t, err)
	secondUnlock()
	_, statErr := os.Stat(lockPath)
	require.NoError(t, statErr, "the stable advisory sidecar must remain after release")
}
