package cli

// Tests for TASK-008.05: CLI move command must relocate artifact file per registry.yaml.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/db"
)

func setupMoveTestWorkspace(t *testing.T) (string, *core.Workspace) {
	t.Helper()
	root := t.TempDir()
	backlogitDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogitDir, 0o755))
	require.NoError(t, config.WriteDefaults(backlogitDir))

	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)
	t.Cleanup(func() { ws.Close() })

	return root, ws
}

// TASK-008.05: After changing status, the file must be in the directory
// mapped by registry.yaml for the new (type, status) pair.
func TestMoveCommand_RelocatesFileToTargetDir(t *testing.T) {
	// Arrange
	root, ws := setupMoveTestWorkspace(t)
	ctx := context.Background()

	feat, err := core.CreateArtifact(ctx, ws, "Relocate feature", "feature")
	require.NoError(t, err)
	artifact, err := core.CreateArtifact(ctx, ws, "Relocate test", "task", core.WithParent(feat.ID))
	require.NoError(t, err)
	require.NoError(t, db.UpsertItem(ctx, ws.DB, artifact))

	// Record original file path
	originalPath, err := core.FindArtifactPath(ctx, ws, artifact.ID)
	require.NoError(t, err)
	require.FileExists(t, originalPath)

	// Act — move to "active" then "done" status (queued→done is not a valid transition)
	cwd := root
	cmd := newMoveCommand(&cwd)
	cmd.SetArgs([]string{artifact.ID, "--status", "active"})
	err = cmd.Execute()
	require.NoError(t, err)

	cmd = newMoveCommand(&cwd)
	cmd.SetArgs([]string{artifact.ID, "--status", "done"})
	err = cmd.Execute()
	require.NoError(t, err)

	// Assert — file should be in a different directory (completed/archive/done)
	newPath, err := core.FindArtifactPath(ctx, ws, artifact.ID)
	require.NoError(t, err)
	assert.NotEqual(t, originalPath, newPath,
		"file should have been relocated to the directory for 'done' status")
	assert.Contains(t, filepath.ToSlash(newPath), "/.backlogit/",
		"relocated file should remain inside the .backlogit workspace")

	// The original path should no longer exist
	_, err = os.Stat(originalPath)
	assert.True(t, os.IsNotExist(err),
		"original file should have been removed after relocation")
}

// TASK-008.05: Target directory should be created if it doesn't exist.
func TestMoveCommand_CreatesTargetDirIfMissing(t *testing.T) {
	// Arrange
	root, ws := setupMoveTestWorkspace(t)
	ctx := context.Background()

	feat, err := core.CreateArtifact(ctx, ws, "DirCreation feature", "feature")
	require.NoError(t, err)
	artifact, err := core.CreateArtifact(ctx, ws, "Dir creation test", "task", core.WithParent(feat.ID))
	require.NoError(t, err)
	require.NoError(t, db.UpsertItem(ctx, ws.DB, artifact))

	// Act — move to status that maps to a potentially non-existent directory
	// (queued→review is not valid; go through queued→active→review)
	cwd := root
	cmd := newMoveCommand(&cwd)
	cmd.SetArgs([]string{artifact.ID, "--status", "active"})
	err = cmd.Execute()
	require.NoError(t, err)

	cmd = newMoveCommand(&cwd)
	cmd.SetArgs([]string{artifact.ID, "--status", "review"})
	err = cmd.Execute()

	// Assert — command should succeed even if target dir didn't exist
	require.NoError(t, err)

	// Verify the artifact can still be found
	newPath, err := core.FindArtifactPath(ctx, ws, artifact.ID)
	require.NoError(t, err)
	assert.FileExists(t, newPath)
	assert.Contains(t, filepath.ToSlash(newPath), "/.backlogit/",
		"relocated file should remain inside the .backlogit workspace")
}
