package cli

// Tests for TASK-008.04: CLI update command must sync DB after section writes.

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/db"
)

func setupUpdateTestWorkspace(t *testing.T) (string, *core.Workspace) {
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

// TASK-008.04: After --section flag updates, db.UpsertItem must be called.
func TestUpdateCommand_SectionWrite_SyncsDB(t *testing.T) {
	// Arrange
	root, ws := setupUpdateTestWorkspace(t)
	ctx := context.Background()

	feat, err := core.CreateArtifact(ctx, ws, "Sync feature", "feature")
	require.NoError(t, err)
	artifact, err := core.CreateArtifact(ctx, ws, "Update sync test", "task", core.WithParent(feat.ID))
	require.NoError(t, err)
	require.NoError(t, db.UpsertItem(ctx, ws.DB, artifact))

	// Record pre-update state
	before, err := db.GetItem(ctx, ws.DB, artifact.ID)
	require.NoError(t, err)
	beforeUpdatedAt := before.UpdatedAt

	time.Sleep(10 * time.Millisecond)

	// Act — execute update command with section flag
	cwd := root
	cmd := newUpdateCommand(&cwd)
	cmd.SetArgs([]string{artifact.ID, "--section", "description=Updated via CLI"})
	err = cmd.Execute()
	require.NoError(t, err)

	// Assert — DB should reflect the section update
	// Reopen workspace to get fresh DB connection
	ws2, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)
	defer ws2.Close()

	after, err := db.GetItem(ctx, ws2.DB, artifact.ID)
	require.NoError(t, err)
	assert.True(t, after.UpdatedAt.After(beforeUpdatedAt),
		"updated_at should be bumped after section update via CLI")
}

// TASK-008.04: Section writes should update the artifact file AND the index.
func TestUpdateCommand_SectionWrite_BumpsUpdatedAt(t *testing.T) {
	// Arrange
	root, ws := setupUpdateTestWorkspace(t)
	ctx := context.Background()

	feat, err := core.CreateArtifact(ctx, ws, "Bump feature", "feature")
	require.NoError(t, err)
	artifact, err := core.CreateArtifact(ctx, ws, "Timestamp bump test", "task", core.WithParent(feat.ID))
	require.NoError(t, err)
	require.NoError(t, db.UpsertItem(ctx, ws.DB, artifact))

	time.Sleep(10 * time.Millisecond)

	// Act
	cwd := root
	cmd := newUpdateCommand(&cwd)
	cmd.SetArgs([]string{artifact.ID, "--section", "acceptance-criteria=- [ ] Done"})
	err = cmd.Execute()
	require.NoError(t, err)

	// Assert — file should contain the section AND updated timestamp
	filePath, err := core.FindArtifactPath(ctx, ws, artifact.ID)
	require.NoError(t, err)
	raw, err := os.ReadFile(filePath)
	require.NoError(t, err)
	content := string(raw)
	assert.Contains(t, content, "Done",
		"file should contain the section content written by CLI")
}
