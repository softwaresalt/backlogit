package telemetry_test

// Harness for 047.002-T: regression coverage for token-ranked server reporting.
//
// These tests guard against a regression where formatServerTable aggregated raw
// tool call counts and sorted alphabetically instead of ranking servers by
// proportional token attribution. The previous behaviour produced misleading
// output: a server with many cheap calls ranked above one with fewer but
// token-intensive calls.
//
// Proportional attribution formula:
//
//	tokens_per_server[s] = session.TotalTokens × (ToolCallsByServer[s] / total_tool_calls)
//
// Fixture: two sessions where "copilot" accumulates more proportional tokens than
// "backlogit" across both sessions:
//
//	Session 1: TotalTokens=1000, backlogit:9 calls, copilot:1 call
//	  → backlogit=900t, copilot=100t
//	Session 2: TotalTokens=2000, backlogit:1 call, copilot:9 calls
//	  → backlogit=200t, copilot=1800t
//	Aggregate: copilot=1900t, backlogit=1100t
//
// Previous alphabetical sort: "backlogit" first (incorrect)
// Correct token-based sort:   "copilot" first
//
// Harness command:
//
//	go test ./internal/telemetry/ -run TestServerReport_Token -v

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/telemetry"
)

// writeTokenRankingJSONL writes a two-session telemetry-sessions.jsonl fixture
// designed to produce different rankings depending on whether sorting is by
// call count (alphabetical) or by proportional token attribution.
//
// Aggregate token attribution:
//
//	copilot  = 100 + 1800 = 1900 tokens  (should rank #1)
//	backlogit = 900 + 200 = 1100 tokens  (should rank #2)
//
// Current implementation ranks alphabetically: backlogit (#1), copilot (#2).
func writeTokenRankingJSONL(t *testing.T, workspacePath string) {
	t.Helper()
	backlogitDir := filepath.Join(workspacePath, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogitDir, 0o755))
	records := []string{
		// Session 1: backlogit dominates call count, copilot is minor.
		`{"record_type":"session_summary","harvested_at":"2026-04-25T00:00:00Z","session_id":"tok-sess-1","branch":"main","repository":"backlogit","total_tokens":1000,"prompt_tokens":700,"completion_tokens":300,"cached_tokens":0,"model_calls":2,"tool_calls":10,"tokens_by_model":{"claude-sonnet-4":1000},"tool_calls_by_server":{"backlogit":9,"copilot":1},"completed_tasks":[],"tokens_per_task":null,"compaction_count":0}`,
		// Session 2: copilot dominates call count, backlogit is minor.
		`{"record_type":"session_summary","harvested_at":"2026-04-25T00:00:00Z","session_id":"tok-sess-2","branch":"main","repository":"backlogit","total_tokens":2000,"prompt_tokens":1400,"completion_tokens":600,"cached_tokens":0,"model_calls":3,"tool_calls":10,"tokens_by_model":{"claude-opus-4.6":2000},"tool_calls_by_server":{"backlogit":1,"copilot":9},"completed_tasks":[],"tokens_per_task":null,"compaction_count":0}`,
	}
	content := strings.Join(records, "\n") + "\n"
	require.NoError(t, os.WriteFile(
		filepath.Join(backlogitDir, "telemetry-sessions.jsonl"),
		[]byte(content), 0o644,
	))
}

// dataRowsFromTableOutput splits a table-format report into non-header data rows.
// Skips the first two lines (header + separator) and returns only non-empty lines.
func dataRowsFromTableOutput(output string) []string {
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	var rows []string
	for i, line := range lines {
		if i < 2 { // skip header and separator
			continue
		}
		line = strings.TrimSpace(line)
		if line != "" {
			rows = append(rows, line)
		}
	}
	return rows
}

// TestServerReport_TokenRanking_CopilotOutranksByTokens asserts that when
// "copilot" has more proportional tokens than "backlogit", it appears first in
// the server ranking table.
//
// CURRENTLY FAILS: formatServerTable sorts alphabetically, placing "backlogit"
// before "copilot" regardless of token attribution.
func TestServerReport_TokenRanking_CopilotOutranksByTokens(t *testing.T) {
	ws := t.TempDir()
	writeTokenRankingJSONL(t, ws)

	output, err := telemetry.GenerateReport(ws, telemetry.ReportOptions{
		GroupBy: "server",
		Format:  telemetry.FormatTable,
	})
	require.NoError(t, err)

	// Both servers must appear in the output.
	require.Contains(t, output, "copilot", "output must include copilot server")
	require.Contains(t, output, "backlogit", "output must include backlogit server")

	// Parse data rows and assert copilot appears before backlogit.
	rows := dataRowsFromTableOutput(output)
	require.GreaterOrEqual(t, len(rows), 2, "output must have at least 2 data rows")
	assert.True(t,
		strings.HasPrefix(rows[0], "copilot"),
		"copilot (1900 proportional tokens) must rank first; got first row: %q", rows[0],
	)
	assert.True(t,
		strings.HasPrefix(rows[1], "backlogit"),
		"backlogit (1100 proportional tokens) must rank second; got second row: %q", rows[1],
	)
}

// TestServerReport_TokenColumn_Present asserts that the server table output
// includes a TOKENS column header, reflecting that ranking is now by token usage.
//
// CURRENTLY FAILS: formatServerTable currently renders "SERVER  TOOL_CALLS" only.
func TestServerReport_TokenColumn_Present(t *testing.T) {
	ws := t.TempDir()
	writeTokenRankingJSONL(t, ws)

	output, err := telemetry.GenerateReport(ws, telemetry.ReportOptions{
		GroupBy: "server",
		Format:  telemetry.FormatTable,
	})
	require.NoError(t, err)
	assert.Contains(t, output, "TOKENS",
		"server ranking table must include a TOKENS column header")
}

// TestServerReport_Limit_AppliedAfterTokenSort asserts that when Limit=1,
// the single row returned is the top-token server ("copilot"), not the
// alphabetically-first server ("backlogit").
//
// CURRENTLY FAILS: formatServerTable applies limit after alphabetical sort,
// returning "backlogit" (alphabetically first, not token-first).
func TestServerReport_Limit_AppliedAfterTokenSort(t *testing.T) {
	ws := t.TempDir()
	writeTokenRankingJSONL(t, ws)

	output, err := telemetry.GenerateReport(ws, telemetry.ReportOptions{
		GroupBy: "server",
		Format:  telemetry.FormatTable,
		Limit:   1,
	})
	require.NoError(t, err)

	rows := dataRowsFromTableOutput(output)
	require.Len(t, rows, 1, "Limit=1 must produce exactly one data row")
	assert.True(t,
		strings.HasPrefix(rows[0], "copilot"),
		"Limit=1 must return the top-token server (copilot=1900t); got row: %q", rows[0],
	)
	assert.NotContains(t, output, "backlogit",
		"backlogit must not appear when Limit=1 and copilot outranks it by tokens")
}

// TestServerReport_TokenValues_Correct asserts that the reported token values
// match the expected proportional attribution totals:
//
//	copilot  = 1000×(1/10) + 2000×(9/10) = 100 + 1800 = 1900
//	backlogit = 1000×(9/10) + 2000×(1/10) = 900 + 200  = 1100
//
// CURRENTLY FAILS: the output does not include token values at all.
func TestServerReport_TokenValues_Correct(t *testing.T) {
	ws := t.TempDir()
	writeTokenRankingJSONL(t, ws)

	output, err := telemetry.GenerateReport(ws, telemetry.ReportOptions{
		GroupBy: "server",
		Format:  telemetry.FormatTable,
	})
	require.NoError(t, err)

	assert.Contains(t, output, "1900",
		"copilot proportional token total (1900) must appear in output")
	assert.Contains(t, output, "1100",
		"backlogit proportional token total (1100) must appear in output")
}
