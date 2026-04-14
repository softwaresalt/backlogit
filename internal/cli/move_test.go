package cli_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/cli"
	"github.com/backlogit/backlogit/internal/core"
	"github.com/backlogit/backlogit/internal/db"
)

// TASK-002.04.05: Implement CLI move command.

func TestMoveCommand_ChangesStatus(t *testing.T) {
	// Arrange
	root := setupCLIWorkspace(t)
	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)

	feat, err := core.CreateArtifact(ctx, ws, "Move feature", "feature")
	require.NoError(t, err)
	artifact, err := core.CreateArtifact(ctx, ws, "Move test", "task", core.WithParent(feat.ID))
	require.NoError(t, err)
	// Transition to active so queued→done is not needed (queued→done is invalid).
	_, err = core.UpdateArtifact(ctx, ws, artifact.ID, map[string]any{"status": "active"})
	require.NoError(t, err)
	_, err = db.Rehydrate(ctx, core.WorkspaceStorageRoot(ws.RootPath), ws.DB)
	require.NoError(t, err)
	ws.Close()

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--cwd", root, "move", artifact.ID, "--status", "done"})

	// Act
	err = cmd.Execute()

	// Assert
	require.NoError(t, err)
}

func TestMoveCommand_MissingStatus(t *testing.T) {
	// Arrange
	root := setupCLIWorkspace(t)
	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)

	feat, err := core.CreateArtifact(ctx, ws, "Missing-status feature", "feature")
	require.NoError(t, err)
	artifact, err := core.CreateArtifact(ctx, ws, "Move no status", "task", core.WithParent(feat.ID))
	require.NoError(t, err)
	ws.Close()

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--cwd", root, "move", artifact.ID})

	// Act
	err = cmd.Execute()

	// Assert
	require.Error(t, err)
}

func TestMoveCommand_NotFound(t *testing.T) {
	// Arrange
	root := setupCLIWorkspace(t)

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--cwd", root, "move", "NONEXISTENT", "--status", "done"})

	// Act
	err := cmd.Execute()

	// Assert
	require.Error(t, err)
}

func TestMoveCommand_OutputContainsConfirmation(t *testing.T) {
	// Arrange
	root := setupCLIWorkspace(t)
	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)

	feat, err := core.CreateArtifact(ctx, ws, "Confirm feature", "feature")
	require.NoError(t, err)
	artifact, err := core.CreateArtifact(ctx, ws, "Move confirm", "task", core.WithParent(feat.ID))
	require.NoError(t, err)
	_, err = db.Rehydrate(ctx, core.WorkspaceStorageRoot(ws.RootPath), ws.DB)
	require.NoError(t, err)
	ws.Close()

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--cwd", root, "move", artifact.ID, "--status", "active"})

	// Act
	err = cmd.Execute()

	// Assert
	require.NoError(t, err)
	assert.NotEmpty(t, buf.String())
}
