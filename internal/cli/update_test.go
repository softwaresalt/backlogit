package cli_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/cli"
	"github.com/backlogit/backlogit/internal/core"
	"github.com/backlogit/backlogit/internal/db"
)

// TASK-002.04.04: Implement CLI update command.

func TestUpdateCommand_StatusChange(t *testing.T) {
	// Arrange
	root := setupCLIWorkspace(t)
	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)

	artifact, err := core.CreateArtifact(ctx, ws, "Update test", "task")
	require.NoError(t, err)
	_, err = db.Rehydrate(ctx, ws.RootPath, ws.DB)
	require.NoError(t, err)
	ws.Close()

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--cwd", root, "update", artifact.ID, "--status", "active"})

	// Act
	err = cmd.Execute()

	// Assert
	require.NoError(t, err)
}

func TestUpdateCommand_TitleChange(t *testing.T) {
	// Arrange
	root := setupCLIWorkspace(t)
	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)

	artifact, err := core.CreateArtifact(ctx, ws, "Original title", "task")
	require.NoError(t, err)
	_, err = db.Rehydrate(ctx, ws.RootPath, ws.DB)
	require.NoError(t, err)
	ws.Close()

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--cwd", root, "update", artifact.ID, "--title", "New title"})

	// Act
	err = cmd.Execute()

	// Assert
	require.NoError(t, err)
}

func TestUpdateCommand_IDChangeRejected(t *testing.T) {
	// Arrange
	root := setupCLIWorkspace(t)
	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)

	artifact, err := core.CreateArtifact(ctx, ws, "ID test", "task")
	require.NoError(t, err)
	_, err = db.Rehydrate(ctx, ws.RootPath, ws.DB)
	require.NoError(t, err)
	ws.Close()

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--cwd", root, "update", artifact.ID, "--id", "TAMPERED"})

	// Act
	err = cmd.Execute()

	// Assert — should be rejected
	require.Error(t, err)
}

func TestUpdateCommand_NotFound(t *testing.T) {
	// Arrange
	root := setupCLIWorkspace(t)

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--cwd", root, "update", "NONEXISTENT", "--status", "done"})

	// Act
	err := cmd.Execute()

	// Assert
	require.Error(t, err)
}
