package telemetry_test

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/telemetry"
)

func requireShipment040TelemetryHarness(t *testing.T, taskID string) {
	t.Helper()

	if active := os.Getenv("BACKLOGIT_ACTIVE_HARNESS"); active != taskID {
		t.Skipf("shipment 040 harness inactive for %s (BACKLOGIT_ACTIVE_HARNESS=%q)", taskID, active)
	}
}

func writeRichTelemetryJSONL(t *testing.T, workspacePath string) {
	t.Helper()

	require.NoError(t, os.MkdirAll(workspacePath, 0o755))

	records := []string{
		`{"record_type":"session_summary","harvested_at":"2026-04-09T00:00:00Z","session_id":"sess-rich-1","branch":"main","repository":"backlogit","total_tokens":2200,"prompt_tokens":1400,"completion_tokens":800,"cached_tokens":100,"model_calls":3,"tool_calls":4,"tokens_by_model":{"claude-sonnet-4":2200},"tool_calls_by_server":{"backlogit":3,"copilot":1},"completed_tasks":[],"tokens_per_task":null,"compaction_count":0}`,
		`{"record_type":"session_summary","harvested_at":"2026-04-09T00:00:00Z","session_id":"sess-rich-2","branch":"feature-y","repository":"backlogit","total_tokens":750,"prompt_tokens":500,"completion_tokens":250,"cached_tokens":0,"model_calls":1,"tool_calls":2,"tokens_by_model":{"gpt-5.4":750},"tool_calls_by_server":{"copilot":2},"completed_tasks":[],"tokens_per_task":null,"compaction_count":1}`,
		`{"record_type":"tool_usage","harvested_at":"2026-04-09T00:00:00Z","session_id":"sess-rich-1","server_name":"backlogit","tool_name":"backlogit_get_item","call_count":2,"total_duration_ms":90}`,
		`{"record_type":"tool_usage","harvested_at":"2026-04-09T00:00:00Z","session_id":"sess-rich-2","server_name":"copilot","tool_name":"view","call_count":2,"total_duration_ms":35}`,
	}

	require.NoError(t, os.WriteFile(
		filepath.Join(workspacePath, "telemetry-sessions.jsonl"),
		[]byte(strings.Join(records, "\n")+"\n"),
		0o644,
	))
}

func writeProcessLogLines(t *testing.T, logsDir, fileName string, lines []string) {
	t.Helper()

	require.NoError(t, os.WriteFile(
		filepath.Join(logsDir, fileName),
		[]byte(strings.Join(lines, "\n")+"\n"),
		0o644,
	))
}

func appendProcessLogLines(t *testing.T, logsDir, fileName string, lines []string) {
	t.Helper()

	f, err := os.OpenFile(filepath.Join(logsDir, fileName), os.O_APPEND|os.O_WRONLY, 0o644)
	require.NoError(t, err)
	defer f.Close()

	_, err = f.WriteString(strings.Join(lines, "\n") + "\n")
	require.NoError(t, err)
}

func writeSessionStoreFixture(t *testing.T, copilotPath, sessionID, repository, branch, workDir string) {
	t.Helper()

	storeDB, err := db.Open(filepath.Join(copilotPath, "session-store.db"))
	require.NoError(t, err)
	defer storeDB.Close()

	_, err = storeDB.Exec(`
		CREATE TABLE sessions (
			id TEXT PRIMARY KEY,
			repository TEXT,
			branch TEXT,
			cwd TEXT,
			created_at TEXT
		)
	`)
	require.NoError(t, err)

	_, err = storeDB.Exec(
		`INSERT INTO sessions (id, repository, branch, cwd, created_at) VALUES (?, ?, ?, ?, ?)`,
		sessionID,
		repository,
		branch,
		workDir,
		"2026-04-21T00:00:00Z",
	)
	require.NoError(t, err)
}

func writeSessionEventsFixture(t *testing.T, copilotPath, sessionID string) {
	t.Helper()

	sessionDir := filepath.Join(copilotPath, "session-state", sessionID)
	require.NoError(t, os.MkdirAll(sessionDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(sessionDir, "events.jsonl"),
		[]byte(`{"event_type":"session.compaction_complete","timestamp":"2026-04-21T00:00:03Z","preCompactionTokens":1800,"compactionTokensUsed":{"input":900,"output":150,"cachedInput":75}}`+"\n"),
		0o644,
	))
}

func writeCompletedTaskLog(t *testing.T, workspacePath, itemID string) {
	t.Helper()

	logsDir := filepath.Join(workspacePath, "logs")
	require.NoError(t, os.MkdirAll(logsDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(logsDir, itemID+".jsonl"),
		[]byte(`{"event_type":"status_changed","item_id":"`+itemID+`","delta":{"from":"active","to":"done"}}`+"\n"),
		0o644,
	))
}

func readTelemetryJSONL(t *testing.T, workspacePath string) ([]telemetry.SessionSummaryRecord, []telemetry.ToolUsageRecord) {
	t.Helper()

	f, err := os.Open(filepath.Join(workspacePath, "telemetry-sessions.jsonl"))
	require.NoError(t, err)
	defer f.Close()

	var sessions []telemetry.SessionSummaryRecord
	var tools []telemetry.ToolUsageRecord
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		var header struct {
			RecordType string `json:"record_type"`
		}
		require.NoError(t, json.Unmarshal(line, &header))

		switch header.RecordType {
		case "session_summary":
			var record telemetry.SessionSummaryRecord
			require.NoError(t, json.Unmarshal(line, &record))
			sessions = append(sessions, record)
		case "tool_usage":
			var record telemetry.ToolUsageRecord
			require.NoError(t, json.Unmarshal(line, &record))
			tools = append(tools, record)
		}
	}
	require.NoError(t, scanner.Err())

	return sessions, tools
}

func openTelemetryIndexDB(t *testing.T, workspacePath string) *sql.DB {
	t.Helper()

	sqliteDB, err := db.Open(filepath.Join(workspacePath, "index.db"))
	require.NoError(t, err)
	t.Cleanup(func() { sqliteDB.Close() })
	return sqliteDB
}

func TestTask039013_ReportHarnessScenarios(t *testing.T) {
	ws := t.TempDir()
	writeRichTelemetryJSONL(t, ws)

	t.Run("multi_session_fixture", func(t *testing.T) {
		output, err := telemetry.GenerateReport(ws, telemetry.ReportOptions{
			GroupBy: "session",
			Format:  telemetry.FormatTable,
		})
		require.NoError(t, err)
		assert.Contains(t, output, "sess-rich-1")
		assert.Contains(t, output, "sess-rich-2")
	})

	t.Run("unsupported_format_validation", func(t *testing.T) {
		output, err := telemetry.GenerateReport(ws, telemetry.ReportOptions{
			GroupBy: "session",
			Format:  telemetry.ReportFormat("xml"),
		})
		require.Error(t, err)
		assert.Empty(t, output)
		assert.Contains(t, err.Error(), "unsupported format value")
	})

	t.Run("unsupported_groupby_validation", func(t *testing.T) {
		output, err := telemetry.GenerateReport(ws, telemetry.ReportOptions{
			GroupBy: "workspace",
			Format:  telemetry.FormatTable,
		})
		require.Error(t, err)
		assert.Empty(t, output)
		assert.Contains(t, err.Error(), "unsupported group-by value")
	})
}

func TestTask039014_HarvestPipelineHarness(t *testing.T) {
	t.Run("full_harvest_pipeline", func(t *testing.T) {
		workspacePath, copilotPath := setupTelemetryHarvestWorkspace(t)
		logsDir := filepath.Join(copilotPath, "logs")

		writeProcessLogLines(t, logsDir, "process-001.log", []string{
			`2026-04-21T00:00:01.000Z [info] startup`,
			`2026-04-21T00:00:02.000Z [telemetry] {"event":"cli.model_call","session_id":"sess-pipeline","request_id":"req-pipeline","model":"claude-sonnet-4","prompt_tokens_count":1500,"completion_tokens_count":900,"total_tokens_count":2400,"cached_tokens_count":300,"duration_ms":1200}`,
			`2026-04-21T00:00:03.000Z [telemetry] {"event":"cli.tool_call","session_id":"sess-pipeline","model_call_id":"req-pipeline","tool_name":"backlogit_get_item","result_type":"text","duration_ms":45}`,
			`2026-04-21T00:00:04.000Z [telemetry] {"event":"cli.tool_call","session_id":"sess-pipeline","model_call_id":"req-pipeline","tool_name":"view","result_type":"text","duration_ms":30}`,
		})
		writeSessionStoreFixture(
			t,
			copilotPath,
			"sess-pipeline",
			"softwaresalt/backlogit",
			"ship/040-s-binary-release-telemetry-markdown",
			workspacePath,
		)
		writeSessionEventsFixture(t, copilotPath, "sess-pipeline")
		writeCompletedTaskLog(t, workspacePath, "039.014-T")

		sqliteDB := openTelemetryIndexDB(t, workspacePath)
		result, err := telemetry.HarvestTelemetry(
			context.Background(),
			workspacePath,
			copilotPath,
			sqliteDB,
			telemetry.HarvestOptions{},
		)
		require.NoError(t, err)
		assert.Equal(t, 1, result.SessionsHarvested)
		assert.Equal(t, 2, result.ToolCallsIndexed)
		assert.Equal(t, 2400, result.TotalTokens)

		sessions, tools := readTelemetryJSONL(t, workspacePath)
		require.Len(t, sessions, 1)
		require.Len(t, tools, 2)

		session := sessions[0]
		assert.Equal(t, "sess-pipeline", session.SessionID)
		assert.Equal(t, "ship/040-s-binary-release-telemetry-markdown", session.Branch)
		assert.Equal(t, "softwaresalt/backlogit", session.Repository)
		assert.Equal(t, 2400, session.TotalTokens)
		assert.Equal(t, 1500, session.PromptTokens)
		assert.Equal(t, 900, session.CompletionTokens)
		assert.Equal(t, 300, session.CachedTokens)
		assert.Equal(t, 1, session.ModelCalls)
		assert.Equal(t, 2, session.ToolCalls)
		assert.Equal(t, map[string]int{"claude-sonnet-4": 2400}, session.TokensByModel)
		assert.Equal(t, map[string]int{"backlogit": 1, "copilot_builtin": 1}, session.ToolCallsByServer)
		assert.Equal(t, []string{"039.014-T"}, session.CompletedTasks)
		assert.Equal(t, 1, session.CompactionCount)
		if assert.NotNil(t, session.TokensPerTask) {
			assert.InDelta(t, 2400, *session.TokensPerTask, 0.001)
		}
		if assert.NotNil(t, session.PeakUtilization) {
			assert.InDelta(t, 1500.0/200000.0, *session.PeakUtilization, 1e-9)
		}
		if assert.NotNil(t, session.RemainingCapacity) {
			assert.Equal(t, 198500, *session.RemainingCapacity)
		}
		if assert.NotNil(t, session.DepletionRate) {
			assert.InDelta(t, 2400, *session.DepletionRate, 0.001)
		}
		if assert.NotNil(t, session.MaxContextTokens) {
			assert.Equal(t, 200000, *session.MaxContextTokens)
		}

		var branch, repository string
		var totalTokens, modelCalls, toolCalls, compactionCount int
		err = sqliteDB.QueryRow(
			`SELECT branch, repository, total_tokens, model_calls, tool_calls, compaction_count
			 FROM telemetry_sessions
			 WHERE session_id = ?`,
			"sess-pipeline",
		).Scan(&branch, &repository, &totalTokens, &modelCalls, &toolCalls, &compactionCount)
		require.NoError(t, err)
		assert.Equal(t, "ship/040-s-binary-release-telemetry-markdown", branch)
		assert.Equal(t, "softwaresalt/backlogit", repository)
		assert.Equal(t, 2400, totalTokens)
		assert.Equal(t, 1, modelCalls)
		assert.Equal(t, 2, toolCalls)
		assert.Equal(t, 1, compactionCount)

		var toolUsageCount int
		err = sqliteDB.QueryRow(
			`SELECT COUNT(*) FROM telemetry_tool_usage WHERE session_id = ?`,
			"sess-pipeline",
		).Scan(&toolUsageCount)
		require.NoError(t, err)
		assert.Equal(t, 2, toolUsageCount)
	})

	t.Run("incremental_checkpoint", func(t *testing.T) {
		workspacePath, copilotPath := setupTelemetryHarvestWorkspace(t)
		logsDir := filepath.Join(copilotPath, "logs")

		writeProcessLogLines(t, logsDir, "process-001.log", []string{
			`2026-04-21T01:00:01.000Z [telemetry] {"event":"cli.model_call","session_id":"sess-incremental-1","request_id":"req-inc-1","model":"gpt-5.1","prompt_tokens_count":500,"completion_tokens_count":200,"total_tokens_count":700,"cached_tokens_count":0,"duration_ms":250}`,
			`2026-04-21T01:00:02.000Z [telemetry] {"event":"cli.tool_call","session_id":"sess-incremental-1","model_call_id":"req-inc-1","tool_name":"backlogit_list_items","result_type":"text","duration_ms":25}`,
		})
		writeCompletedTaskLog(t, workspacePath, "039.014-T")

		sqliteDB := openTelemetryIndexDB(t, workspacePath)
		firstResult, err := telemetry.HarvestTelemetry(
			context.Background(),
			workspacePath,
			copilotPath,
			sqliteDB,
			telemetry.HarvestOptions{},
		)
		require.NoError(t, err)
		assert.Equal(t, 1, firstResult.SessionsHarvested)

		firstCheckpoint, err := telemetry.LoadCheckpoint(workspacePath)
		require.NoError(t, err)
		firstOffset := firstCheckpoint.FileOffsets["process-001.log"]
		assert.Greater(t, firstOffset, int64(0))
		assert.False(t, firstCheckpoint.LastHarvest.IsZero())

		appendProcessLogLines(t, logsDir, "process-001.log", []string{
			`2026-04-21T01:05:01.000Z [telemetry] {"event":"cli.model_call","session_id":"sess-incremental-2","request_id":"req-inc-2","model":"claude-sonnet-4","prompt_tokens_count":800,"completion_tokens_count":400,"total_tokens_count":1200,"cached_tokens_count":50,"duration_ms":500}`,
			`2026-04-21T01:05:02.000Z [telemetry] {"event":"cli.tool_call","session_id":"sess-incremental-2","model_call_id":"req-inc-2","tool_name":"view","result_type":"text","duration_ms":20}`,
		})

		secondResult, err := telemetry.HarvestTelemetry(
			context.Background(),
			workspacePath,
			copilotPath,
			sqliteDB,
			telemetry.HarvestOptions{},
		)
		require.NoError(t, err)
		assert.Equal(t, 2, secondResult.SessionsHarvested)
		assert.Equal(t, 2, secondResult.ToolCallsIndexed)
		assert.Equal(t, 1900, secondResult.TotalTokens)

		secondCheckpoint, err := telemetry.LoadCheckpoint(workspacePath)
		require.NoError(t, err)
		assert.Greater(t, secondCheckpoint.FileOffsets["process-001.log"], firstOffset)
		assert.False(t, secondCheckpoint.LastHarvest.IsZero())

		sessions, tools := readTelemetryJSONL(t, workspacePath)
		require.Len(t, sessions, 2)
		require.Len(t, tools, 2)
		assert.ElementsMatch(
			t,
			[]string{"sess-incremental-1", "sess-incremental-2"},
			[]string{sessions[0].SessionID, sessions[1].SessionID},
		)

		var sessionCount, toolCount int
		require.NoError(t, sqliteDB.QueryRow(`SELECT COUNT(*) FROM telemetry_sessions`).Scan(&sessionCount))
		require.NoError(t, sqliteDB.QueryRow(`SELECT COUNT(*) FROM telemetry_tool_usage`).Scan(&toolCount))
		assert.Equal(t, 2, sessionCount)
		assert.Equal(t, 2, toolCount)
	})
}

func TestTask039015_GenerateReportMarkdownFormat(t *testing.T) {
	requireShipment040TelemetryHarness(t, "039.015-T")

	ws := t.TempDir()
	writeMinimalTelemetryJSONL(t, ws)

	output, err := telemetry.GenerateReport(ws, telemetry.ReportOptions{
		GroupBy: "session",
		Format:  telemetry.FormatMarkdown,
	})
	require.NoError(t, err)
	assert.Contains(t, output, "# Telemetry Report")
	assert.Contains(t, output, "## Session Summary")
	assert.Contains(t, output, "| Session |")
	assert.Contains(t, output, "sess-rpt-1")
}

func TestTask039015_GenerateReportMarkdownServerGrouping(t *testing.T) {
	requireShipment040TelemetryHarness(t, "039.015-T")

	ws := t.TempDir()
	writeMinimalTelemetryJSONL(t, ws)

	output, err := telemetry.GenerateReport(ws, telemetry.ReportOptions{
		GroupBy: "server",
		Format:  telemetry.FormatMarkdown,
	})
	require.NoError(t, err)
	assert.Contains(t, output, "## Tool Calls by Server")
	assert.Contains(t, output, "| Server |")
	assert.Contains(t, output, "backlogit")
}
