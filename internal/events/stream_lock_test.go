package events

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAcquireItemLogFileLock_DoesNotRemoveReplacementAfterStaleReclaim(t *testing.T) {
	logsDir := t.TempDir()
	firstUnlock, err := acquireItemLogFileLock(context.Background(), logsDir, "T001")
	require.NoError(t, err)
	lockPath := itemLogLockSidecarPath(logsDir, "T001")
	staleTime := time.Now().Add(-itemLogLockStaleTTL - time.Second)
	require.NoError(t, os.Chtimes(lockPath, staleTime, staleTime))

	secondUnlock, err := acquireItemLogFileLock(context.Background(), logsDir, "T001")
	require.NoError(t, err)
	firstUnlock()
	_, statErr := os.Stat(lockPath)
	require.NoError(t, statErr, "the original owner must not remove a replacement lock")

	secondUnlock()
	_, statErr = os.Stat(lockPath)
	require.ErrorIs(t, statErr, os.ErrNotExist)
}
