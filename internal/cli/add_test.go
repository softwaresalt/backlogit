package cli_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/cli"
	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/models"
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

// readArtifactFile walks the .backlogit directory and returns the raw content of the
// Markdown file whose base name contains id. Fails the test if no such file is found.
func readArtifactFile(t *testing.T, root, id string) string {
	t.Helper()
	var content string
	found := false
	err := filepath.WalkDir(filepath.Join(root, ".backlogit"), func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if filepath.Ext(path) == ".md" && strings.Contains(filepath.Base(path), id) {
			raw, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			content = string(raw)
			found = true
		}
		return nil
	})
	require.NoError(t, err)
	require.True(t, found, "artifact file not found for ID %s", id)
	return content
}

// createFeatureAndGetID creates a parent feature via CLI and returns its ID.
func createFeatureAndGetID(t *testing.T, root, title string) string {
	t.Helper()
	featCmd := cli.NewRootCommand()
	featBuf := new(bytes.Buffer)
	featCmd.SetOut(featBuf)
	featCmd.SetErr(featBuf)
	featCmd.SetArgs([]string{"--cwd", root, "add", "--type", "feature", "--title", title})
	require.NoError(t, featCmd.Execute())
	return extractID(t, featBuf.String())
}

// getArtifactFromCLIWorkspace opens a fresh workspace, rehydrates, and returns the artifact.
func getArtifactFromCLIWorkspace(t *testing.T, root, id string) *models.Artifact {
	t.Helper()
	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)
	t.Cleanup(func() { ws.Close() })
	_, err = db.Rehydrate(ctx, core.WorkspaceStorageRoot(ws.RootPath), ws.DB)
	require.NoError(t, err)
	artifact, err := db.GetItem(ctx, ws.DB, id)
	require.NoError(t, err)
	return artifact
}

func TestAddCommand_Priority(t *testing.T) {
	root := setupCLIWorkspace(t)
	featID := createFeatureAndGetID(t, root, "Priority feature")

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--cwd", root, "add", "--type", "task", "--title", "Priority task", "--parent", featID, "--priority", "high"})

	require.NoError(t, cmd.Execute())

	id := extractID(t, buf.String())
	artifact := getArtifactFromCLIWorkspace(t, root, id)
	assert.Equal(t, "high", artifact.Priority)
}

func TestAddCommand_AssignedTo(t *testing.T) {
	root := setupCLIWorkspace(t)
	featID := createFeatureAndGetID(t, root, "Assigned feature")

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--cwd", root, "add", "--type", "task", "--title", "Assigned task", "--parent", featID, "--assigned-to", "agent-x"})

	require.NoError(t, cmd.Execute())

	id := extractID(t, buf.String())
	artifact := getArtifactFromCLIWorkspace(t, root, id)
	assert.Equal(t, "agent-x", artifact.AssignedTo)
}

func TestAddCommand_Labels(t *testing.T) {
	root := setupCLIWorkspace(t)
	featID := createFeatureAndGetID(t, root, "Labels feature")

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--cwd", root, "add", "--type", "task", "--title", "Labels task", "--parent", featID, "--labels", "a,b,c"})

	require.NoError(t, cmd.Execute())

	id := extractID(t, buf.String())
	artifact := getArtifactFromCLIWorkspace(t, root, id)
	assert.Equal(t, []string{"a", "b", "c"}, artifact.Labels)
}

func TestAddCommand_EmptyLabels(t *testing.T) {
	root := setupCLIWorkspace(t)
	featID := createFeatureAndGetID(t, root, "Empty labels feature")

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--cwd", root, "add", "--type", "task", "--title", "Empty labels task", "--parent", featID, "--labels", ""})

	require.NoError(t, cmd.Execute())

	id := extractID(t, buf.String())
	artifact := getArtifactFromCLIWorkspace(t, root, id)
	assert.Empty(t, artifact.Labels, "empty --labels should not persist any label entries")
}

func TestAddCommand_AllFlags(t *testing.T) {
	root := setupCLIWorkspace(t)
	featID := createFeatureAndGetID(t, root, "All-flags feature")

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"--cwd", root, "add",
		"--type", "task",
		"--title", "All-flags task",
		"--parent", featID,
		"--priority", "medium",
		"--sprint", "s1",
		"--assigned-to", "bot",
		"--owner", "team",
		"--labels", "x,y",
		"--dependencies", "001-F",
		"--references", "docs/foo.md",
		"--commit", "abc123",
	})

	require.NoError(t, cmd.Execute())

	id := extractID(t, buf.String())
	artifact := getArtifactFromCLIWorkspace(t, root, id)
	assert.Equal(t, "medium", artifact.Priority)
	assert.Equal(t, "s1", artifact.Sprint)
	assert.Equal(t, "bot", artifact.AssignedTo)
	assert.Equal(t, "team", artifact.Owner)
	assert.Equal(t, []string{"x", "y"}, artifact.Labels)
	assert.Equal(t, "abc123", artifact.Commit)
}
