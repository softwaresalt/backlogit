package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/cli"
	"github.com/backlogit/backlogit/internal/config"
)

// TASK-002.04.01: Implement CLI add command.

// extractID extracts the artifact ID from "Created <type>: <id>" output.
func extractID(t *testing.T, output string) string {
	t.Helper()
	// Output format: "Created feature: 001-F\n"
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		parts := strings.SplitN(line, ": ", 2)
		if len(parts) == 2 {
			return strings.TrimSpace(parts[1])
		}
	}
	t.Fatalf("could not extract ID from output: %q", output)
	return ""
}

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
	// Create parent feature first
	featCmd := cli.NewRootCommand()
	featBuf := new(bytes.Buffer)
	featCmd.SetOut(featBuf)
	featCmd.SetErr(featBuf)
	featCmd.SetArgs([]string{"--cwd", root, "add", "--type", "feature", "--title", "Parent feature"})
	require.NoError(t, featCmd.Execute())
	featID := extractID(t, featBuf.String())

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--cwd", root, "add", "--type", "task", "--title", "Test task", "--parent", featID})

	// Act
	err := cmd.Execute()

	// Assert
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "task")
}

func TestAddCommand_WithDescription(t *testing.T) {
	// Arrange
	root := setupCLIWorkspace(t)
	featCmd := cli.NewRootCommand()
	featBuf := new(bytes.Buffer)
	featCmd.SetOut(featBuf)
	featCmd.SetErr(featBuf)
	featCmd.SetArgs([]string{"--cwd", root, "add", "--type", "feature", "--title", "Desc feature"})
	require.NoError(t, featCmd.Execute())
	featID := extractID(t, featBuf.String())

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--cwd", root, "add", "--type", "task", "--title", "Described task", "--parent", featID, "--description", "A detailed description"})

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
	// Create parent feature first
	featCmd := cli.NewRootCommand()
	featBuf := new(bytes.Buffer)
	featCmd.SetOut(featBuf)
	featCmd.SetErr(featBuf)
	featCmd.SetArgs([]string{"--cwd", root, "add", "--type", "feature", "--title", "File feature"})
	require.NoError(t, featCmd.Execute())
	featID := extractID(t, featBuf.String())

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--cwd", root, "add", "--type", "task", "--title", "File check", "--parent", featID})

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
