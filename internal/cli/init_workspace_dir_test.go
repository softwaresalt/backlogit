package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/cli"
	"github.com/softwaresalt/backlogit/internal/config"
)

func TestInitCommand_CreatesBacklogDirectory(t *testing.T) {
	root := t.TempDir()

	buf := new(bytes.Buffer)
	cmd := cli.NewRootCommand()
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"init", "--cwd", root})

	require.NoError(t, cmd.Execute())
	assert.DirExists(t, filepath.Join(root, ".backlog"))
	assert.FileExists(t, filepath.Join(root, ".backlog", "config.yaml"))
	assert.NoDirExists(t, filepath.Join(root, ".backlogit"))
}

func TestInitCommand_RefusesLegacyWorkspaceRoot(t *testing.T) {
	root := t.TempDir()
	legacyDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(legacyDir, 0o755))
	require.NoError(t, config.WriteDefaults(legacyDir))

	buf := new(bytes.Buffer)
	cmd := cli.NewRootCommand()
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"init", "--cwd", root})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "workspace already exists at .backlogit")
}
