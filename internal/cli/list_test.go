package cli_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/cli"
	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/models"
)

// TASK-002.04.02: Implement CLI list command.

func TestListCommand_TableOutput(t *testing.T) {
	// Arrange
	root := setupCLIWorkspace(t)
	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)
	defer ws.Close()

	// Seed a task
	now := time.Now()
	require.NoError(t, db.UpsertItem(ctx, ws.DB, &models.Artifact{
		ID: "T001", Title: "Sample task", Status: models.StatusQueued,
		ArtifactType: "task", CreatedAt: now, UpdatedAt: now,
	}))

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--cwd", root, "list"})

	// Act
	err = cmd.Execute()

	// Assert
	require.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "T001")
	assert.Contains(t, output, "Sample task")
}

func TestListCommand_JSONOutput(t *testing.T) {
	// Arrange
	root := setupCLIWorkspace(t)
	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)
	defer ws.Close()

	now := time.Now()
	require.NoError(t, db.UpsertItem(ctx, ws.DB, &models.Artifact{
		ID: "T001", Title: "JSON task", Status: models.StatusQueued,
		ArtifactType: "task", CreatedAt: now, UpdatedAt: now,
	}))

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--cwd", root, "list", "--json"})

	// Act
	err = cmd.Execute()

	// Assert
	require.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, `"id"`)
}

func TestListCommand_FilterByType(t *testing.T) {
	// Arrange
	root := setupCLIWorkspace(t)
	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)
	defer ws.Close()

	now := time.Now()
	require.NoError(t, db.UpsertItem(ctx, ws.DB, &models.Artifact{
		ID: "T001", Title: "Task", Status: models.StatusQueued,
		ArtifactType: "task", CreatedAt: now, UpdatedAt: now,
	}))
	require.NoError(t, db.UpsertItem(ctx, ws.DB, &models.Artifact{
		ID: "B001", Title: "Bug", Status: models.StatusQueued,
		ArtifactType: "bug", CreatedAt: now, UpdatedAt: now,
	}))

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--cwd", root, "list", "--type", "bug"})

	// Act
	err = cmd.Execute()

	// Assert
	require.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "B001")
	assert.NotContains(t, output, "T001")
}
