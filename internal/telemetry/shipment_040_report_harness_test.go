package telemetry_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

	backlogitDir := filepath.Join(workspacePath, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogitDir, 0o755))

	records := []string{
		`{"record_type":"session_summary","harvested_at":"2026-04-09T00:00:00Z","session_id":"sess-rich-1","branch":"main","repository":"backlogit","total_tokens":2200,"prompt_tokens":1400,"completion_tokens":800,"cached_tokens":100,"model_calls":3,"tool_calls":4,"tokens_by_model":{"claude-sonnet-4":2200},"tool_calls_by_server":{"backlogit":3,"copilot":1},"completed_tasks":[],"tokens_per_task":null,"compaction_count":0}`,
		`{"record_type":"session_summary","harvested_at":"2026-04-09T00:00:00Z","session_id":"sess-rich-2","branch":"feature-y","repository":"backlogit","total_tokens":750,"prompt_tokens":500,"completion_tokens":250,"cached_tokens":0,"model_calls":1,"tool_calls":2,"tokens_by_model":{"gpt-5.4":750},"tool_calls_by_server":{"copilot":2},"completed_tasks":[],"tokens_per_task":null,"compaction_count":1}`,
		`{"record_type":"tool_usage","harvested_at":"2026-04-09T00:00:00Z","session_id":"sess-rich-1","server_name":"backlogit","tool_name":"backlogit_get_item","call_count":2,"total_duration_ms":90}`,
		`{"record_type":"tool_usage","harvested_at":"2026-04-09T00:00:00Z","session_id":"sess-rich-2","server_name":"copilot","tool_name":"view","call_count":2,"total_duration_ms":35}`,
	}

	require.NoError(t, os.WriteFile(
		filepath.Join(backlogitDir, "telemetry-sessions.jsonl"),
		[]byte(strings.Join(records, "\n")+"\n"),
		0o644,
	))
}

func TestTask039013_ReportHarnessScenariosPending(t *testing.T) {
	ws := t.TempDir()
	writeRichTelemetryJSONL(t, ws)

	t.Run("multi_session_fixture", func(t *testing.T) {
		output, err := telemetry.GenerateReport(ws, telemetry.ReportOptions{
			GroupBy: "session",
			Format:  "table",
		})
		require.NoError(t, err)
		assert.Contains(t, output, "sess-rich-1")
		assert.Contains(t, output, "sess-rich-2")
	})

	t.Run("unsupported_format_validation", func(t *testing.T) {
		output, err := telemetry.GenerateReport(ws, telemetry.ReportOptions{
			GroupBy: "session",
			Format:  "xml",
		})
		require.Error(t, err)
		assert.Empty(t, output)
		assert.Contains(t, err.Error(), "unsupported format value")
	})

	t.Run("unsupported_groupby_validation", func(t *testing.T) {
		output, err := telemetry.GenerateReport(ws, telemetry.ReportOptions{
			GroupBy: "workspace",
			Format:  "table",
		})
		require.Error(t, err)
		assert.Empty(t, output)
		assert.Contains(t, err.Error(), "unsupported group-by value")
	})
}

func TestTask039014_HarvestPipelineHarnessPending(t *testing.T) {
	requireShipment040TelemetryHarness(t, "039.014-T")

	t.Run("full_harvest_pipeline", func(t *testing.T) {
		t.Fatal("not implemented: add end-to-end harvest pipeline coverage for 039.014-T")
	})

	t.Run("incremental_checkpoint", func(t *testing.T) {
		t.Fatal("not implemented: add incremental harvest checkpoint coverage for 039.014-T")
	})
}

func TestTask039015_GenerateReportMarkdownFormat(t *testing.T) {
	requireShipment040TelemetryHarness(t, "039.015-T")

	ws := t.TempDir()
	writeMinimalTelemetryJSONL(t, ws)

	output, err := telemetry.GenerateReport(ws, telemetry.ReportOptions{
		GroupBy: "session",
		Format:  "markdown",
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
		Format:  "markdown",
	})
	require.NoError(t, err)
	assert.Contains(t, output, "## Tool Calls by Server")
	assert.Contains(t, output, "| Server |")
	assert.Contains(t, output, "backlogit")
}
