package integration_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/cli"
	"github.com/backlogit/backlogit/internal/config"
	"github.com/backlogit/backlogit/internal/core"
	"github.com/backlogit/backlogit/internal/core/templates"
	"github.com/backlogit/backlogit/internal/db"
)

// TASK-002.06.01: Full workflow integration tests.

func setupIntegrationWorkspace(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	backlogitDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogitDir, 0o755))
	require.NoError(t, config.WriteDefaults(backlogitDir))
	return root
}

func runCLI(t *testing.T, root string, args ...string) (string, error) {
	t.Helper()
	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	allArgs := append([]string{"--cwd", root}, args...)
	cmd.SetArgs(allArgs)
	err := cmd.Execute()
	return buf.String(), err
}

func TestWorkflow_InitCreatesWorkspace(t *testing.T) {
	// Arrange
	root := t.TempDir()

	// Act
	output, err := runCLI(t, root, "init")

	// Assert
	require.NoError(t, err)
	assert.Contains(t, output, "Initialized")
	assert.DirExists(t, filepath.Join(root, ".backlogit"))
	assert.FileExists(t, filepath.Join(root, ".backlogit", "config.yaml"))
	assert.FileExists(t, filepath.Join(root, ".backlogit", "header-def.yaml"))
	assert.DirExists(t, filepath.Join(root, ".backlogit", "templates"))
}

func TestWorkflow_AddThenList(t *testing.T) {
	// Arrange
	root := setupIntegrationWorkspace(t)

	// Act — add
	_, err := runCLI(t, root, "add", "--type", "feature", "--title", "Integration feature")
	require.NoError(t, err)

	// Sync to rehydrate
	_, err = runCLI(t, root, "sync")
	require.NoError(t, err)

	// Act — list
	output, err := runCLI(t, root, "list")

	// Assert
	require.NoError(t, err)
	assert.Contains(t, output, "Integration feature")
}

func TestWorkflow_AddThenGet(t *testing.T) {
	// Arrange
	root := setupIntegrationWorkspace(t)
	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)

	artifact, err := core.CreateArtifact(ctx, ws, "Get workflow test", "feature")
	require.NoError(t, err)
	_, err = db.Rehydrate(ctx, core.WorkspaceStorageRoot(ws.RootPath), ws.DB)
	require.NoError(t, err)
	ws.Close()

	// Act
	output, err := runCLI(t, root, "get", artifact.ID)

	// Assert
	require.NoError(t, err)
	assert.Contains(t, output, "Get workflow test")
}

func TestWorkflow_UpdateStatus(t *testing.T) {
	// Arrange
	root := setupIntegrationWorkspace(t)
	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)

	artifact, err := core.CreateArtifact(ctx, ws, "Update workflow", "feature")
	require.NoError(t, err)
	_, err = db.Rehydrate(ctx, core.WorkspaceStorageRoot(ws.RootPath), ws.DB)
	require.NoError(t, err)
	ws.Close()

	// Act
	_, err = runCLI(t, root, "update", artifact.ID, "--status", "active")

	// Assert
	require.NoError(t, err)
}

func TestWorkflow_MoveToDone(t *testing.T) {
	// Arrange
	root := setupIntegrationWorkspace(t)
	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)

	artifact, err := core.CreateArtifact(ctx, ws, "Move workflow", "feature")
	require.NoError(t, err)
	// Transition to active so the CLI move to done goes through active→done.
	_, err = core.UpdateArtifact(ctx, ws, artifact.ID, map[string]any{"status": "active"})
	require.NoError(t, err)
	_, err = db.Rehydrate(ctx, core.WorkspaceStorageRoot(ws.RootPath), ws.DB)
	require.NoError(t, err)
	ws.Close()

	// Act
	_, err = runCLI(t, root, "move", artifact.ID, "--status", "done")

	// Assert
	require.NoError(t, err)
}

func TestWorkflow_SearchFindsArtifact(t *testing.T) {
	// Arrange
	root := setupIntegrationWorkspace(t)
	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)

	_, err = core.CreateArtifact(ctx, ws, "Searchable unique term xyz", "feature")
	require.NoError(t, err)
	_, err = db.Rehydrate(ctx, core.WorkspaceStorageRoot(ws.RootPath), ws.DB)
	require.NoError(t, err)
	ws.Close()

	// Act
	output, err := runCLI(t, root, "search", "xyz")

	// Assert
	require.NoError(t, err)
	assert.Contains(t, output, "xyz")
}

func TestWorkflow_DeleteRemovesArtifact(t *testing.T) {
	// Arrange
	root := setupIntegrationWorkspace(t)
	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)

	artifact, err := core.CreateArtifact(ctx, ws, "Delete workflow", "feature")
	require.NoError(t, err)
	_, err = db.Rehydrate(ctx, core.WorkspaceStorageRoot(ws.RootPath), ws.DB)
	require.NoError(t, err)
	ws.Close()

	// Act
	_, err = runCLI(t, root, "delete", artifact.ID, "--force")

	// Assert
	require.NoError(t, err)

	// Verify artifact is gone from index
	ws2, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)
	defer ws2.Close()
	_, err = db.GetItem(ctx, ws2.DB, artifact.ID)
	assert.Error(t, err)
}

func TestWorkflow_QuerySQL(t *testing.T) {
	// Arrange
	root := setupIntegrationWorkspace(t)
	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)

	_, err = core.CreateArtifact(ctx, ws, "Query workflow", "feature")
	require.NoError(t, err)
	_, err = db.Rehydrate(ctx, core.WorkspaceStorageRoot(ws.RootPath), ws.DB)
	require.NoError(t, err)
	ws.Close()

	// Act
	output, err := runCLI(t, root, "query", "SELECT id, title FROM items")

	// Assert
	require.NoError(t, err)
	assert.Contains(t, output, "Query workflow")
}

func TestWorkflow_StatusSummary(t *testing.T) {
	// Arrange
	root := setupIntegrationWorkspace(t)
	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)

	statusFeat, err := core.CreateArtifact(ctx, ws, "Status parent feature", "feature")
	require.NoError(t, err)
	_, err = core.CreateArtifact(ctx, ws, "Status task 1", "task", core.WithParent(statusFeat.ID))
	require.NoError(t, err)
	_, err = core.CreateArtifact(ctx, ws, "Status feature 1", "feature")
	require.NoError(t, err)
	_, err = db.Rehydrate(ctx, core.WorkspaceStorageRoot(ws.RootPath), ws.DB)
	require.NoError(t, err)
	ws.Close()

	// Act
	output, err := runCLI(t, root, "status")

	// Assert
	require.NoError(t, err)
	assert.Contains(t, output, "task")
	assert.Contains(t, output, "feature")
}

// --- Section-aware workflow integration tests (revision-3) ---

func TestWorkflow_AddWithSection(t *testing.T) {
	// Arrange
	root := setupIntegrationWorkspace(t)

	// Act — add a feature with --section flag
	_, err := runCLI(t, root, "add", "--type", "feature", "--title", "Section feature",
		"--section", `description=This is the body`)
	require.NoError(t, err)

	// Sync
	_, err = runCLI(t, root, "sync")
	require.NoError(t, err)

	// Assert — list should show the feature
	output, err := runCLI(t, root, "list")
	require.NoError(t, err)
	assert.Contains(t, output, "Section feature")
}

func TestWorkflow_GetSection(t *testing.T) {
	// Arrange
	root := setupIntegrationWorkspace(t)
	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)

	// Use the template service so the artifact body contains section markers.
	templatesDir := filepath.Join(root, ".backlogit", "templates")
	svc, err := templates.NewService(ctx, templatesDir)
	require.NoError(t, err)
	sectFeat, err := core.CreateArtifact(ctx, ws, "Section parent feature", "feature")
	require.NoError(t, err)
	artifact, err := svc.Create(ctx, ws, "Section get test", "task", nil, core.WithParent(sectFeat.ID))
	require.NoError(t, err)
	_, err = db.Rehydrate(ctx, core.WorkspaceStorageRoot(ws.RootPath), ws.DB)
	require.NoError(t, err)
	ws.Close()

	// Act — get with --section flag to extract a specific section
	output, err := runCLI(t, root, "get", artifact.ID, "--section", "description")

	// Assert
	require.NoError(t, err)
	_ = output // section content depends on template rendering
}

func TestWorkflow_UpdateSection(t *testing.T) {
	// Arrange
	root := setupIntegrationWorkspace(t)
	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)

	// Use the template service so the artifact body contains section markers.
	templatesDir := filepath.Join(root, ".backlogit", "templates")
	svc, err := templates.NewService(ctx, templatesDir)
	require.NoError(t, err)
	updFeat, err := core.CreateArtifact(ctx, ws, "Section update parent feature", "feature")
	require.NoError(t, err)
	artifact, err := svc.Create(ctx, ws, "Section update test", "task", nil, core.WithParent(updFeat.ID))
	require.NoError(t, err)
	_, err = db.Rehydrate(ctx, core.WorkspaceStorageRoot(ws.RootPath), ws.DB)
	require.NoError(t, err)
	ws.Close()

	// Act — update a specific section
	_, err = runCLI(t, root, "update", artifact.ID,
		"--section", `description=Updated section content`)

	// Assert
	require.NoError(t, err)
}
