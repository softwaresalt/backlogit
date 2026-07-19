package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/cli"
)

func requireNoSizingTODOCLI(t *testing.T, err error) {
	t.Helper()
	if err != nil && strings.Contains(err.Error(), "TODO: implement 108-F size estimation") {
		t.Fatalf("%v", err)
	}
}

func TestSE5CLIUpdateSizeProvenanceFlagsHarness(t *testing.T) {
	root := setupCLIWorkspace(t)
	id := createSizeTask(t, root)

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"--cwd", root,
		"update", id,
		"--size", "M",
		"--size-source", "human",
		"--size-ruleset-version", "ruleset-alpha",
	})

	err := cmd.Execute()
	requireNoSizingTODOCLI(t, err)
	require.NoError(t, err)
}

func TestSE5CLIInvalidSizeSourceHarness(t *testing.T) {
	root := setupCLIWorkspace(t)
	id := createSizeTask(t, root)

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--cwd", root, "update", id, "--size", "M", "--size-source", "robot"})

	err := cmd.Execute()
	requireNoSizingTODOCLI(t, err)
	require.Error(t, err)
	require.NotContains(t, err.Error(), "task is busy")
}
