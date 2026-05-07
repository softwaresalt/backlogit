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

	"github.com/softwaresalt/backlogit/internal/telemetry"
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
		Format:  telemetry.FormatTable,
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
		Format:  telemetry.FormatJSON,
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
		Format:  telemetry.FormatTable,
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
		Format:    telemetry.FormatTable,
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
		Format:  telemetry.FormatTable,
	})
	require.NoError(t, err, "missing data must not return an error")
	assert.NotEmpty(t, output,
		"GenerateReport must return an informative message when no data exists")
}

// ---- GenerateTrendReport ----------------------------------------------------

// writeTrendTelemetryJSONL writes a fixture for trend report tests with two
// distinct dates and branches to validate both grouping modes.
func writeTrendTelemetryJSONL(t *testing.T, workspacePath string) {
	t.Helper()
	backlogitDir := filepath.Join(workspacePath, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogitDir, 0o755))
	// Two sessions on 2026-04-09, one on 2026-04-10; branches main and feat-x.
	records := []string{
		`{"record_type":"session_summary","harvested_at":"2026-04-09T00:00:00Z","session_id":"s1","branch":"main","repository":"repo","total_tokens":1500,"model_calls":2,"tool_calls":3,"tokens_per_task":750,"compaction_count":0}`,
		`{"record_type":"session_summary","harvested_at":"2026-04-09T12:00:00Z","session_id":"s2","branch":"feat-x","repository":"repo","total_tokens":900,"model_calls":1,"tool_calls":1,"compaction_count":0}`,
		`{"record_type":"session_summary","harvested_at":"2026-04-10T00:00:00Z","session_id":"s3","branch":"main","repository":"repo","total_tokens":600,"model_calls":1,"tool_calls":2,"compaction_count":0}`,
	}
	content := strings.Join(records, "\n") + "\n"
	require.NoError(t, os.WriteFile(
		filepath.Join(backlogitDir, "telemetry-sessions.jsonl"),
		[]byte(content), 0o644,
	))
}

func TestGenerateReport_Limit_RestrictsRowCount(t *testing.T) {
	ws := t.TempDir()
	writeMinimalTelemetryJSONL(t, ws)

	output, err := telemetry.GenerateReport(ws, telemetry.ReportOptions{
		GroupBy: "session",
		Format:  telemetry.FormatTable,
		Limit:   1,
	})
	require.NoError(t, err)
	// With Limit=1, only one session should appear.
	c1 := strings.Count(output, "sess-rpt-1")
	c2 := strings.Count(output, "sess-rpt-2")
	total := c1 + c2
	assert.LessOrEqual(t, total, 1, "Limit=1 must restrict output to at most 1 session row")
}

func TestGenerateTrendReport_ByDate_GroupsCorrectly(t *testing.T) {
	ws := t.TempDir()
	writeTrendTelemetryJSONL(t, ws)

	output, err := telemetry.GenerateTrendReport(ws, telemetry.TrendOptions{
		By:     "date",
		Format: telemetry.FormatTable,
	})
	require.NoError(t, err)
	assert.Contains(t, output, "2026-04-09", "date group should appear in output")
	assert.Contains(t, output, "2026-04-10", "second date group should appear in output")
}

func TestGenerateTrendReport_ByBranch_GroupsCorrectly(t *testing.T) {
	ws := t.TempDir()
	writeTrendTelemetryJSONL(t, ws)

	output, err := telemetry.GenerateTrendReport(ws, telemetry.TrendOptions{
		By:     "branch",
		Format: telemetry.FormatTable,
	})
	require.NoError(t, err)
	assert.Contains(t, output, "main", "branch 'main' should appear in output")
	assert.Contains(t, output, "feat-x", "branch 'feat-x' should appear in output")
}

func TestGenerateTrendReport_JSONFormat_Valid(t *testing.T) {
	ws := t.TempDir()
	writeTrendTelemetryJSONL(t, ws)

	output, err := telemetry.GenerateTrendReport(ws, telemetry.TrendOptions{
		By:     "date",
		Format: telemetry.FormatJSON,
	})
	require.NoError(t, err)

	var decoded []any
	require.NoError(t, json.Unmarshal([]byte(output), &decoded),
		"GenerateTrendReport with format=json must produce a valid JSON array")
	assert.NotEmpty(t, decoded)
}

func TestGenerateTrendReport_MarkdownFormat_Valid(t *testing.T) {
	ws := t.TempDir()
	writeTrendTelemetryJSONL(t, ws)

	output, err := telemetry.GenerateTrendReport(ws, telemetry.TrendOptions{
		By:     "date",
		Format: telemetry.FormatMarkdown,
	})
	require.NoError(t, err)
	assert.Contains(t, output, "|", "Markdown output must contain table delimiter")
}

func TestGenerateTrendReport_NoData_ReturnsInformativeMessage(t *testing.T) {
	ws := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(ws, ".backlogit"), 0o755))

	output, err := telemetry.GenerateTrendReport(ws, telemetry.TrendOptions{
		By:     "date",
		Format: telemetry.FormatTable,
	})
	require.NoError(t, err, "missing data must not return an error")
	assert.NotEmpty(t, output)
}

func TestGenerateTrendReport_Limit_RestrictsGroups(t *testing.T) {
	ws := t.TempDir()
	writeTrendTelemetryJSONL(t, ws)

	output, err := telemetry.GenerateTrendReport(ws, telemetry.TrendOptions{
		By:     "date",
		Format: telemetry.FormatTable,
		Limit:  1,
	})
	require.NoError(t, err)
	// With Limit=1, only one date group should appear.
	c9 := strings.Count(output, "2026-04-09")
	c10 := strings.Count(output, "2026-04-10")
	assert.LessOrEqual(t, c9+c10, 1, "Limit=1 should restrict output to at most 1 group")
}
