package core_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/models"
)

func setupQueueWorkspace(t *testing.T) *core.Workspace {
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

	require.NoError(t, os.WriteFile(filepath.Join(backlogDir, "config.yaml"),
		[]byte("artifact_types:\n  - task\n  - bug\nid_prefix_map:\n  task: T\n  bug: B\nmax_slug_length: 60\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(backlogDir, "header-def.yaml"), []byte("defaults: {}\ntypes: {}\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(backlogDir, "registry.yaml"), []byte("routes: {}\n"), 0o644))

	ctx := context.Background()
	items := []*models.Artifact{
		{ID: "T001", Title: "Task A", Status: models.StatusQueued, ArtifactType: "task", Priority: "high"},
		{ID: "T002", Title: "Task B", Status: models.StatusActive, ArtifactType: "task", Priority: "medium"},
		{ID: "T003", Title: "Task C", Status: models.StatusDone, ArtifactType: "task", Priority: "low"},
		{ID: "B001", Title: "Bug A", Status: models.StatusQueued, ArtifactType: "bug", Priority: "high"},
	}
	for _, item := range items {
		require.NoError(t, db.UpsertItem(ctx, database, item))
	}

	return &core.Workspace{RootPath: tmpDir, DB: database}
}

func TestQueryQueue_FilterByStatus(t *testing.T) {
	// Arrange
	ws := setupQueueWorkspace(t)
	ctx := context.Background()
	filter := &core.QueueFilter{Statuses: []string{"queued"}}

	// Act
	view, err := core.QueryQueue(ctx, ws.DB, filter)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 2, view.TotalCount, "should return T001 and B001")
}

func TestQueryQueue_GroupByType(t *testing.T) {
	// Arrange
	ws := setupQueueWorkspace(t)
	ctx := context.Background()
	filter := &core.QueueFilter{GroupBy: "type"}

	// Act
	view, err := core.QueryQueue(ctx, ws.DB, filter)

	// Assert
	require.NoError(t, err)
	assert.NotEmpty(t, view.Groups, "should have groups when GroupBy is set")
}

func TestQueryQueue_IgnoresBlankTypeAndStatusFilters(t *testing.T) {
	// Arrange
	ws := setupQueueWorkspace(t)
	ctx := context.Background()
	filter := &core.QueueFilter{
		Types:    []string{""},
		Statuses: []string{""},
	}

	// Act
	view, err := core.QueryQueue(ctx, ws.DB, filter)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 4, view.TotalCount, "blank filters should behave like no filter")
}

func TestMoveInQueue_ReordersAndSurvivesRehydrate(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	low, err := core.CreateArtifact(ctx, ws, "Low priority task", "feature", core.WithPriority("low"))
	require.NoError(t, err)
	high, err := core.CreateArtifact(ctx, ws, "High priority task", "feature", core.WithPriority("high"))
	require.NoError(t, err)

	filter := &core.QueueFilter{Statuses: []string{"queued"}, SortBy: "priority"}

	initial, err := core.QueryQueue(ctx, ws.DB, filter)
	require.NoError(t, err)
	require.Len(t, initial.Items, 2)
	assert.Equal(t, high.ID, initial.Items[0].ID)
	assert.Equal(t, low.ID, initial.Items[1].ID)

	err = core.MoveInQueue(ctx, ws, low.ID, 1, filter)
	require.NoError(t, err)

	reordered, err := core.QueryQueue(ctx, ws.DB, filter)
	require.NoError(t, err)
	require.Len(t, reordered.Items, 2)
	assert.Equal(t, low.ID, reordered.Items[0].ID)
	assert.Equal(t, high.ID, reordered.Items[1].ID)

	_, err = db.Rehydrate(ctx, core.WorkspaceStorageRoot(ws.RootPath), ws.DB)
	require.NoError(t, err)

	rehydrated, err := core.QueryQueue(ctx, ws.DB, filter)
	require.NoError(t, err)
	require.Len(t, rehydrated.Items, 2)
	assert.Equal(t, low.ID, rehydrated.Items[0].ID)
	assert.Equal(t, high.ID, rehydrated.Items[1].ID)
}

func TestMoveInQueue_RejectsInvalidPosition(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	artifact, err := core.CreateArtifact(ctx, ws, "Queue item", "feature")
	require.NoError(t, err)

	err = core.MoveInQueue(ctx, ws, artifact.ID, 0, &core.QueueFilter{Statuses: []string{"queued"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "position must be >=")
}

func TestMoveInQueue_RollsBackPersistedPositionsOnFailure(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	high, err := core.CreateArtifact(ctx, ws, "High priority task", "feature", core.WithPriority("high"))
	require.NoError(t, err)
	medium, err := core.CreateArtifact(ctx, ws, "Medium priority task", "feature", core.WithPriority("medium"))
	require.NoError(t, err)
	low, err := core.CreateArtifact(ctx, ws, "Low priority task", "feature", core.WithPriority("low"))
	require.NoError(t, err)

	filter := &core.QueueFilter{Statuses: []string{"queued"}, SortBy: "priority"}

	initial, err := core.QueryQueue(ctx, ws.DB, filter)
	require.NoError(t, err)
	require.Len(t, initial.Items, 3)
	assert.Equal(t, []string{high.ID, medium.ID, low.ID}, []string{
		initial.Items[0].ID,
		initial.Items[1].ID,
		initial.Items[2].ID,
	})

	failingPath, err := core.FindArtifactPath(ctx, ws, high.ID)
	require.NoError(t, err)
	originalHigh, err := os.ReadFile(failingPath)
	require.NoError(t, err)
	require.NoError(t, os.Remove(failingPath))

	err = core.MoveInQueue(ctx, ws, low.ID, 1, filter)
	require.Error(t, err)
	// The move reloads each reordered item from Markdown before persisting its
	// new position (so archive provenance and other raw frontmatter survive the
	// rewrite). Removing high's file surfaces as a reload failure that names the
	// failing item and still triggers rollback of already-persisted items.
	assert.Contains(t, err.Error(), high.ID)

	lowPath, err := core.FindArtifactPath(ctx, ws, low.ID)
	require.NoError(t, err)
	lowData, err := os.ReadFile(lowPath)
	require.NoError(t, err)
	assert.NotContains(t, string(lowData), "queue_position", "rollback should restore the original file contents")

	lowItem, err := db.GetItem(ctx, ws.DB, low.ID)
	require.NoError(t, err)
	if lowItem.CustomFields != nil {
		_, hasQueuePosition := lowItem.CustomFields["queue_position"]
		assert.False(t, hasQueuePosition, "rollback should remove the queued position from the DB record")
	}

	require.NoError(t, os.WriteFile(failingPath, originalHigh, 0o644))
	_, err = db.Rehydrate(ctx, core.WorkspaceStorageRoot(ws.RootPath), ws.DB)
	require.NoError(t, err)

	rolledBack, err := core.QueryQueue(ctx, ws.DB, filter)
	require.NoError(t, err)
	require.Len(t, rolledBack.Items, 3)
	assert.Equal(t, []string{high.ID, medium.ID, low.ID}, []string{
		rolledBack.Items[0].ID,
		rolledBack.Items[1].ID,
		rolledBack.Items[2].ID,
	})
}

func TestBulkUpdateStatus(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()
	feature, err := core.CreateArtifact(ctx, ws, "Bulk update feature", "feature")
	require.NoError(t, err)
	task, err := core.CreateArtifact(ctx, ws, "Bulk update task", "task", core.WithParent(feature.ID))
	require.NoError(t, err)

	// Act
	result, err := core.BulkUpdateStatus(ctx, ws.DB, ws, []string{task.ID}, "active")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 1, result.Succeeded, "should update the queued item")
	assert.Empty(t, result.Failed)

	updated, err := db.GetItem(ctx, ws.DB, task.ID)
	require.NoError(t, err)
	assert.Equal(t, models.StatusActive, updated.Status)
}

func TestBulkUpdateStatus_MarkdownFirst_WritesBeforeDB(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	feat, err := core.CreateArtifact(ctx, ws, "Feature to archive", "feature")
	require.NoError(t, err)

	result, err := core.BulkUpdateStatus(ctx, ws.DB, ws, []string{feat.ID}, "done")

	require.NoError(t, err)
	assert.Equal(t, 1, result.Succeeded)
	assert.Empty(t, result.Failed)

	// Markdown file must reflect the new status.
	filePath, pathErr := core.FindArtifactPath(ctx, ws, feat.ID)
	require.NoError(t, pathErr)
	data, readErr := os.ReadFile(filePath)
	require.NoError(t, readErr)
	assert.Contains(t, string(data), "status: done", "Markdown must be updated")

	// DB index must also reflect the new status.
	item, getErr := db.GetItem(ctx, ws.DB, feat.ID)
	require.NoError(t, getErr)
	assert.Equal(t, models.StatusDone, item.Status, "DB index must match Markdown after bulk update")
}

func TestBulkUpdateStatus_PartialFailure_ReportsAccurately(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	feat, err := core.CreateArtifact(ctx, ws, "Parent feature", "feature")
	require.NoError(t, err)
	task, err := core.CreateArtifact(ctx, ws, "Orphaned task", "task", core.WithParent(feat.ID))
	require.NoError(t, err)

	// Remove the task's Markdown file to simulate a missing artifact on disk.
	filePath, pathErr := core.FindArtifactPath(ctx, ws, task.ID)
	require.NoError(t, pathErr)
	require.NoError(t, os.Remove(filePath))

	result, err := core.BulkUpdateStatus(ctx, ws.DB, ws, []string{feat.ID, task.ID}, "done")

	require.NoError(t, err, "partial failure must not propagate as a function error")
	assert.Equal(t, 1, result.Succeeded, "feature without missing file should succeed")
	assert.Len(t, result.Failed, 1, "task with missing file should be reported as failed")
	assert.Contains(t, result.Failed, task.ID)
}

// --- 134.003-T: Failing queue suppression tests for shipment blocking (U3 red harness) ---

// TestShipmentQueueSuppression_DependentSuppressedUntilPrerequisiteTerminal verifies
// that a dependent shipment is suppressed in the queued-shipment view until the
// prerequisite shipment reaches a terminal status. The direction is:
//
//	"prereq must ship before dependent" = dependent depends_on prereq.
//
// Red harness: fails until U4 implements AddShipmentBlock (stub errors, so no edge
// is created and suppression cannot take effect).
func TestShipmentQueueSuppression_DependentSuppressedUntilPrerequisiteTerminal(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	prereq, err := core.CreateShipment(ctx, ws, "Prereq shipment", nil)
	require.NoError(t, err)
	dependent, err := core.CreateShipment(ctx, ws, "Dependent shipment", nil)
	require.NoError(t, err)

	// Add blocking edge: dependent depends_on prereq.
	require.NoError(t, core.AddShipmentBlock(ctx, ws, dependent.ID, prereq.ID),
		"AddShipmentBlock must succeed for two queued shipments")

	// Query the queued-shipment view: dependent must be suppressed while prereq
	// is still in a non-terminal status.
	filter := &core.QueueFilter{
		Types:    []string{"shipment"},
		Statuses: []string{"queued"},
	}
	view, err := core.QueryQueue(ctx, ws.DB, filter)
	require.NoError(t, err)

	visibleIDs := make(map[string]bool)
	for _, item := range view.Items {
		visibleIDs[item.ID] = true
	}

	assert.True(t, visibleIDs[prereq.ID],
		"prerequisite shipment must be visible in the queued-shipment view")
	assert.False(t, visibleIDs[dependent.ID],
		"dependent shipment must be suppressed from the queued-shipment view "+
			"while its prerequisite is in a non-terminal status (pinned direction: "+
			"dependent depends_on prereq)")
}

// TestShipmentQueueSuppression_PrerequisiteNotSuppressed verifies that the
// prerequisite shipment itself is NEVER suppressed — only the dependent is.
// This pins the blocking direction: dependent→prerequisite (not vice versa).
//
// Red harness: fails until U4 implements AddShipmentBlock.
func TestShipmentQueueSuppression_PrerequisiteNotSuppressed(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	prereq, err := core.CreateShipment(ctx, ws, "Prereq (must not be suppressed)", nil)
	require.NoError(t, err)
	dependent, err := core.CreateShipment(ctx, ws, "Dependent (must be suppressed)", nil)
	require.NoError(t, err)

	require.NoError(t, core.AddShipmentBlock(ctx, ws, dependent.ID, prereq.ID))

	filter := &core.QueueFilter{
		Types:    []string{"shipment"},
		Statuses: []string{"queued"},
	}
	view, err := core.QueryQueue(ctx, ws.DB, filter)
	require.NoError(t, err)

	visibleIDs := make(map[string]bool)
	for _, item := range view.Items {
		visibleIDs[item.ID] = true
	}

	assert.True(t, visibleIDs[prereq.ID],
		"prerequisite must remain visible — only the dependent is suppressed")
}

// TestShipmentQueueSuppression_DependentBecomesVisibleWhenPrereqTerminal verifies
// that the dependent shipment becomes visible once the prerequisite moves to a
// terminal status (shipped).
//
// Red harness: fails until U4 implements AddShipmentBlock.
func TestShipmentQueueSuppression_DependentBecomesVisibleWhenPrereqTerminal(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	prereq, err := core.CreateShipment(ctx, ws, "Prereq shipment (will ship)", nil)
	require.NoError(t, err)
	dependent, err := core.CreateShipment(ctx, ws, "Dependent shipment (will unblock)", nil)
	require.NoError(t, err)

	require.NoError(t, core.AddShipmentBlock(ctx, ws, dependent.ID, prereq.ID))

	// Move prerequisite to "shipped" (terminal status).
	require.NoError(t, core.MoveShipmentStatus(ctx, ws, prereq.ID, core.ShipmentShipped))

	// Now the dependent must be visible in the queued view.
	filter := &core.QueueFilter{
		Types:    []string{"shipment"},
		Statuses: []string{"queued"},
	}
	view, err := core.QueryQueue(ctx, ws.DB, filter)
	require.NoError(t, err)

	visibleIDs := make(map[string]bool)
	for _, item := range view.Items {
		visibleIDs[item.ID] = true
	}

	assert.True(t, visibleIDs[dependent.ID],
		"dependent shipment must become visible once its prerequisite reaches a terminal status")
}
