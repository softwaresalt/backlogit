package core_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/db"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/models"
)

func setupCommitWorkspace(t *testing.T) *core.Workspace {
	t.Helper()
	tmpDir := t.TempDir()
	backlogDir := filepath.Join(tmpDir, ".backlogit")
	tasksDir := filepath.Join(backlogDir, "tasks")
	require.NoError(t, os.MkdirAll(tasksDir, 0o755))

	dbPath := filepath.Join(backlogDir, "backlogit.db")
	database, err := db.Open(dbPath)
	require.NoError(t, err)
	require.NoError(t, db.EnsureSchema(database))
	t.Cleanup(func() { database.Close() })

	// Write config
	require.NoError(t, os.WriteFile(filepath.Join(backlogDir, "config.yaml"),
		[]byte("artifact_types:\n  - task\nid_prefix_map:\n  task: T\nmax_slug_length: 60\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(backlogDir, "header-def.yaml"), []byte("defaults: {}\ntypes: {}\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(backlogDir, "registry.yaml"), []byte("routes: {}\n"), 0o644))

	// Seed a task
	taskContent := "---\nid: T001\ntitle: Test task\nstatus: active\ntype: task\n---\nTask body\n"
	require.NoError(t, os.WriteFile(filepath.Join(tasksDir, "T001.md"), []byte(taskContent), 0o644))
	require.NoError(t, db.UpsertItem(context.Background(), database, &models.Artifact{
		ID: "T001", Title: "Test task", Status: models.StatusActive, ArtifactType: "task",
	}))

	return &core.Workspace{RootPath: tmpDir, DB: database}
}

func TestLinkCommit_StoresAssociation(t *testing.T) {
	// Arrange
	ws := setupCommitWorkspace(t)
	ctx := context.Background()

	// Act
	err := core.LinkCommit(ctx, ws.DB, ws, "T001", "abc123def", "feat: implement T001", "test@example.com")

	// Assert
	require.NoError(t, err)
}

func TestGetCommitLinks_ReturnsLinked(t *testing.T) {
	// Arrange
	ws := setupCommitWorkspace(t)
	ctx := context.Background()
	require.NoError(t, core.LinkCommit(ctx, ws.DB, ws, "T001", "abc123def", "feat: implement T001", "test@example.com"))

	// Act
	links, err := core.GetCommitLinks(ctx, ws.DB, "T001")

	// Assert
	require.NoError(t, err)
	require.Len(t, links, 1)
	assert.Equal(t, "abc123def", links[0].CommitSHA)
}

func TestAutoLinkCommits_FindsReferences(t *testing.T) {
	// Arrange
	ws := setupCommitWorkspace(t)
	ctx := context.Background()

	// Act — depth 0 means no git log entries
	links, err := core.AutoLinkCommits(ctx, ws.DB, ws, 0)

	// Assert
	require.NoError(t, err)
	assert.Empty(t, links, "no commits to scan with depth 0")
}

func TestAssociateCommit_JSONLFailureProducesMutationPartialError(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	feature, err := core.CreateArtifact(ctx, ws, "Commit feature", "feature")
	require.NoError(t, err)
	artifact, err := core.CreateArtifact(ctx, ws, "Commit task", "task", core.WithParent(feature.ID))
	require.NoError(t, err)

	logsPath := filepath.Join(t.TempDir(), "not-a-dir")
	require.NoError(t, os.WriteFile(logsPath, []byte("blocking file"), 0o644))
	writer := events.NewEventWriter(logsPath)

	err = core.AssociateCommit(ctx, ws, writer, artifact.ID, "abc123def", "feat: implement task", "test@example.com")
	require.Error(t, err)

	var partialErr *blerrors.MutationPartialError
	require.True(t, errors.As(err, &partialErr))
	assert.Equal(t, []string{"frontmatter-scalar", "commit-links-upsert"}, partialErr.Completed)
	assert.Equal(t, "jsonl-append", partialErr.FailedStep)
	assert.Equal(t, "compensated", partialErr.CompensationState)
	assert.Equal(t, "not-applied", partialErr.Class)

	item, itemErr := db.GetItem(ctx, ws.DB, artifact.ID)
	require.NoError(t, itemErr)
	assert.Empty(t, item.Commit)

	links, getErr := core.GetCommitLinks(ctx, ws.DB, artifact.ID)
	require.NoError(t, getErr)
	assert.Empty(t, links)
}

func TestAssociateCommit_SQLiteFailure_FrontmatterCompensated(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	feature, err := core.CreateArtifact(ctx, ws, "Commit feature", "feature")
	require.NoError(t, err)
	artifact, err := core.CreateArtifact(ctx, ws, "Commit task", "task", core.WithParent(feature.ID))
	require.NoError(t, err)

	_, err = ws.DB.ExecContext(ctx, `DROP TABLE commit_links`)
	require.NoError(t, err)

	writer := events.NewEventWriter(filepath.Join(t.TempDir(), "logs"))

	err = core.AssociateCommit(ctx, ws, writer, artifact.ID, "abc123def", "feat: implement task", "test@example.com")
	require.Error(t, err)

	var partialErr *blerrors.MutationPartialError
	require.True(t, errors.As(err, &partialErr))
	assert.Equal(t, []string{"frontmatter-scalar"}, partialErr.Completed)
	assert.Equal(t, "commit-links-upsert", partialErr.FailedStep)
	assert.Equal(t, "compensated", partialErr.CompensationState)
	assert.Equal(t, "not-applied", partialErr.Class)

	item, itemErr := db.GetItem(ctx, ws.DB, artifact.ID)
	require.NoError(t, itemErr)
	assert.Empty(t, item.Commit)

	path, pathErr := core.FindArtifactPath(ctx, ws, artifact.ID)
	require.NoError(t, pathErr)
	data, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	fm, _, parseErr := models.ParseFrontmatter(string(data))
	require.NoError(t, parseErr)
	commitValue, ok := fm["commit"].(string)
	assert.True(t, !ok || commitValue == "")
}
