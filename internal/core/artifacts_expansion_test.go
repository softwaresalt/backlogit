package core_test

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/config"
	"github.com/backlogit/backlogit/internal/core"
	"github.com/backlogit/backlogit/internal/db"
)

// TASK-002.01.05: Update core CRUD with new fields and ID immutability.

func setupTestWorkspace(t *testing.T) *core.Workspace {
	t.Helper()
	root := t.TempDir()
	backlogitDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogitDir, 0o755))
	require.NoError(t, config.WriteDefaults(backlogitDir))

	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)
	t.Cleanup(func() { ws.Close() })
	return ws
}

func setupTestWorkspaceWithBugLevel(t *testing.T, level int) *core.Workspace {
	t.Helper()

	root := t.TempDir()
	backlogitDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogitDir, 0o755))
	require.NoError(t, config.WriteDefaults(backlogitDir))

	configPath := filepath.Join(backlogitDir, "config.yaml")
	configContent, err := os.ReadFile(configPath)
	require.NoError(t, err)
	updated := strings.Replace(string(configContent), "bug_level: 3", "bug_level: "+strconv.Itoa(level), 1)
	require.NoError(t, os.WriteFile(configPath, []byte(updated), 0o644))

	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)
	t.Cleanup(func() { ws.Close() })
	return ws
}

func TestCreateArtifact_WithNewOptions(t *testing.T) {
	// Arrange
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	// Act
	feat, err := core.CreateArtifact(ctx, ws, "Test feature", "feature")
	require.NoError(t, err)
	artifact, err := core.CreateArtifact(ctx, ws, "Test task", "task",
		core.WithParent(feat.ID),
		core.WithAssignedTo("alice"),
		core.WithOwner("bob"),
		core.WithLabels([]string{"backend", "urgent"}),
		core.WithDependencies([]string{"T002"}),
		core.WithReferences([]string{"docs/spec.md"}),
		core.WithCommit("abc123"),
	)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "alice", artifact.AssignedTo)
	assert.Equal(t, "bob", artifact.Owner)
	assert.Equal(t, []string{"backend", "urgent"}, artifact.Labels)
	assert.Equal(t, []string{"T002"}, artifact.Dependencies)
	assert.Equal(t, []string{"docs/spec.md"}, artifact.References)
	assert.Equal(t, "abc123", artifact.Commit)
}

func TestUpdateArtifact_NewFields(t *testing.T) {
	// Arrange
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	feat, err := core.CreateArtifact(ctx, ws, "Update feature", "feature")
	require.NoError(t, err)
	artifact, err := core.CreateArtifact(ctx, ws, "Update test", "task", core.WithParent(feat.ID))
	require.NoError(t, err)

	// Act — update new fields
	updated, err := core.UpdateArtifact(ctx, ws, artifact.ID, map[string]any{
		"assigned_to": "charlie",
		"owner":       "dave",
		"labels":      []string{"updated-label"},
		"commit":      "newcommit",
	})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "charlie", updated.AssignedTo)
	assert.Equal(t, "dave", updated.Owner)
	assert.Equal(t, []string{"updated-label"}, updated.Labels)
	assert.Equal(t, "newcommit", updated.Commit)
}

func TestUpdateArtifact_IDImmutability(t *testing.T) {
	// Arrange
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	feat, err := core.CreateArtifact(ctx, ws, "Immutable feature", "feature")
	require.NoError(t, err)
	artifact, err := core.CreateArtifact(ctx, ws, "Immutable ID test", "task", core.WithParent(feat.ID))
	require.NoError(t, err)

	// Act — attempt to change ID
	_, err = core.UpdateArtifact(ctx, ws, artifact.ID, map[string]any{
		"id": "TAMPERED-ID",
	})

	// Assert — should be rejected with descriptive error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "immutable")
}

func TestCreateArtifact_WithoutNewOptions(t *testing.T) {
	// Arrange
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	// Act — no new options; must now provide a parent for a level-2 task
	feat, err := core.CreateArtifact(ctx, ws, "Plain feature", "feature")
	require.NoError(t, err)
	artifact, err := core.CreateArtifact(ctx, ws, "Plain task", "task", core.WithParent(feat.ID))

	// Assert
	require.NoError(t, err)
	assert.Empty(t, artifact.AssignedTo)
	assert.Empty(t, artifact.Owner)
	assert.Nil(t, artifact.Labels)
}

func TestUpdateArtifact_RejectsParentIDChange(t *testing.T) {
	// Arrange
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	parent, err := core.CreateArtifact(ctx, ws, "Parent feature", "feature")
	require.NoError(t, err)
	require.NoError(t, db.UpsertItem(ctx, ws.DB, parent))

	artifact, err := core.CreateArtifact(ctx, ws, "Parent test", "task",
		core.WithParent(parent.ID),
	)
	require.NoError(t, err)

	// Act — DB sync to make artifact findable
	_, err = db.Rehydrate(ctx, core.WorkspaceStorageRoot(ws.RootPath), ws.DB)
	require.NoError(t, err)

	// Act — attempt to change parent_id via update (not immutable but tested for completeness)
	updated, err := core.UpdateArtifact(ctx, ws, artifact.ID, map[string]any{
		"status": "active",
	})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "active", string(updated.Status))
}

func TestCreateArtifact_WritesUnderBacklogitStorage(t *testing.T) {
	// Arrange
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	// Act
	feat, err := core.CreateArtifact(ctx, ws, "Storage feature", "feature")
	require.NoError(t, err)
	artifact, err := core.CreateArtifact(ctx, ws, "Stored under backlogit", "task", core.WithParent(feat.ID))
	require.NoError(t, err)

	filePath, err := core.FindArtifactPath(ctx, ws, artifact.ID)
	require.NoError(t, err)

	// Assert
	assert.FileExists(t, filePath)
	assert.Contains(t, filepath.ToSlash(filePath), "/.backlogit/")
}

func TestCreateArtifact_UsesHierarchicalTaskIDsUnderFeatureParent(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	featureOne, err := core.CreateArtifact(ctx, ws, "Feature one", "feature")
	require.NoError(t, err)
	require.NoError(t, db.UpsertItem(ctx, ws.DB, featureOne))

	firstTask, err := core.CreateArtifact(ctx, ws, "First child task", "task", core.WithParent(featureOne.ID))
	require.NoError(t, err)
	require.NoError(t, db.UpsertItem(ctx, ws.DB, firstTask))

	secondTask, err := core.CreateArtifact(ctx, ws, "Second child task", "task", core.WithParent(featureOne.ID))
	require.NoError(t, err)
	require.NoError(t, db.UpsertItem(ctx, ws.DB, secondTask))

	featureTwo, err := core.CreateArtifact(ctx, ws, "Feature two", "feature")
	require.NoError(t, err)
	require.NoError(t, db.UpsertItem(ctx, ws.DB, featureTwo))

	otherFeatureTask, err := core.CreateArtifact(ctx, ws, "Other feature child task", "task", core.WithParent(featureTwo.ID))
	require.NoError(t, err)

	assert.Equal(t, "001.001-T", firstTask.ID)
	assert.Equal(t, "001.002-T", secondTask.ID)
	assert.Equal(t, "002.001-T", otherFeatureTask.ID)
}

func TestCreateArtifact_UsesHierarchicalSubtaskIDsAndFilenamesUnderTaskParent(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	feature, err := core.CreateArtifact(ctx, ws, "Feature root", "feature")
	require.NoError(t, err)
	require.NoError(t, db.UpsertItem(ctx, ws.DB, feature))

	task, err := core.CreateArtifact(ctx, ws, "Task parent", "task", core.WithParent(feature.ID))
	require.NoError(t, err)
	require.NoError(t, db.UpsertItem(ctx, ws.DB, task))

	firstSubtask, err := core.CreateArtifact(ctx, ws, "First child subtask", "subtask", core.WithParent(task.ID))
	require.NoError(t, err)
	require.NoError(t, db.UpsertItem(ctx, ws.DB, firstSubtask))

	secondSubtask, err := core.CreateArtifact(ctx, ws, "Second child subtask", "subtask", core.WithParent(task.ID))
	require.NoError(t, err)

	assert.Equal(t, "001.001.001-ST", firstSubtask.ID)
	assert.Equal(t, "001.001.002-ST", secondSubtask.ID)

	filePath, err := core.FindArtifactPath(ctx, ws, firstSubtask.ID)
	require.NoError(t, err)
	assert.Equal(
		t,
		filepath.Join(core.WorkspaceStorageRoot(ws.RootPath), "queue", firstSubtask.ID+".md"),
		filePath,
	)
}

func TestCreateArtifact_ReviewRequiresFeatureParent(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	_, err := core.CreateArtifact(ctx, ws, "Branch review", "review")

	require.Error(t, err)
	assert.Contains(t, err.Error(), `requires parent_id`)
}

func TestCreateArtifact_ReviewRejectsDisallowedParentType(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	feat, err := core.CreateArtifact(ctx, ws, "Disallowed parent feature", "feature")
	require.NoError(t, err)
	task, err := core.CreateArtifact(ctx, ws, "Parent task", "task", core.WithParent(feat.ID))
	require.NoError(t, err)

	_, err = core.CreateArtifact(ctx, ws, "Task review", "review", core.WithParent(task.ID))

	require.Error(t, err)
	assert.Contains(t, err.Error(), `artifact type "review" is not allowed under parent type "task"`)
}

func TestCreateArtifact_ReviewAllowsFeatureParent(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	feature, err := core.CreateArtifact(ctx, ws, "Release pipeline fix", "feature")
	require.NoError(t, err)

	review, err := core.CreateArtifact(ctx, ws, "Branch review", "review", core.WithParent(feature.ID))

	require.NoError(t, err)
	assert.Equal(t, feature.ID, review.ParentID)
	assert.Equal(t, "review", review.ArtifactType)
}

func TestCreateArtifact_BugRejectsFeatureParentWhenBugLevelThree(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	feature, err := core.CreateArtifact(ctx, ws, "Feature parent", "feature")
	require.NoError(t, err)

	_, err = core.CreateArtifact(ctx, ws, "Feature bug", "bug", core.WithParent(feature.ID))

	require.Error(t, err)
	assert.Contains(t, err.Error(), `artifact type "bug" is not allowed under parent type "feature"`)
}

func TestCreateArtifact_BugAllowsTaskParentWhenBugLevelThree(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	feature, err := core.CreateArtifact(ctx, ws, "Feature parent", "feature")
	require.NoError(t, err)
	require.NoError(t, db.UpsertItem(ctx, ws.DB, feature))

	task, err := core.CreateArtifact(ctx, ws, "Task parent", "task", core.WithParent(feature.ID))
	require.NoError(t, err)

	bug, err := core.CreateArtifact(ctx, ws, "Task bug", "bug", core.WithParent(task.ID))

	require.NoError(t, err)
	assert.Equal(t, task.ID, bug.ParentID)
	assert.Equal(t, "001.001.001-B", bug.ID)
}

func TestCreateArtifact_BugAllowsFeatureParentWhenBugLevelTwo(t *testing.T) {
	ws := setupTestWorkspaceWithBugLevel(t, 2)
	ctx := context.Background()

	feature, err := core.CreateArtifact(ctx, ws, "Feature parent", "feature")
	require.NoError(t, err)

	bug, err := core.CreateArtifact(ctx, ws, "Feature bug", "bug", core.WithParent(feature.ID))

	require.NoError(t, err)
	assert.Equal(t, feature.ID, bug.ParentID)
	assert.Equal(t, "001.001-B", bug.ID)
	level, levelErr := core.LevelForType(ws.Config.QueueLayout, "bug")
	require.NoError(t, levelErr)
	assert.Equal(t, 2, level)
}

func TestCreateArtifact_BugRejectsTaskParentWhenBugLevelTwo(t *testing.T) {
	ws := setupTestWorkspaceWithBugLevel(t, 2)
	ctx := context.Background()

	feature, err := core.CreateArtifact(ctx, ws, "Feature parent", "feature")
	require.NoError(t, err)
	require.NoError(t, db.UpsertItem(ctx, ws.DB, feature))

	task, err := core.CreateArtifact(ctx, ws, "Task parent", "task", core.WithParent(feature.ID))
	require.NoError(t, err)

	_, err = core.CreateArtifact(ctx, ws, "Task bug", "bug", core.WithParent(task.ID))

	require.Error(t, err)
	assert.Contains(t, err.Error(), `artifact type "bug" is not allowed under parent type "task"`)
}
