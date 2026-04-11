package cli_test

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
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

	feat, err := core.CreateArtifact(ctx, ws, "Update feature", "feature")
	require.NoError(t, err)
	artifact, err := core.CreateArtifact(ctx, ws, "Update test", "task", core.WithParent(feat.ID))
	require.NoError(t, err)
	_, err = db.Rehydrate(ctx, core.WorkspaceStorageRoot(ws.RootPath), ws.DB)
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

	feat, err := core.CreateArtifact(ctx, ws, "Title feature", "feature")
	require.NoError(t, err)
	artifact, err := core.CreateArtifact(ctx, ws, "Original title", "task", core.WithParent(feat.ID))
	require.NoError(t, err)
	_, err = db.Rehydrate(ctx, core.WorkspaceStorageRoot(ws.RootPath), ws.DB)
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

	feat, err := core.CreateArtifact(ctx, ws, "ID feature", "feature")
	require.NoError(t, err)
	artifact, err := core.CreateArtifact(ctx, ws, "ID test", "task", core.WithParent(feat.ID))
	require.NoError(t, err)
	_, err = db.Rehydrate(ctx, core.WorkspaceStorageRoot(ws.RootPath), ws.DB)
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

func TestUpdateCommand_Description(t *testing.T) {
	root := setupCLIWorkspace(t)
	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)

	feat, err := core.CreateArtifact(ctx, ws, "Desc feature", "feature")
	require.NoError(t, err)
	_, err = db.Rehydrate(ctx, core.WorkspaceStorageRoot(ws.RootPath), ws.DB)
	require.NoError(t, err)
	ws.Close()

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--cwd", root, "update", feat.ID, "--description", "new description text"})

	err = cmd.Execute()

	require.NoError(t, err)
}

func TestUpdateCommand_AssignedTo(t *testing.T) {
	root := setupCLIWorkspace(t)
	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)

	feat, err := core.CreateArtifact(ctx, ws, "Assigned feature", "feature")
	require.NoError(t, err)
	_, err = db.Rehydrate(ctx, core.WorkspaceStorageRoot(ws.RootPath), ws.DB)
	require.NoError(t, err)
	ws.Close()

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--cwd", root, "update", feat.ID, "--assigned-to", "agent"})

	err = cmd.Execute()

	require.NoError(t, err)
}

func TestUpdateCommand_Labels(t *testing.T) {
	root := setupCLIWorkspace(t)
	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)

	feat, err := core.CreateArtifact(ctx, ws, "Label feature", "feature")
	require.NoError(t, err)
	_, err = db.Rehydrate(ctx, core.WorkspaceStorageRoot(ws.RootPath), ws.DB)
	require.NoError(t, err)
	ws.Close()

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--cwd", root, "update", feat.ID, "--labels", "alpha,beta"})

	err = cmd.Execute()

	require.NoError(t, err)
	content := readArtifactFile(t, root, feat.ID)
	assert.Contains(t, content, "- alpha")
	assert.Contains(t, content, "- beta")
}

func TestUpdateCommand_EmptyLabels(t *testing.T) {
	root := setupCLIWorkspace(t)
	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)

	feat, err := core.CreateArtifact(ctx, ws, "Empty label feature", "feature")
	require.NoError(t, err)
	_, err = db.Rehydrate(ctx, core.WorkspaceStorageRoot(ws.RootPath), ws.DB)
	require.NoError(t, err)
	ws.Close()

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--cwd", root, "update", feat.ID, "--labels", ""})

	err = cmd.Execute()

	require.NoError(t, err)
}

func TestUpdateCommand_StatusAndSection(t *testing.T) {
	root := setupCLIWorkspace(t)
	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)

	feat, err := core.CreateArtifact(ctx, ws, "Relocate feature", "feature", core.WithStatus("active"))
	require.NoError(t, err)
	_, err = db.Rehydrate(ctx, core.WorkspaceStorageRoot(ws.RootPath), ws.DB)
	require.NoError(t, err)
	ws.Close()

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--cwd", root, "update", feat.ID, "--status", "active", "--section", "description=Updated description after relocation"})

	err = cmd.Execute()

	require.NoError(t, err)
}

func TestUpdateCommand_NoDuplicateWrites(t *testing.T) {
	root := setupCLIWorkspace(t)
	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)

	feat, err := core.CreateArtifact(ctx, ws, "Dedup feature", "feature")
	require.NoError(t, err)
	artifact, err := core.CreateArtifact(ctx, ws, "Dedup task", "task", core.WithParent(feat.ID))
	require.NoError(t, err)
	_, err = db.Rehydrate(ctx, core.WorkspaceStorageRoot(ws.RootPath), ws.DB)
	require.NoError(t, err)
	origPath, err := core.FindArtifactPath(ctx, ws, artifact.ID)
	require.NoError(t, err)
	ws.Close()

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--cwd", root, "update", artifact.ID, "--status", "done"})

	require.NoError(t, cmd.Execute())

	// Original queue/ path must be absent — no ghost file from a stale WriteArtifactFile call.
	_, statErr := os.Stat(origPath)
	assert.True(t, os.IsNotExist(statErr), "original path should be absent after relocation: %s", origPath)
}
