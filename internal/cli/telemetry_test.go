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
