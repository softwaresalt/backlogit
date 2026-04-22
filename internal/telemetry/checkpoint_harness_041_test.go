package telemetry_test

// Harness for 040.003-T (partial): Add fsync to telemetry checkpoint writes.
//
// Contract test: TestSaveCheckpoint_NoDotTmpAfterSuccess verifies that
// SaveCheckpoint leaves no .tmp files after a successful atomic write.
// Currently passes (existing code is already atomic via temp-file-then-rename),
// and serves as a regression boundary once the fsync call is added.

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/telemetry"
)

// TestSaveCheckpoint_NoDotTmpAfterSuccess confirms that SaveCheckpoint leaves
// no intermediate .tmp files in the workspace .backlogit/ directory after a
// successful write.
func TestSaveCheckpoint_NoDotTmpAfterSuccess(t *testing.T) {
	ws := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(ws, ".backlogit"), 0o755))

	cp := &telemetry.HarvestCheckpoint{
		FileOffsets: map[string]int64{"process-001.log": 4096},
		LastHarvest: time.Now().UTC(),
		Version:     1,
	}
	require.NoError(t, telemetry.SaveCheckpoint(ws, cp))

	entries, err := os.ReadDir(filepath.Join(ws, ".backlogit"))
	require.NoError(t, err)
	for _, e := range entries {
		assert.NotContains(t, e.Name(), ".tmp",
			"no temp files must remain after SaveCheckpoint succeeds")
	}
}
