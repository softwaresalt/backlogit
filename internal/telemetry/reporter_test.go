package telemetry_test

// Harness for 031.004-T: CLI Telemetry Report Subcommand.
//
// All tests will FAIL until reporter.go is implemented.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/telemetry"
)

// writeMinimalTelemetryJSONL writes a minimal telemetry-sessions.jsonl for
// report tests that do not need a full harvest run.
func writeMinimalTelemetryJSONL(t *testing.T, workspacePath string) {
	t.Helper()
	backlogitDir := filepath.Join(workspacePath, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogitDir, 0o755))
	records := []string{
		`{"record_type":"session_summary","harvested_at":"2026-04-09T00:00:00Z","session_id":"sess-rpt-1","branch":"main","repository":"backlogit","total_tokens":1500,"prompt_tokens":1000,"completion_tokens":500,"cached_tokens":200,"model_calls":2,"tool_calls":3,"tokens_by_model":{"claude-sonnet-4":1500},"tool_calls_by_server":{"backlogit":2,"copilot":1},"completed_tasks":[],"tokens_per_task":null,"compaction_count":0}`,
		`{"record_type":"tool_usage","harvested_at":"2026-04-09T00:00:00Z","session_id":"sess-rpt-1","server_name":"backlogit","tool_name":"backlogit_get_item","call_count":2,"total_duration_ms":90}`,
		`{"record_type":"session_summary","harvested_at":"2026-04-09T00:00:00Z","session_id":"sess-rpt-2","branch":"feature-x","repository":"backlogit","total_tokens":400,"prompt_tokens":300,"completion_tokens":100,"cached_tokens":0,"model_calls":1,"tool_calls":1,"tokens_by_model":{"gpt-5.1":400},"tool_calls_by_server":{"copilot":1},"completed_tasks":[],"tokens_per_task":null,"compaction_count":0}`,
	}
	content := strings.Join(records, "\n") + "\n"
	require.NoError(t, os.WriteFile(
		filepath.Join(backlogitDir, "telemetry-sessions.jsonl"),
		[]byte(content), 0o644,
	))
}

// ---- GenerateReport: table format -------------------------------------------

func TestGenerateReport_TableFormat_ProducesAlignedOutput(t *testing.T) {
	ws := t.TempDir()
	writeMinimalTelemetryJSONL(t, ws)

	output, err := telemetry.GenerateReport(ws, telemetry.ReportOptions{
		GroupBy: "session",
		Format:  "table",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, output)
	// Table format must contain session IDs.
	assert.Contains(t, output, "sess-rpt-1")
	assert.Contains(t, output, "sess-rpt-2")
}

func TestGenerateReport_JSONFormat_ProducesValidJSON(t *testing.T) {
	ws := t.TempDir()
	writeMinimalTelemetryJSONL(t, ws)

	output, err := telemetry.GenerateReport(ws, telemetry.ReportOptions{
		GroupBy: "session",
		Format:  "json",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, output)

	var decoded any
	require.NoError(t, json.Unmarshal([]byte(output), &decoded),
		"GenerateReport with format=json must return valid JSON")
}

func TestGenerateReport_ByServer_GroupsByServer(t *testing.T) {
	ws := t.TempDir()
	writeMinimalTelemetryJSONL(t, ws)

	output, err := telemetry.GenerateReport(ws, telemetry.ReportOptions{
		GroupBy: "server",
		Format:  "table",
	})
	require.NoError(t, err)
	assert.Contains(t, output, "backlogit", "--by server must include server names")
}

func TestGenerateReport_SessionFilter_LimitsToSingleSession(t *testing.T) {
	ws := t.TempDir()
	writeMinimalTelemetryJSONL(t, ws)

	output, err := telemetry.GenerateReport(ws, telemetry.ReportOptions{
		SessionID: "sess-rpt-1",
		GroupBy:   "session",
		Format:    "table",
	})
	require.NoError(t, err)
	assert.Contains(t, output, "sess-rpt-1")
	assert.NotContains(t, output, "sess-rpt-2",
		"--session filter must exclude other sessions")
}

func TestGenerateReport_NoData_ReturnsInformativeMessage(t *testing.T) {
	ws := t.TempDir()
	// No telemetry-sessions.jsonl in this workspace.
	require.NoError(t, os.MkdirAll(filepath.Join(ws, ".backlogit"), 0o755))

	output, err := telemetry.GenerateReport(ws, telemetry.ReportOptions{
		GroupBy: "session",
		Format:  "table",
	})
	require.NoError(t, err, "missing data must not return an error")
	assert.NotEmpty(t, output,
		"GenerateReport must return an informative message when no data exists")
}

func TestGenerateReport_Limit_RestrictsRowCount(t *testing.T) {
	ws := t.TempDir()
	writeMinimalTelemetryJSONL(t, ws)

	output, err := telemetry.GenerateReport(ws, telemetry.ReportOptions{
		GroupBy: "session",
		Format:  "table",
		Limit:   1,
	})
	require.NoError(t, err)
	// With Limit=1, only one session should appear.
	c1 := strings.Count(output, "sess-rpt-1")
	c2 := strings.Count(output, "sess-rpt-2")
	total := c1 + c2
	assert.LessOrEqual(t, total, 1, "Limit=1 must restrict output to at most 1 session row")
}
