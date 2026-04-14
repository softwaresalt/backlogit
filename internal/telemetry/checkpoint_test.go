package telemetry_test

// Harness for 031.001-T: Incremental Harvest with Byte-Offset Checkpoints.
//
// All tests in this file will FAIL until checkpoint.go is fully implemented.
// They are intentionally written in the red phase to drive TDD.

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/telemetry"
)

// ---- LoadCheckpoint / SaveCheckpoint ------------------------------------------

func TestLoadCheckpoint_MissingFile_ReturnsZeroCheckpoint(t *testing.T) {
	// A missing checkpoint file is not an error — the caller treats it as
	// "no prior harvest" and processes all logs from offset 0.
	ws := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(ws, ".backlogit"), 0o755))

	cp, err := telemetry.LoadCheckpoint(ws)
	require.NoError(t, err, "missing checkpoint file must not return an error")
	require.NotNil(t, cp)
	assert.Empty(t, cp.FileOffsets, "zero checkpoint must have no file offsets")
	assert.True(t, cp.LastHarvest.IsZero(), "zero checkpoint must have a zero LastHarvest")
}

func TestLoadCheckpoint_CorruptJSON_ReturnsZeroCheckpoint(t *testing.T) {
	// Corrupt JSON is treated as missing — log a warning and return a zero
	// checkpoint so the next harvest re-processes all files.
	ws := t.TempDir()
	backlogitDir := filepath.Join(ws, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogitDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(backlogitDir, ".telemetry-checkpoint.json"),
		[]byte("not valid json }{"),
		0o644,
	))

	cp, err := telemetry.LoadCheckpoint(ws)
	require.NoError(t, err, "corrupt checkpoint file must not return an error")
	require.NotNil(t, cp)
	assert.Empty(t, cp.FileOffsets)
}

func TestSaveAndLoadCheckpoint_RoundtripPreservesOffsets(t *testing.T) {
	ws := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(ws, ".backlogit"), 0o755))

	now := time.Now().UTC().Truncate(time.Second)
	original := &telemetry.HarvestCheckpoint{
		FileOffsets: map[string]int64{
			"process-001.log": 4096,
			"process-002.log": 8192,
		},
		LastHarvest: now,
		Version:     1,
	}

	require.NoError(t, telemetry.SaveCheckpoint(ws, original))

	loaded, err := telemetry.LoadCheckpoint(ws)
	require.NoError(t, err)
	assert.Equal(t, original.FileOffsets, loaded.FileOffsets)
	assert.Equal(t, original.LastHarvest.Unix(), loaded.LastHarvest.Unix())
	assert.Equal(t, original.Version, loaded.Version)
}

func TestSaveCheckpoint_AtomicWrite_DoesNotCorruptOnPartialWrite(t *testing.T) {
	// SaveCheckpoint must use temp-file-then-rename for atomicity.
	// We can't easily simulate a partial write in a unit test, but we can
	// assert that no .tmp files are left behind after a successful save.
	ws := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(ws, ".backlogit"), 0o755))

	cp := &telemetry.HarvestCheckpoint{
		FileOffsets: map[string]int64{"a.log": 100},
		LastHarvest: time.Now().UTC(),
		Version:     1,
	}
	require.NoError(t, telemetry.SaveCheckpoint(ws, cp))

	entries, err := os.ReadDir(filepath.Join(ws, ".backlogit"))
	require.NoError(t, err)
	for _, e := range entries {
		assert.NotContains(t, e.Name(), ".tmp", "no temp files should remain after SaveCheckpoint")
	}
}

// ---- HarvestOptions (Force flag) ----------------------------------------------

func TestHarvestTelemetry_Force_ReprocessesAllLogs(t *testing.T) {
	// With Force: true, the harvest must process all log content regardless of
	// any existing checkpoint — byte offsets are ignored.
	workspacePath, copilotPath := setupTelemetryHarvestWorkspace(t)
	writeSampleProcessLog(t, filepath.Join(copilotPath, "logs"))

	sqliteDB, err := db.Open(filepath.Join(workspacePath, ".backlogit", "index.db"))
	require.NoError(t, err)
	defer sqliteDB.Close()

	// Seed a checkpoint that would skip the only log file if respected.
	require.NoError(t, os.MkdirAll(filepath.Join(workspacePath, ".backlogit"), 0o755))
	bigOffset := &telemetry.HarvestCheckpoint{
		FileOffsets: map[string]int64{
			"process-001.log": 999999,
		},
		LastHarvest: time.Now().UTC(),
		Version:     1,
	}
	require.NoError(t, telemetry.SaveCheckpoint(workspacePath, bigOffset))

	result, err := telemetry.HarvestTelemetry(
		context.Background(), workspacePath, copilotPath, sqliteDB,
		telemetry.HarvestOptions{Force: true},
	)
	require.NoError(t, err)
	assert.Equal(t, 2, result.SessionsHarvested, "Force must re-process all sessions despite checkpoint")
}

func TestHarvestTelemetry_Incremental_SkipsProcessedBytes(t *testing.T) {
	// With Force: false and an existing checkpoint, already-processed bytes must
	// be skipped so new-only content is harvested.
	workspacePath, copilotPath := setupTelemetryHarvestWorkspace(t)
	logsDir := filepath.Join(copilotPath, "logs")
	writeSampleProcessLog(t, logsDir)

	sqliteDB, err := db.Open(filepath.Join(workspacePath, ".backlogit", "index.db"))
	require.NoError(t, err)
	defer sqliteDB.Close()

	// First (full) harvest to establish checkpoint.
	r1, err := telemetry.HarvestTelemetry(
		context.Background(), workspacePath, copilotPath, sqliteDB,
		telemetry.HarvestOptions{},
	)
	require.NoError(t, err)
	require.Equal(t, 2, r1.SessionsHarvested)

	// Second harvest with no new log content must produce 0 new sessions.
	r2, err := telemetry.HarvestTelemetry(
		context.Background(), workspacePath, copilotPath, sqliteDB,
		telemetry.HarvestOptions{Force: false},
	)
	require.NoError(t, err)
	// After incremental harvest with no new content, sessions merged = same total.
	// The key assertion is that we don't double-count.
	assert.Equal(t, r1.SessionsHarvested, r2.SessionsHarvested,
		"incremental harvest must not create duplicate sessions")
}

func TestHarvestTelemetry_NewLogFile_FullyProcessed(t *testing.T) {
	// A log file that did not exist during the first harvest must be fully
	// processed during the second incremental harvest.
	workspacePath, copilotPath := setupTelemetryHarvestWorkspace(t)
	logsDir := filepath.Join(copilotPath, "logs")
	writeSampleProcessLog(t, logsDir) // process-001.log: 2 sessions

	sqliteDB, err := db.Open(filepath.Join(workspacePath, ".backlogit", "index.db"))
	require.NoError(t, err)
	defer sqliteDB.Close()

	// First harvest: 2 sessions from process-001.log.
	r1, err := telemetry.HarvestTelemetry(
		context.Background(), workspacePath, copilotPath, sqliteDB,
		telemetry.HarvestOptions{},
	)
	require.NoError(t, err)
	assert.Equal(t, 2, r1.SessionsHarvested)

	// Add a second log file with a new session.
	newLog := `2026-04-10T00:00:01.000Z [info] startup
2026-04-10T00:00:02.000Z [telemetry] {"event":"cli.model_call","session_id":"sess-new","request_id":"req-new","model":"gpt-5.1","prompt_tokens_count":200,"completion_tokens_count":50,"total_tokens_count":250,"cached_tokens_count":0,"duration_ms":100}
`
	require.NoError(t, os.WriteFile(filepath.Join(logsDir, "process-002.log"), []byte(newLog), 0o644))

	r2, err := telemetry.HarvestTelemetry(
		context.Background(), workspacePath, copilotPath, sqliteDB,
		telemetry.HarvestOptions{Force: false},
	)
	require.NoError(t, err)
	assert.Equal(t, 3, r2.SessionsHarvested,
		"incremental harvest must include the session from the newly added log file")
}
