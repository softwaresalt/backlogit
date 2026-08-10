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

func TestMigrateCommand_WorkspaceDirDryRun(t *testing.T) {
	root := t.TempDir()
	legacyDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(legacyDir, 0o755))
	require.NoError(t, config.WriteDefaults(legacyDir))

	buf := new(bytes.Buffer)
	cmd := cli.NewRootCommand()
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"migrate", "--cwd", root, "--workspace-dir", "--dry-run"})

	require.NoError(t, cmd.Execute())
	assert.Contains(t, buf.String(), "would move")
	assert.DirExists(t, legacyDir)
	assert.NoDirExists(t, filepath.Join(root, ".backlog"))
}

func TestMigrateCommand_WorkspaceDirMovesLegacyRoot(t *testing.T) {
	root := t.TempDir()
	legacyDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(legacyDir, 0o755))
	require.NoError(t, config.WriteDefaults(legacyDir))

	buf := new(bytes.Buffer)
	cmd := cli.NewRootCommand()
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"migrate", "--cwd", root, "--workspace-dir"})

	require.NoError(t, cmd.Execute())
	assert.Contains(t, buf.String(), "Migrated workspace directory")
	assert.NoDirExists(t, legacyDir)
	assert.DirExists(t, filepath.Join(root, ".backlog"))
}
