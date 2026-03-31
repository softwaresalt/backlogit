package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/cli"
	"github.com/backlogit/backlogit/internal/config"
)

// TASK-002.04.01: Implement CLI add command.

func setupCLIWorkspace(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	backlogitDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogitDir, 0o755))
	require.NoError(t, config.WriteDefaults(backlogitDir))
	return root
}

func TestAddCommand_CreatesArtifact(t *testing.T) {
	// Arrange
	root := setupCLIWorkspace(t)
	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--cwd", root, "add", "--type", "task", "--title", "Test task"})

	// Act
	err := cmd.Execute()

	// Assert
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "task")
}

func TestAddCommand_WithDescription(t *testing.T) {
	// Arrange
	root := setupCLIWorkspace(t)
	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--cwd", root, "add", "--type", "task", "--title", "Described task", "--description", "A detailed description"})

	// Act
	err := cmd.Execute()

	// Assert
	require.NoError(t, err)
}

func TestAddCommand_MissingType(t *testing.T) {
	// Arrange
	root := setupCLIWorkspace(t)
	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--cwd", root, "add", "--title", "No type"})

	// Act
	err := cmd.Execute()

	// Assert
	require.Error(t, err)
}

func TestAddCommand_MissingTitle(t *testing.T) {
	// Arrange
	root := setupCLIWorkspace(t)
	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--cwd", root, "add", "--type", "task"})

	// Act
	err := cmd.Execute()

	// Assert
	require.Error(t, err)
}

func TestAddCommand_CreatesMarkdownFile(t *testing.T) {
	// Arrange
	root := setupCLIWorkspace(t)
	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--cwd", root, "add", "--type", "task", "--title", "File check"})

	// Act
	err := cmd.Execute()
	require.NoError(t, err)

	// Assert — at least one .md file should exist in the workspace
	found := false
	require.NoError(t, filepath.WalkDir(filepath.Join(root, ".backlogit"), func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if filepath.Ext(path) == ".md" {
			found = true
		}
		return nil
	}))
	assert.True(t, found, "expected at least one .md file created")
}
