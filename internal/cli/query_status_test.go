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

// TASK-002.04.07: Implement CLI query and status commands.

func TestQueryCommand_SelectStatement(t *testing.T) {
	// Arrange
	root := setupCLIWorkspace(t)
	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)

	now := time.Now()
	require.NoError(t, db.UpsertItem(ctx, ws.DB, &models.Artifact{
		ID: "T001", Title: "Query test", Status: models.StatusQueued,
		ArtifactType: "task", CreatedAt: now, UpdatedAt: now,
	}))
	ws.Close()

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--cwd", root, "query", "SELECT id, title FROM items"})

	// Act
	err = cmd.Execute()

	// Assert
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "T001")
}

func TestQueryCommand_RejectsNonSelect(t *testing.T) {
	// Arrange
	root := setupCLIWorkspace(t)

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--cwd", root, "query", "DELETE FROM items"})

	// Act
	err := cmd.Execute()

	// Assert
	require.Error(t, err)
}

func TestStatusCommand_DisplaysSummary(t *testing.T) {
	// Arrange
	root := setupCLIWorkspace(t)
	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)

	now := time.Now()
	require.NoError(t, db.UpsertItem(ctx, ws.DB, &models.Artifact{
		ID: "T001", Title: "Status test", Status: models.StatusQueued,
		ArtifactType: "task", CreatedAt: now, UpdatedAt: now,
	}))
	require.NoError(t, db.UpsertItem(ctx, ws.DB, &models.Artifact{
		ID: "B001", Title: "Bug status", Status: models.StatusActive,
		ArtifactType: "bug", CreatedAt: now, UpdatedAt: now,
	}))
	ws.Close()

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--cwd", root, "status"})

	// Act
	err = cmd.Execute()

	// Assert
	require.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "task")
	assert.Contains(t, output, "bug")
}

func TestStatusCommand_EmptyWorkspace(t *testing.T) {
	// Arrange
	root := setupCLIWorkspace(t)

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--cwd", root, "status"})

	// Act
	err := cmd.Execute()

	// Assert
	require.NoError(t, err)
}
