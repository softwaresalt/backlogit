package telemetry_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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
	workspacePath = filepath.Join(root, "workspace", ".backlogit")
	copilotPath = filepath.Join(root, ".copilot")
	logsDir := filepath.Join(copilotPath, "logs")
	require.NoError(t, os.MkdirAll(workspacePath, 0o755))
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

	sqliteDB, err := db.Open(filepath.Join(workspacePath, "index.db"))
	require.NoError(t, err)
	defer sqliteDB.Close()

	result, err := telemetry.HarvestTelemetry(context.Background(), workspacePath, copilotPath, sqliteDB, telemetry.HarvestOptions{})
	require.NoError(t, err)
	assert.Equal(t, 2, result.SessionsHarvested, "expected 2 sessions from sample log")
	assert.Greater(t, result.TotalTokens, 0)

	// telemetry-sessions.jsonl should exist
	jsonlPath := filepath.Join(workspacePath, "telemetry-sessions.jsonl")
	assert.FileExists(t, jsonlPath)
}

func TestHarvestTelemetry_ReHarvestIsIdempotent(t *testing.T) {
	workspacePath, copilotPath := setupTelemetryHarvestWorkspace(t)
	writeSampleProcessLog(t, filepath.Join(copilotPath, "logs"))

	sqliteDB, err := db.Open(filepath.Join(workspacePath, "index.db"))
	require.NoError(t, err)
	defer sqliteDB.Close()

	r1, err := telemetry.HarvestTelemetry(context.Background(), workspacePath, copilotPath, sqliteDB, telemetry.HarvestOptions{})
	require.NoError(t, err)

	r2, err := telemetry.HarvestTelemetry(context.Background(), workspacePath, copilotPath, sqliteDB, telemetry.HarvestOptions{})
	require.NoError(t, err)

	assert.Equal(t, r1.SessionsHarvested, r2.SessionsHarvested, "re-harvest should yield same session count")
	assert.Equal(t, r1.TotalTokens, r2.TotalTokens, "re-harvest should yield same token totals")
}

func TestHarvestTelemetry_WritesTokensByServerToJSONL(t *testing.T) {
	workspacePath, copilotPath := setupTelemetryHarvestWorkspace(t)
	writeSampleProcessLog(t, filepath.Join(copilotPath, "logs"))

	sqliteDB, err := db.Open(filepath.Join(workspacePath, "index.db"))
	require.NoError(t, err)
	defer sqliteDB.Close()

	_, err = telemetry.HarvestTelemetry(context.Background(), workspacePath, copilotPath, sqliteDB, telemetry.HarvestOptions{})
	require.NoError(t, err)

	// Read back the JSONL and verify tokens_by_server field is present.
	jsonlPath := filepath.Join(workspacePath, "telemetry-sessions.jsonl")
	content, err := os.ReadFile(jsonlPath)
	require.NoError(t, err)

	// The sample log has sess-h1 with one backlogit tool call and 1500 total tokens.
	// tokens_by_server must contain {"backlogit": 1500}.
	type sessionRecord struct {
		RecordType     string         `json:"record_type"`
		SessionID      string         `json:"session_id"`
		TotalTokens    int            `json:"total_tokens"`
		TokensByServer map[string]int `json:"tokens_by_server"`
	}
	found := false
	for _, line := range strings.Split(strings.TrimSpace(string(content)), "\n") {
		var rec sessionRecord
		if json.Unmarshal([]byte(line), &rec) != nil || rec.RecordType != "session_summary" {
			continue
		}
		if rec.SessionID == "sess-h1" {
			found = true
			assert.Equal(t, 1500, rec.TokensByServer["backlogit"],
				"sess-h1 should have all tokens attributed to backlogit server")
		}
	}
	assert.True(t, found, "session sess-h1 should be present in JSONL output")
}

func TestHarvestTelemetry_MissingCopilotDir(t *testing.T) {
	workspacePath := t.TempDir()
	require.NoError(t, os.MkdirAll(workspacePath, 0o755))

	sqliteDB, err := db.Open(filepath.Join(workspacePath, "index.db"))
	require.NoError(t, err)
	defer sqliteDB.Close()

	_, err = telemetry.HarvestTelemetry(context.Background(), workspacePath, "/nonexistent/.copilot", sqliteDB, telemetry.HarvestOptions{})
	require.Error(t, err)
	assert.ErrorIs(t, err, errors.ErrTelemetrySourceMissing)
}

// ---- 054.002-T: model_class and reasoning_level populated in JSONL ----------

func TestHarvestTelemetry_WritesModelClassToJSONL(t *testing.T) {
	workspacePath, copilotPath := setupTelemetryHarvestWorkspace(t)
	writeSampleProcessLog(t, filepath.Join(copilotPath, "logs"))

	sqliteDB, err := db.Open(filepath.Join(workspacePath, "index.db"))
	require.NoError(t, err)
	defer sqliteDB.Close()

	_, err = telemetry.HarvestTelemetry(context.Background(), workspacePath, copilotPath, sqliteDB, telemetry.HarvestOptions{})
	require.NoError(t, err)

	jsonlPath := filepath.Join(workspacePath, "telemetry-sessions.jsonl")
	content, err := os.ReadFile(jsonlPath)
	require.NoError(t, err)

	type sessionRecord struct {
		RecordType     string `json:"record_type"`
		SessionID      string `json:"session_id"`
		ModelClass     string `json:"model_class"`
		ReasoningLevel string `json:"reasoning_level"`
	}

	classes := make(map[string]string)   // sessionID → model_class
	reasoning := make(map[string]string) // sessionID → reasoning_level
	for _, line := range strings.Split(strings.TrimSpace(string(content)), "\n") {
		var rec sessionRecord
		if json.Unmarshal([]byte(line), &rec) != nil || rec.RecordType != "session_summary" {
			continue
		}
		classes[rec.SessionID] = rec.ModelClass
		reasoning[rec.SessionID] = rec.ReasoningLevel
	}

	// sess-h1 uses "claude-sonnet-4" → class "sonnet", no reasoning level
	assert.Equal(t, "sonnet", classes["sess-h1"],
		"sess-h1 (claude-sonnet-4) should have model_class=sonnet")
	assert.Equal(t, "", reasoning["sess-h1"],
		"sess-h1 (claude-sonnet-4) should have no reasoning_level")

	// sess-h2 uses "gpt-5.1" → class "gpt", no reasoning level
	assert.Equal(t, "gpt", classes["sess-h2"],
		"sess-h2 (gpt-5.1) should have model_class=gpt")
	assert.Equal(t, "", reasoning["sess-h2"],
		"sess-h2 (gpt-5.1) should have no reasoning_level")
}
