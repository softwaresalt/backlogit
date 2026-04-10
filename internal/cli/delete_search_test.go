package cli_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/cli"
	"github.com/backlogit/backlogit/internal/core"
	"github.com/backlogit/backlogit/internal/db"
	"github.com/backlogit/backlogit/internal/models"
)

// TASK-002.04.06: Implement CLI delete and search commands.

func TestDeleteCommand_Force(t *testing.T) {
	// Arrange
	root := setupCLIWorkspace(t)
	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)

	feat, err := core.CreateArtifact(ctx, ws, "Delete feature", "feature")
	require.NoError(t, err)
	artifact, err := core.CreateArtifact(ctx, ws, "Delete me", "task", core.WithParent(feat.ID))
	require.NoError(t, err)
	_, err = db.Rehydrate(ctx, core.WorkspaceStorageRoot(ws.RootPath), ws.DB)
	require.NoError(t, err)
	ws.Close()

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--cwd", root, "delete", artifact.ID, "--force"})

	// Act
	err = cmd.Execute()

	// Assert
	require.NoError(t, err)
}

func TestDeleteCommand_NotFound(t *testing.T) {
	// Arrange
	root := setupCLIWorkspace(t)

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--cwd", root, "delete", "NONEXISTENT", "--force"})

	// Act
	err := cmd.Execute()

	// Assert
	require.Error(t, err)
}

func TestSearchCommand_FindsResults(t *testing.T) {
	// Arrange
	root := setupCLIWorkspace(t)
	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)

	now := time.Now()
	require.NoError(t, db.UpsertItem(ctx, ws.DB, &models.Artifact{
		ID: "T050", Title: "Searchable authentication fix", Status: models.StatusQueued,
		ArtifactType: "task", CreatedAt: now, UpdatedAt: now,
	}))
	ws.Close()

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--cwd", root, "search", "authentication"})

	// Act
	err = cmd.Execute()

	// Assert
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "T050")
}

func TestSearchCommand_NoResults(t *testing.T) {
	// Arrange
	root := setupCLIWorkspace(t)

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--cwd", root, "search", "xyznonexistent"})

	// Act
	err := cmd.Execute()

	// Assert
	require.NoError(t, err)
}

func TestSearchCommand_WithLimit(t *testing.T) {
	// Arrange
	root := setupCLIWorkspace(t)
	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)

	now := time.Now()
	for i := 0; i < 5; i++ {
		require.NoError(t, db.UpsertItem(ctx, ws.DB, &models.Artifact{
			ID: "S" + string(rune('1'+i)) + "00", Title: "Limit test task",
			Status: models.StatusQueued, ArtifactType: "task",
			CreatedAt: now, UpdatedAt: now,
		}))
	}
	ws.Close()

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--cwd", root, "search", "Limit", "--limit", "2"})

	// Act
	err = cmd.Execute()

	// Assert
	require.NoError(t, err)
}
