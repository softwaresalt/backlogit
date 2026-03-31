package templates_test

// Tests for TASK-008.03: Template service Update must sync DB and set updated_at.

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/config"
	"github.com/backlogit/backlogit/internal/core"
	"github.com/backlogit/backlogit/internal/core/templates"
	"github.com/backlogit/backlogit/internal/db"
)

func setupServiceSyncWorkspace(t *testing.T) (*core.Workspace, *templates.Service) {
	t.Helper()
	root := t.TempDir()
	backlogitDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogitDir, 0o755))
	require.NoError(t, config.WriteDefaults(backlogitDir))

	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)
	t.Cleanup(func() { ws.Close() })

	templatesDir := filepath.Join(backlogitDir, "templates")
	svc, err := templates.NewService(ctx, templatesDir)
	require.NoError(t, err)

	return ws, svc
}

// TASK-008.03: After Update writes sections, the DB must reflect the change.
func TestUpdate_SyncsDBAfterSectionWrite(t *testing.T) {
	// Arrange
	ws, svc := setupServiceSyncWorkspace(t)
	ctx := context.Background()

	// Create artifact via template service
	artifact, err := svc.Create(ctx, ws, "Sync test task", "task", nil)
	require.NoError(t, err)

	// Record pre-update DB state
	beforeArtifact, err := db.GetItem(ctx, ws.DB, artifact.ID)
	require.NoError(t, err)
	beforeUpdatedAt := beforeArtifact.UpdatedAt

	// Small delay to ensure timestamp difference
	time.Sleep(10 * time.Millisecond)

	// Act — update sections
	_, err = svc.Update(ctx, ws, artifact.ID, map[string]string{
		"description": "Updated description content",
	})
	require.NoError(t, err)

	// Assert — DB should reflect the update
	afterArtifact, err := db.GetItem(ctx, ws.DB, artifact.ID)
	require.NoError(t, err)
	assert.True(t, afterArtifact.UpdatedAt.After(beforeUpdatedAt),
		"updated_at should be bumped after section update (before=%v, after=%v)",
		beforeUpdatedAt, afterArtifact.UpdatedAt)
}

// TASK-008.03: Update must set updated_at in frontmatter before writing file.
func TestUpdate_SetsUpdatedAtInFrontmatter(t *testing.T) {
	// Arrange
	ws, svc := setupServiceSyncWorkspace(t)
	ctx := context.Background()

	artifact, err := svc.Create(ctx, ws, "Timestamp test", "task", nil)
	require.NoError(t, err)
	createdAt := artifact.CreatedAt

	time.Sleep(10 * time.Millisecond)

	// Act
	updated, err := svc.Update(ctx, ws, artifact.ID, map[string]string{
		"description": "New description",
	})
	require.NoError(t, err)

	// Assert — returned artifact should have updated timestamp
	assert.True(t, updated.UpdatedAt.After(createdAt),
		"returned artifact updated_at should be after created_at")

	// Verify the file on disk has the updated timestamp
	filePath, err := core.FindArtifactPath(ctx, ws, artifact.ID)
	require.NoError(t, err)
	raw, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Contains(t, string(raw), "updated_at:",
		"frontmatter should contain updated_at field")
}
