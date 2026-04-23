package events_test

// Harness for 040.002-T: Add fsync to hook checkpoint writes.
//
// Contract test: TestCheckpointStore_SaveCheckpoint_NoDotTmpAfterSuccess
// verifies the atomic write contract — no temp files remain after a successful
// SaveCheckpoint. Currently passes (existing code is already atomic), but
// serves as a regression boundary once the fsync call is added.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/events"
)

// TestCheckpointStore_SaveCheckpoint_NoDotTmpAfterSuccess confirms that
// SaveCheckpoint leaves no intermediate .tmp files in the checkpoint directory
// after a successful write (atomicity + cleanup contract).
func TestCheckpointStore_SaveCheckpoint_NoDotTmpAfterSuccess(t *testing.T) {
	dir := t.TempDir()
	cs := events.NewCheckpointStore(dir)

	require.NoError(t, cs.SaveCheckpoint("stage", 42))

	checkpointDir := filepath.Join(dir, "runtime", "hooks")
	entries, err := os.ReadDir(checkpointDir)
	require.NoError(t, err)
	for _, e := range entries {
		assert.NotContains(t, e.Name(), ".tmp",
			"no temp files must remain after SaveCheckpoint succeeds")
	}
}
