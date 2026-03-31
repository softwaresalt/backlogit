package cli_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/cli"
	"github.com/backlogit/backlogit/internal/core"
	"github.com/backlogit/backlogit/internal/db"
)

// TASK-002.04.03: Implement CLI get command.

func TestGetCommand_DisplaysArtifact(t *testing.T) {
	// Arrange
	root := setupCLIWorkspace(t)
	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)

	artifact, err := core.CreateArtifact(ctx, ws, "Get test", "task")
	require.NoError(t, err)
	_, err = db.Rehydrate(ctx, ws.RootPath, ws.DB)
	require.NoError(t, err)
	ws.Close()

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--cwd", root, "get", artifact.ID})

	// Act
	err = cmd.Execute()

	// Assert
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Get test")
}

func TestGetCommand_JSONOutput(t *testing.T) {
	// Arrange
	root := setupCLIWorkspace(t)
	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)

	artifact, err := core.CreateArtifact(ctx, ws, "JSON get test", "task")
	require.NoError(t, err)
	_, err = db.Rehydrate(ctx, ws.RootPath, ws.DB)
	require.NoError(t, err)
	ws.Close()

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--cwd", root, "get", artifact.ID, "--json"})

	// Act
	err = cmd.Execute()

	// Assert
	require.NoError(t, err)
	assert.Contains(t, buf.String(), `"id"`)
}

func TestGetCommand_SectionExtraction(t *testing.T) {
	// Arrange — create a file with sections
	root := setupCLIWorkspace(t)
	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)

	artifact, err := core.CreateArtifact(ctx, ws, "Section test", "task",
		core.WithDescription("<!-- BEGIN:description -->\nSection content\n<!-- END:description -->"),
	)
	require.NoError(t, err)
	_, err = db.Rehydrate(ctx, ws.RootPath, ws.DB)
	require.NoError(t, err)
	ws.Close()

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--cwd", root, "get", artifact.ID, "--section", "description"})

	// Act
	err = cmd.Execute()

	// Assert
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Section content")
}

func TestGetCommand_NotFound(t *testing.T) {
	// Arrange
	root := setupCLIWorkspace(t)

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--cwd", root, "get", "NONEXISTENT"})

	// Act
	err := cmd.Execute()

	// Assert
	require.Error(t, err)
}

// Ensure setupCLIWorkspace is available (defined in add_test.go, same package).
var _ = os.TempDir
var _ = filepath.Join
