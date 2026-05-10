package cli_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/cli"
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

func TestTelemetrySchemaSubcmd_Exists(t *testing.T) {
	cwd := t.TempDir()
	root := cli.NewTelemetryCmd(&cwd)
	schema, _, err := root.Find([]string{"schema"})
	require.NoError(t, err)
	require.NotNil(t, schema, "'schema' subcommand should be registered")
}

func TestTelemetrySchemaSubcmd_AcceptsFormatFlag(t *testing.T) {
	cwd := t.TempDir()
	root := cli.NewTelemetryCmd(&cwd)
	schema, _, err := root.Find([]string{"schema"})
	require.NoError(t, err)
	require.NotNil(t, schema)

	formatFlag := schema.Flags().Lookup("format")
	require.NotNil(t, formatFlag, "schema subcommand must accept --format flag")
}

func TestTelemetrySchemaSubcmd_RunsText(t *testing.T) {
	cwd := t.TempDir()
	root := cli.NewTelemetryCmd(&cwd)
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"schema", "--format", "text"})
	err := root.Execute()
	require.NoError(t, err)
	assert.Contains(t, out.String(), "JSONL Fact Tables")
	assert.Contains(t, out.String(), "SQL Tables")
}

func TestTelemetrySchemaSubcmd_RunsJSON(t *testing.T) {
	cwd := t.TempDir()
	root := cli.NewTelemetryCmd(&cwd)
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"schema", "--format", "json"})
	err := root.Execute()
	require.NoError(t, err)
	assert.Contains(t, out.String(), `"fact_tables"`)
	assert.Contains(t, out.String(), `"sql_tables"`)
}

func TestTelemetrySchemaSubcmd_RunsMarkdown(t *testing.T) {
	cwd := t.TempDir()
	root := cli.NewTelemetryCmd(&cwd)
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"schema", "--format", "markdown"})
	err := root.Execute()
	require.NoError(t, err)
	assert.Contains(t, out.String(), "## JSONL Fact Tables")
	assert.Contains(t, out.String(), "## SQL Tables")
}

func TestTelemetryBranchSubcmd_Exists(t *testing.T) {
	cwd := t.TempDir()
	root := cli.NewTelemetryCmd(&cwd)
	branch, _, err := root.Find([]string{"branch"})
	require.NoError(t, err)
	require.NotNil(t, branch, "'branch' subcommand should be registered")
}

func TestTelemetryBranchSubcmd_AcceptsFlags(t *testing.T) {
	cwd := t.TempDir()
	root := cli.NewTelemetryCmd(&cwd)
	branch, _, err := root.Find([]string{"branch"})
	require.NoError(t, err)

	for _, name := range []string{"format", "type", "limit"} {
		f := branch.Flags().Lookup(name)
		assert.NotNil(t, f, "branch subcommand must accept --%s flag", name)
	}
}

func TestTelemetryBranchSubcmd_RunsNoData(t *testing.T) {
	cwd := t.TempDir()
	root := cli.NewTelemetryCmd(&cwd)
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"branch"})
	err := root.Execute()
	require.NoError(t, err)
	assert.Contains(t, out.String(), "No telemetry data found")
}

func TestTelemetryBranchSubcmd_RejectsInvalidFormat(t *testing.T) {
	cwd := t.TempDir()
	root := cli.NewTelemetryCmd(&cwd)
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"branch", "--format", "invalid"})
	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported format")
}
