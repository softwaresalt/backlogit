package cli_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/cli"
)

func TestNewTelemetryCmd_ReturnsCommand(t *testing.T) {
	// NewTelemetryCmd must return a non-nil cobra.Command for integration with
	// the CLI root before implementation is complete.
	cwd := t.TempDir()
	cmd := cli.NewTelemetryCmd(&cwd)
	require.NotNil(t, cmd)
	assert.Equal(t, "telemetry", cmd.Use)
}

func TestTelemetryHarvestSubcmd_ExistsAndRunsFails(t *testing.T) {
	cwd := t.TempDir()
	root := cli.NewTelemetryCmd(&cwd)
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))

	// Sub-command `harvest` must exist.
	harvest, _, err := root.Find([]string{"harvest"})
	require.NoError(t, err)
	require.NotNil(t, harvest, "'harvest' subcommand should be registered")
}

func TestTelemetryListSubcmd_Exists(t *testing.T) {
	cwd := t.TempDir()
	root := cli.NewTelemetryCmd(&cwd)
	list, _, err := root.Find([]string{"list"})
	require.NoError(t, err)
	require.NotNil(t, list, "'list' subcommand should be registered")
}

func TestTelemetryTopSubcmd_Exists(t *testing.T) {
	cwd := t.TempDir()
	root := cli.NewTelemetryCmd(&cwd)
	top, _, err := root.Find([]string{"top"})
	require.NoError(t, err)
	require.NotNil(t, top, "'top' subcommand should be registered")
}

// ---- Harness for 031.004-T: new subcommands and flags -----------------------

func TestTelemetryReportSubcmd_Exists(t *testing.T) {
	// 'report' subcommand must be registered (031.004-T).
	cwd := t.TempDir()
	root := cli.NewTelemetryCmd(&cwd)
	report, _, err := root.Find([]string{"report"})
	require.NoError(t, err)
	require.NotNil(t, report, "'report' subcommand should be registered")
}

func TestTelemetryReportSubcmd_AcceptsSessionFlag(t *testing.T) {
	cwd := t.TempDir()
	root := cli.NewTelemetryCmd(&cwd)
	report, _, err := root.Find([]string{"report"})
	require.NoError(t, err)
	require.NotNil(t, report)

	sessionFlag := report.Flags().Lookup("session")
	require.NotNil(t, sessionFlag, "report subcommand must accept --session flag")
}

func TestTelemetryReportSubcmd_AcceptsByFlag(t *testing.T) {
	cwd := t.TempDir()
	root := cli.NewTelemetryCmd(&cwd)
	report, _, err := root.Find([]string{"report"})
	require.NoError(t, err)
	require.NotNil(t, report)

	byFlag := report.Flags().Lookup("by")
	require.NotNil(t, byFlag, "report subcommand must accept --by flag")
}

func TestTelemetryReportSubcmd_AcceptsFormatFlag(t *testing.T) {
	cwd := t.TempDir()
	root := cli.NewTelemetryCmd(&cwd)
	report, _, err := root.Find([]string{"report"})
	require.NoError(t, err)
	require.NotNil(t, report)

	formatFlag := report.Flags().Lookup("format")
	require.NotNil(t, formatFlag, "report subcommand must accept --format flag")
}

func TestTelemetryHarvestSubcmd_AcceptsSinceFlag(t *testing.T) {
	// harvest must accept --since DATE for 031.004-T.
	cwd := t.TempDir()
	root := cli.NewTelemetryCmd(&cwd)
	harvest, _, err := root.Find([]string{"harvest"})
	require.NoError(t, err)
	require.NotNil(t, harvest)

	sinceFlag := harvest.Flags().Lookup("since")
	require.NotNil(t, sinceFlag, "harvest subcommand must accept --since flag")
}

func TestTelemetryHarvestSubcmd_AcceptsForceFlag(t *testing.T) {
	// harvest must accept --force for 031.004-T.
	cwd := t.TempDir()
	root := cli.NewTelemetryCmd(&cwd)
	harvest, _, err := root.Find([]string{"harvest"})
	require.NoError(t, err)
	require.NotNil(t, harvest)

	forceFlag := harvest.Flags().Lookup("force")
	require.NotNil(t, forceFlag, "harvest subcommand must accept --force flag")
}

func TestTelemetryTopSubcmd_AcceptsNFlag(t *testing.T) {
	cwd := t.TempDir()
	root := cli.NewTelemetryCmd(&cwd)
	top, _, err := root.Find([]string{"top"})
	require.NoError(t, err)
	require.NotNil(t, top)

	nFlag := top.Flags().Lookup("n")
	require.NotNil(t, nFlag, "top subcommand must accept --n flag")
}
