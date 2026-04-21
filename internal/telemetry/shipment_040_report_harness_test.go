package telemetry_test

import (
	"os"
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

func TestTask039013_ReportHarnessScenariosPending(t *testing.T) {
	requireShipment040TelemetryHarness(t, "039.013-T")

	t.Run("multi_session_fixture", func(t *testing.T) {
		t.Fatal("not implemented: add multi-session behavioral telemetry report coverage for 039.013-T")
	})

	t.Run("unsupported_format_validation", func(t *testing.T) {
		t.Fatal("not implemented: add unsupported format validation coverage for 039.013-T")
	})

	t.Run("unsupported_groupby_validation", func(t *testing.T) {
		t.Fatal("not implemented: add unsupported group-by validation coverage for 039.013-T")
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
