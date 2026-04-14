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

func TestMetadataCatalogCommand_PrintsCatalogJSON(t *testing.T) {
	root := setupCLIWorkspace(t)
	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--cwd", root, "metadata", "catalog"})

	err := cmd.Execute()

	require.NoError(t, err)
	assert.Contains(t, buf.String(), "\"artifact_types\"")
	assert.Contains(t, buf.String(), "\"mcp_tools\"")
}

func TestMetadataExportCommand_WritesFile(t *testing.T) {
	root := setupCLIWorkspace(t)
	target := filepath.Join(".github", "instructions", "backlogit-command-map.md")

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--cwd", root, "metadata", "export-command-map", target})

	err := cmd.Execute()

	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Wrote command map")
	data, readErr := os.ReadFile(filepath.Join(root, target))
	require.NoError(t, readErr)
	assert.Contains(t, string(data), "## CLI Commands")
}
