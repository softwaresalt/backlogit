package telemetry_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/telemetry"
)

func setupTelemetryHarvestWorkspace(t *testing.T) (workspacePath, copilotPath string) {
	t.Helper()
	root := t.TempDir()
	workspacePath = filepath.Join(root, "workspace")
	copilotPath = filepath.Join(root, ".copilot")
	backlogitDir := filepath.Join(workspacePath, ".backlogit")
	logsDir := filepath.Join(copilotPath, "logs")
	require.NoError(t, os.MkdirAll(backlogitDir, 0o755))
	require.NoError(t, os.MkdirAll(logsDir, 0o755))
	return workspacePath, copilotPath
}

func writeSampleProcessLog(t *testing.T, logsDir string) {
	t.Helper()
	content := `2026-04-09T00:00:01.000Z [info] startup
2026-04-09T00:00:02.000Z [telemetry] {"event":"cli.model_call","session_id":"sess-h1","request_id":"req-h1","model":"claude-sonnet-4","prompt_tokens_count":1000,"completion_tokens_count":500,"total_tokens_count":1500,"cached_tokens_count":200,"duration_ms":1200}
2026-04-09T00:00:03.000Z [telemetry] {"event":"cli.tool_call","session_id":"sess-h1","model_call_id":"req-h1","tool_name":"backlogit_create_item","result_type":"text","duration_ms":45}
2026-04-09T00:00:04.000Z [telemetry] {"event":"cli.model_call","session_id":"sess-h2","request_id":"req-h2","model":"gpt-5.1","prompt_tokens_count":300,"completion_tokens_count":100,"total_tokens_count":400,"cached_tokens_count":0,"duration_ms":350}
`
	require.NoError(t, os.WriteFile(filepath.Join(logsDir, "process-001.log"), []byte(content), 0o644))
}

func TestHarvestTelemetry_ProducesSessionSummaries(t *testing.T) {
	workspacePath, copilotPath := setupTelemetryHarvestWorkspace(t)
	writeSampleProcessLog(t, filepath.Join(copilotPath, "logs"))

	sqliteDB, err := db.Open(filepath.Join(workspacePath, ".backlogit", "index.db"))
	require.NoError(t, err)
	defer sqliteDB.Close()

	result, err := telemetry.HarvestTelemetry(context.Background(), workspacePath, copilotPath, sqliteDB, telemetry.HarvestOptions{})
	require.NoError(t, err)
	assert.Equal(t, 2, result.SessionsHarvested, "expected 2 sessions from sample log")
	assert.Greater(t, result.TotalTokens, 0)

	// telemetry-sessions.jsonl should exist
	jsonlPath := filepath.Join(workspacePath, ".backlogit", "telemetry-sessions.jsonl")
	assert.FileExists(t, jsonlPath)
}

func TestHarvestTelemetry_ReHarvestIsIdempotent(t *testing.T) {
	workspacePath, copilotPath := setupTelemetryHarvestWorkspace(t)
	writeSampleProcessLog(t, filepath.Join(copilotPath, "logs"))

	sqliteDB, err := db.Open(filepath.Join(workspacePath, ".backlogit", "index.db"))
	require.NoError(t, err)
	defer sqliteDB.Close()

	r1, err := telemetry.HarvestTelemetry(context.Background(), workspacePath, copilotPath, sqliteDB, telemetry.HarvestOptions{})
	require.NoError(t, err)

	r2, err := telemetry.HarvestTelemetry(context.Background(), workspacePath, copilotPath, sqliteDB, telemetry.HarvestOptions{})
	require.NoError(t, err)

	assert.Equal(t, r1.SessionsHarvested, r2.SessionsHarvested, "re-harvest should yield same session count")
	assert.Equal(t, r1.TotalTokens, r2.TotalTokens, "re-harvest should yield same token totals")
}

func TestHarvestTelemetry_MissingCopilotDir(t *testing.T) {
	workspacePath := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(workspacePath, ".backlogit"), 0o755))

	sqliteDB, err := db.Open(filepath.Join(workspacePath, ".backlogit", "index.db"))
	require.NoError(t, err)
	defer sqliteDB.Close()

	_, err = telemetry.HarvestTelemetry(context.Background(), workspacePath, "/nonexistent/.copilot", sqliteDB, telemetry.HarvestOptions{})
	require.Error(t, err)
	assert.ErrorIs(t, err, errors.ErrTelemetrySourceMissing)
}
