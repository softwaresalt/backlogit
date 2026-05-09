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

// ---- Unit 1: IsGhostSession + ghost filtering in GenerateTrendReport --------

// writeGhostTrendJSONL creates a JSONL fixture with 3 active sessions
// (1000 tokens, 2 model calls, 4 tool calls each) and 2 ghost sessions
// (all zeros), all harvested on 2026-05-08 on branch "main".
func writeGhostTrendJSONL(t *testing.T, workspacePath string) {
	t.Helper()
	backlogitDir := filepath.Join(workspacePath, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogitDir, 0o755))
	records := []string{
		`{"record_type":"session_summary","harvested_at":"2026-05-08T00:00:00Z","session_id":"ghost-active-1","branch":"main","repository":"repo","total_tokens":1000,"model_calls":2,"tool_calls":4,"compaction_count":0}`,
		`{"record_type":"session_summary","harvested_at":"2026-05-08T00:00:00Z","session_id":"ghost-active-2","branch":"main","repository":"repo","total_tokens":1000,"model_calls":2,"tool_calls":4,"compaction_count":0}`,
		`{"record_type":"session_summary","harvested_at":"2026-05-08T00:00:00Z","session_id":"ghost-active-3","branch":"main","repository":"repo","total_tokens":1000,"model_calls":2,"tool_calls":4,"compaction_count":0}`,
		`{"record_type":"session_summary","harvested_at":"2026-05-08T00:00:00Z","session_id":"ghost-sess-1","branch":"main","repository":"repo","total_tokens":0,"model_calls":0,"tool_calls":0,"compaction_count":0}`,
		`{"record_type":"session_summary","harvested_at":"2026-05-08T00:00:00Z","session_id":"ghost-sess-2","branch":"main","repository":"repo","total_tokens":0,"model_calls":0,"tool_calls":0,"compaction_count":0}`,
	}
	content := strings.Join(records, "\n") + "\n"
	require.NoError(t, os.WriteFile(
		filepath.Join(backlogitDir, "telemetry-sessions.jsonl"),
		[]byte(content), 0o644,
	))
}

func TestIsGhostSession_AllZero_ReturnsTrue(t *testing.T) {
	s := telemetry.SessionSummaryRecord{
		TotalTokens: 0,
		ModelCalls:  0,
		ToolCalls:   0,
	}
	assert.True(t, telemetry.IsGhostSession(s))
}

func TestIsGhostSession_NonzeroTokens_ReturnsFalse(t *testing.T) {
	s := telemetry.SessionSummaryRecord{
		TotalTokens: 1000,
		ModelCalls:  0,
		ToolCalls:   0,
	}
	assert.False(t, telemetry.IsGhostSession(s))
}

func TestIsGhostSession_ZeroTokensNonzeroModelCalls_ReturnsFalse(t *testing.T) {
	s := telemetry.SessionSummaryRecord{
		TotalTokens: 0,
		ModelCalls:  2,
		ToolCalls:   0,
	}
	assert.False(t, telemetry.IsGhostSession(s))
}

func TestGenerateTrendReport_GhostSessionsExcludedFromAverages(t *testing.T) {
	ws := t.TempDir()
	writeGhostTrendJSONL(t, ws)

	output, err := telemetry.GenerateTrendReport(ws, telemetry.TrendOptions{
		By:     "date",
		Format: telemetry.FormatJSON,
	})
	require.NoError(t, err)

	var groups []telemetry.TrendGroup
	require.NoError(t, json.Unmarshal([]byte(output), &groups))
	require.Len(t, groups, 1, "expected one date group")

	g := groups[0]
	assert.Equal(t, 3, g.Sessions, "ghost sessions must not increment Sessions")
	assert.InDelta(t, 1000.0, g.AvgTokensSession, 0.01,
		"avg_tokens_per_session must exclude ghost sessions from denominator")
}

// ---- Unit 2: [empty] marker in session list output --------------------------

func TestFormatSessionTable_GhostSessions_ShowEmptyMarker(t *testing.T) {
	ws := t.TempDir()
	writeGhostTrendJSONL(t, ws)

	output, err := telemetry.GenerateReport(ws, telemetry.ReportOptions{
		Format: telemetry.FormatTable,
	})
	require.NoError(t, err)
	assert.Contains(t, output, "[empty]",
		"ghost sessions must display [empty] marker in table output")
}

func TestFormatSessionMarkdown_GhostSessions_ShowEmptyMarker(t *testing.T) {
	ws := t.TempDir()
	writeGhostTrendJSONL(t, ws)

	output, err := telemetry.GenerateReport(ws, telemetry.ReportOptions{
		Format: telemetry.FormatMarkdown,
	})
	require.NoError(t, err)
	assert.Contains(t, output, "[empty]",
		"ghost sessions must display [empty] marker in markdown output")
}

func TestFormatSessionTable_ActiveSession_NoEmptyMarker(t *testing.T) {
	ws := t.TempDir()
	writeGhostTrendJSONL(t, ws)

	output, err := telemetry.GenerateReport(ws, telemetry.ReportOptions{
		Format: telemetry.FormatTable,
	})
	require.NoError(t, err)
	// Active sessions must appear without an [empty] marker.
	assert.Contains(t, output, "ghost-active-1")
	// Exactly 2 ghost sessions are in the fixture, so [empty] appears exactly 2 times.
	assert.Equal(t, 2, strings.Count(output, "[empty]"),
		"only ghost sessions must show the [empty] marker")
}

// ---- Unit 3: AvgModelCalls / AvgToolCalls in TrendGroup ---------------------

func TestGenerateTrendReport_CallRateColumns_InJSON(t *testing.T) {
	ws := t.TempDir()
	writeGhostTrendJSONL(t, ws)

	output, err := telemetry.GenerateTrendReport(ws, telemetry.TrendOptions{
		By:     "date",
		Format: telemetry.FormatJSON,
	})
	require.NoError(t, err)

	var groups []telemetry.TrendGroup
	require.NoError(t, json.Unmarshal([]byte(output), &groups))
	require.Len(t, groups, 1)

	g := groups[0]
	assert.InDelta(t, 2.0, g.AvgModelCalls, 0.01, "avg_model_calls should be 2.0")
	assert.InDelta(t, 4.0, g.AvgToolCalls, 0.01, "avg_tool_calls should be 4.0")
}

func TestGenerateTrendReport_CallRateColumns_InTable(t *testing.T) {
	ws := t.TempDir()
	writeGhostTrendJSONL(t, ws)

	output, err := telemetry.GenerateTrendReport(ws, telemetry.TrendOptions{
		By:     "date",
		Format: telemetry.FormatTable,
	})
	require.NoError(t, err)
	assert.Contains(t, output, "AVG_MODEL_CALLS",
		"table header must include AVG_MODEL_CALLS column")
	assert.Contains(t, output, "AVG_TOOL_CALLS",
		"table header must include AVG_TOOL_CALLS column")
}

func TestGenerateTrendReport_CallRateColumns_ExcludeGhosts(t *testing.T) {
	ws := t.TempDir()
	writeGhostTrendJSONL(t, ws)

	output, err := telemetry.GenerateTrendReport(ws, telemetry.TrendOptions{
		By:     "date",
		Format: telemetry.FormatJSON,
	})
	require.NoError(t, err)

	var groups []telemetry.TrendGroup
	require.NoError(t, json.Unmarshal([]byte(output), &groups))
	require.Len(t, groups, 1)

	g := groups[0]
	// Ghost sessions contribute 0 model and 0 tool calls; if included in
	// the denominator, AvgModelCalls would drop from 2.0 to 1.2 (3*2/5).
	assert.InDelta(t, 2.0, g.AvgModelCalls, 0.01,
		"ghost sessions must not inflate AvgModelCalls denominator")
	assert.InDelta(t, 4.0, g.AvgToolCalls, 0.01,
		"ghost sessions must not inflate AvgToolCalls denominator")
}

func TestGenerateTrendReport_ExistingFields_Unchanged(t *testing.T) {
	ws := t.TempDir()
	writeGhostTrendJSONL(t, ws)

	output, err := telemetry.GenerateTrendReport(ws, telemetry.TrendOptions{
		By:     "date",
		Format: telemetry.FormatJSON,
	})
	require.NoError(t, err)

	var groups []telemetry.TrendGroup
	require.NoError(t, json.Unmarshal([]byte(output), &groups))
	require.Len(t, groups, 1)

	g := groups[0]
	assert.Equal(t, 3, g.Sessions,
		"Sessions must only count non-ghost sessions")
	assert.Equal(t, 3000, g.TotalTokens,
		"TotalTokens must be the sum of active sessions only")
	assert.InDelta(t, 1000.0, g.AvgTokensSession, 0.01,
		"AvgTokensSession must use the active-session count as denominator")
}

// ---- Unit 4: PRIMARY_MODEL column in session list output --------------------

// writeModelAwareJSONL creates a fixture with sessions that have tokens_by_model
// populated so that PrimaryModel resolution can be verified.
func writeModelAwareJSONL(t *testing.T, workspacePath string) {
	t.Helper()
	backlogitDir := filepath.Join(workspacePath, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogitDir, 0o755))
	records := []string{
		`{"record_type":"session_summary","harvested_at":"2026-05-08T00:00:00Z","session_id":"ma-sess-1","branch":"main","repository":"repo","total_tokens":1500,"model_calls":2,"tool_calls":3,"tokens_by_model":{"claude-sonnet-4.6":1500},"compaction_count":0,"model_class":"sonnet"}`,
		`{"record_type":"session_summary","harvested_at":"2026-05-08T00:00:00Z","session_id":"ma-sess-2","branch":"main","repository":"repo","total_tokens":400,"model_calls":1,"tool_calls":1,"tokens_by_model":{"gpt-5.4":400},"compaction_count":0,"model_class":"gpt"}`,
	}
	content := strings.Join(records, "\n") + "\n"
	require.NoError(t, os.WriteFile(
		filepath.Join(backlogitDir, "telemetry-sessions.jsonl"),
		[]byte(content), 0o644,
	))
}

func TestFormatSessionTable_PrimaryModel_Column(t *testing.T) {
	ws := t.TempDir()
	writeModelAwareJSONL(t, ws)

	output, err := telemetry.GenerateReport(ws, telemetry.ReportOptions{
		Format: telemetry.FormatTable,
	})
	require.NoError(t, err)
	assert.Contains(t, output, "PRIMARY_MODEL",
		"table header must include PRIMARY_MODEL column")
	assert.Contains(t, output, "claude-sonnet-4.6",
		"PRIMARY_MODEL column must show the primary model name")
	assert.Contains(t, output, "gpt-5.4",
		"PRIMARY_MODEL column must show the primary model name for second session")
}

func TestFormatSessionMarkdown_PrimaryModel_Column(t *testing.T) {
	ws := t.TempDir()
	writeModelAwareJSONL(t, ws)

	output, err := telemetry.GenerateReport(ws, telemetry.ReportOptions{
		Format: telemetry.FormatMarkdown,
	})
	require.NoError(t, err)
	assert.Contains(t, output, "Primary Model",
		"markdown header must include Primary Model column")
	assert.Contains(t, output, "claude-sonnet-4.6",
		"PRIMARY_MODEL column must show the primary model name")
}

func TestFormatSessionTable_NoPrimaryModel_ShowsDash(t *testing.T) {
	// Sessions with no tokens_by_model should show "-" in the PRIMARY_MODEL column.
	ws := t.TempDir()
	writeGhostTrendJSONL(t, ws)

	output, err := telemetry.GenerateReport(ws, telemetry.ReportOptions{
		Format: telemetry.FormatTable,
	})
	require.NoError(t, err)
	assert.Contains(t, output, "PRIMARY_MODEL",
		"table header must include PRIMARY_MODEL column even when no model data available")
}

// ---- Unit 5: --by model and --by class report dimensions --------------------

func TestGenerateReport_ByModel_GroupsByPrimaryModel(t *testing.T) {
	ws := t.TempDir()
	writeModelAwareJSONL(t, ws)

	output, err := telemetry.GenerateReport(ws, telemetry.ReportOptions{
		GroupBy: "model",
		Format:  telemetry.FormatTable,
	})
	require.NoError(t, err)
	assert.Contains(t, output, "claude-sonnet-4.6",
		"--by model must show primary model names")
	assert.Contains(t, output, "gpt-5.4",
		"--by model must show primary model names for all sessions")
}

func TestGenerateReport_ByClass_GroupsByModelClass(t *testing.T) {
	ws := t.TempDir()
	writeModelAwareJSONL(t, ws)

	output, err := telemetry.GenerateReport(ws, telemetry.ReportOptions{
		GroupBy: "class",
		Format:  telemetry.FormatTable,
	})
	require.NoError(t, err)
	assert.Contains(t, output, "sonnet",
		"--by class must show model class sonnet")
	assert.Contains(t, output, "gpt",
		"--by class must show model class gpt")
}

func TestGenerateReport_ByModel_JSON_Valid(t *testing.T) {
	ws := t.TempDir()
	writeModelAwareJSONL(t, ws)

	output, err := telemetry.GenerateReport(ws, telemetry.ReportOptions{
		GroupBy: "model",
		Format:  telemetry.FormatJSON,
	})
	require.NoError(t, err)
	var decoded any
	require.NoError(t, json.Unmarshal([]byte(output), &decoded),
		"--by model with json format must produce valid JSON")
}

func TestGenerateReport_ByClass_Markdown_Valid(t *testing.T) {
	ws := t.TempDir()
	writeModelAwareJSONL(t, ws)

	output, err := telemetry.GenerateReport(ws, telemetry.ReportOptions{
		GroupBy: "class",
		Format:  telemetry.FormatMarkdown,
	})
	require.NoError(t, err)
	assert.Contains(t, output, "|",
		"--by class markdown output must contain table delimiter")
}

func TestGenerateReport_InvalidGroupBy_ReturnsError(t *testing.T) {
	ws := t.TempDir()
	writeModelAwareJSONL(t, ws)

	_, err := telemetry.GenerateReport(ws, telemetry.ReportOptions{
		GroupBy: "invalid",
		Format:  telemetry.FormatTable,
	})
	require.Error(t, err, "unsupported group-by value must return an error")
}

// ---- Unit 6: --by class grouping in trend report ----------------------------

// writeClassTrendJSONL creates a fixture with sessions of different model
// classes harvested on the same date to verify trend grouping by class.
func writeClassTrendJSONL(t *testing.T, workspacePath string) {
	t.Helper()
	backlogitDir := filepath.Join(workspacePath, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogitDir, 0o755))
	records := []string{
		// Two sonnet sessions
		`{"record_type":"session_summary","harvested_at":"2026-05-08T00:00:00Z","session_id":"ct-1","branch":"main","repository":"repo","total_tokens":1500,"model_calls":2,"tool_calls":3,"tokens_by_model":{"claude-sonnet-4.6":1500},"compaction_count":0,"model_class":"sonnet"}`,
		`{"record_type":"session_summary","harvested_at":"2026-05-08T00:00:00Z","session_id":"ct-2","branch":"feat","repository":"repo","total_tokens":1000,"model_calls":1,"tool_calls":2,"tokens_by_model":{"claude-sonnet-4.6":1000},"compaction_count":0,"model_class":"sonnet"}`,
		// One gpt session
		`{"record_type":"session_summary","harvested_at":"2026-05-08T00:00:00Z","session_id":"ct-3","branch":"main","repository":"repo","total_tokens":400,"model_calls":1,"tool_calls":1,"tokens_by_model":{"gpt-5.4":400},"compaction_count":0,"model_class":"gpt"}`,
	}
	content := strings.Join(records, "\n") + "\n"
	require.NoError(t, os.WriteFile(
		filepath.Join(backlogitDir, "telemetry-sessions.jsonl"),
		[]byte(content), 0o644,
	))
}

func TestGenerateTrendReport_ByClass_GroupsByModelClass(t *testing.T) {
	ws := t.TempDir()
	writeClassTrendJSONL(t, ws)

	output, err := telemetry.GenerateTrendReport(ws, telemetry.TrendOptions{
		By:     "class",
		Format: telemetry.FormatTable,
	})
	require.NoError(t, err)
	assert.Contains(t, output, "sonnet",
		"--by class trend must show sonnet group")
	assert.Contains(t, output, "gpt",
		"--by class trend must show gpt group")
}

func TestGenerateTrendReport_ByClass_JSON_AggregatesCorrectly(t *testing.T) {
	ws := t.TempDir()
	writeClassTrendJSONL(t, ws)

	output, err := telemetry.GenerateTrendReport(ws, telemetry.TrendOptions{
		By:     "class",
		Format: telemetry.FormatJSON,
	})
	require.NoError(t, err)

	var groups []telemetry.TrendGroup
	require.NoError(t, json.Unmarshal([]byte(output), &groups))
	require.Len(t, groups, 2, "expected 2 class groups: sonnet and gpt")

	totals := make(map[string]int)
	for _, g := range groups {
		totals[g.Group] = g.TotalTokens
	}
	assert.Equal(t, 2500, totals["sonnet"],
		"sonnet group should aggregate tokens from both sonnet sessions")
	assert.Equal(t, 400, totals["gpt"],
		"gpt group should have tokens from the gpt session")
}

func TestGenerateTrendReport_ByClass_FallbackDerivation(t *testing.T) {
	// Sessions without model_class field should derive it from TokensByModel.
	ws := t.TempDir()
	backlogitDir := filepath.Join(ws, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogitDir, 0o755))
	records := []string{
		// No model_class field — should derive from tokens_by_model
		`{"record_type":"session_summary","harvested_at":"2026-05-08T00:00:00Z","session_id":"fb-1","branch":"main","repository":"repo","total_tokens":800,"model_calls":1,"tool_calls":2,"tokens_by_model":{"claude-haiku-4.5":800},"compaction_count":0}`,
	}
	content := strings.Join(records, "\n") + "\n"
	require.NoError(t, os.WriteFile(
		filepath.Join(backlogitDir, "telemetry-sessions.jsonl"),
		[]byte(content), 0o644,
	))

	output, err := telemetry.GenerateTrendReport(ws, telemetry.TrendOptions{
		By:     "class",
		Format: telemetry.FormatTable,
	})
	require.NoError(t, err)
	assert.Contains(t, output, "haiku",
		"--by class trend should derive class from tokens_by_model when model_class is absent")
}

func TestGenerateTrendReport_InvalidBy_ReturnsError(t *testing.T) {
	ws := t.TempDir()
	writeClassTrendJSONL(t, ws)

	_, err := telemetry.GenerateTrendReport(ws, telemetry.TrendOptions{
		By:     "invalid",
		Format: telemetry.FormatTable,
	})
	require.Error(t, err, "unsupported By value must return an error")
}
