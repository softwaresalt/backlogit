package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/cli"
)

func writeShipment040CLITelemetryJSONL(t *testing.T, workspacePath string) {
	t.Helper()

	backlogitDir := filepath.Join(workspacePath, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogitDir, 0o755))

	content := `{"record_type":"session_summary","harvested_at":"2026-04-09T00:00:00Z","session_id":"sess-cli-md-1","branch":"main","repository":"backlogit","total_tokens":900,"prompt_tokens":600,"completion_tokens":300,"cached_tokens":0,"model_calls":1,"tool_calls":2,"tokens_by_model":{"claude-sonnet-4":900},"tool_calls_by_server":{"backlogit":2},"completed_tasks":[],"tokens_per_task":null,"compaction_count":0}` + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(backlogitDir, "telemetry-sessions.jsonl"), []byte(content), 0o644))
}

func TestTask039016_TelemetryReportFlagDocumentsMarkdown(t *testing.T) {
	if active := os.Getenv("BACKLOGIT_ACTIVE_HARNESS"); active != "039.016-T" {
		t.Skipf("shipment 040 harness inactive for 039.016-T (BACKLOGIT_ACTIVE_HARNESS=%q)", active)
	}

	cwd := t.TempDir()
	root := cli.NewTelemetryCmd(&cwd)
	report, _, err := root.Find([]string{"report"})
	require.NoError(t, err)
	require.NotNil(t, report)

	formatFlag := report.Flags().Lookup("format")
	require.NotNil(t, formatFlag)
	assert.Contains(t, formatFlag.Usage, "markdown")
}

func TestTask039016_TelemetryReportCommandSupportsMarkdown(t *testing.T) {
	if active := os.Getenv("BACKLOGIT_ACTIVE_HARNESS"); active != "039.016-T" {
		t.Skipf("shipment 040 harness inactive for 039.016-T (BACKLOGIT_ACTIVE_HARNESS=%q)", active)
	}

	cwd := t.TempDir()
	writeShipment040CLITelemetryJSONL(t, cwd)

	root := cli.NewTelemetryCmd(&cwd)
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"report", "--format", "markdown"})

	err := root.Execute()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "# Telemetry Report")
	assert.Contains(t, buf.String(), "sess-cli-md-1")
}
