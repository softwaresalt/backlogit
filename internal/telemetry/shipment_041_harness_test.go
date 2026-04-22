package telemetry

// Harness for 040.003-T: Add fsync to telemetry JSONL harvest writes.
//
// This file uses package telemetry (internal) to access the unexported
// writeTelemetryJSONL function directly.
//
// Contract test: TestWriteTelemetryJSONL_NoDotTmpAfterSuccess verifies that
// writeTelemetryJSONL cleans up all temp files after a successful atomic write.
// Currently passes (existing code already does temp-file-then-rename), and
// serves as a regression boundary once the fsync call is added.

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWriteTelemetryJSONL_NoDotTmpAfterSuccess confirms that writeTelemetryJSONL
// leaves no intermediate .tmp files in the output directory after a successful
// write. The function must fsync before close and rename atomically.
func TestWriteTelemetryJSONL_NoDotTmpAfterSuccess(t *testing.T) {
	dir := t.TempDir()
	jsonlPath := dir + "/telemetry-sessions.jsonl"

	err := writeTelemetryJSONL(
		jsonlPath,
		nil,            // summaries: empty run
		nil,            // toolStats: no tool calls
		nil,            // serverCallsPerSession: no server calls
		time.Now().UTC(),
		nil,            // priorSessions: no prior records
		nil,            // priorTools: no prior tool records
	)
	require.NoError(t, err)

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, e := range entries {
		assert.NotContains(t, e.Name(), ".tmp",
			"no temp files must remain after writeTelemetryJSONL succeeds")
	}

	// Output file must exist and be readable.
	_, statErr := os.Stat(jsonlPath)
	assert.NoError(t, statErr, "telemetry-sessions.jsonl must exist after a successful write")
}
