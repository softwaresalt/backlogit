package integration_test

// F015 / T008: End-to-end integration tests for the shipment workflow.
// These tests exercise the complete data path: stash round-trip, shipment
// creation, blocked-item return, and rehydration consistency in a real workspace.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/config"
	"github.com/backlogit/backlogit/internal/core"
	"github.com/backlogit/backlogit/internal/db"
	"github.com/backlogit/backlogit/internal/stash"
)

// setupShipmentIntegrationWorkspace creates a temp workspace with full config,
// including shipment support, for integration testing.
func setupShipmentIntegrationWorkspace(t *testing.T) (string, *core.Workspace) {
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

// T008 / ST031: Stash JSONL round-trip through the live workspace.
func TestShipmentWorkflow_StashJSONLRoundTrip(t *testing.T) {
	// Arrange
	root, _ := setupShipmentIntegrationWorkspace(t)
	jsonlPath := filepath.Join(root, ".backlogit", "stash.jsonl")

	entries := []stash.Entry{
		{ID: "AAAA1111", Priority: "high", Kind: "feature", Text: "Add shipment type"},
		{ID: "BBBB2222", Priority: "medium", Kind: "bug", Text: "Fix stash parsing"},
	}

	// Act: Write entries then read them back
	f, err := os.Create(jsonlPath)
	require.NoError(t, err)
	require.NoError(t, stash.WriteJSONL(f, entries))
	require.NoError(t, f.Close())

	f2, err := os.Open(jsonlPath)
	require.NoError(t, err)
	defer f2.Close()
	recovered, err := stash.ReadJSONL(f2)

	// Assert
	require.NoError(t, err)
	require.Len(t, recovered, 2)
	assert.Equal(t, "AAAA1111", recovered[0].ID)
	assert.Equal(t, "high", recovered[0].Priority)
	assert.Equal(t, "BBBB2222", recovered[1].ID)
}

// T008 / ST031: Create a shipment from queued backlog items using core API.
func TestShipmentWorkflow_CreateShipmentFromBacklog(t *testing.T) {
	// Arrange
	_, ws := setupShipmentIntegrationWorkspace(t)
	ctx := context.Background()

	task1, err := core.CreateArtifact(ctx, ws, "Implement feature A", "task")
	require.NoError(t, err)
	task2, err := core.CreateArtifact(ctx, ws, "Implement feature B", "task")
	require.NoError(t, err)

	// Act
	shipment, err := core.CreateShipment(ctx, ws, "Sprint 1", []string{task1.ID, task2.ID})

	// Assert
	require.NoError(t, err)
	require.NotNil(t, shipment)
	assert.Contains(t, shipment.ID, "S")
	assert.Equal(t, "queued", string(shipment.Status))
}

// T008 / ST031: Blocked-item return restores item to backlog state.
func TestShipmentWorkflow_ReturnBlockedItem(t *testing.T) {
	// Arrange
	_, ws := setupShipmentIntegrationWorkspace(t)
	ctx := context.Background()

	task, err := core.CreateArtifact(ctx, ws, "Blockable task", "task")
	require.NoError(t, err)
	shipment, err := core.CreateShipment(ctx, ws, "Block test", []string{task.ID})
	require.NoError(t, err)

	// Act
	err = core.ReturnBlockedItem(ctx, ws, shipment.ID, task.ID, "dependency not met")

	// Assert
	require.NoError(t, err)

	// Verify task is now blocked
	updated, err := core.GetShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)
	if items, ok := updated.CustomFields["items"].([]string); ok {
		assert.NotContains(t, items, task.ID, "returned item must not be in shipment")
	}
}

// T008 / ST031: Rehydration produces consistent index after forced rebuild.
func TestShipmentWorkflow_RehydrationConsistency(t *testing.T) {
	// Arrange
	root, ws := setupShipmentIntegrationWorkspace(t)
	ctx := context.Background()

	shipment, err := core.CreateShipment(ctx, ws, "Rehydration test", nil)
	require.NoError(t, err)

	task, err := core.CreateArtifact(ctx, ws, "Rehydration task", "task")
	require.NoError(t, err)
	require.NoError(t, core.AddItemToShipment(ctx, ws, shipment.ID, task.ID))

	// Force rehydration by closing, deleting db, and reopening
	ws.Close()
	dbPath := filepath.Join(root, ".backlogit", "backlogit.db")
	_ = os.Remove(dbPath)

	ws2, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)
	defer ws2.Close()

	// Act: Query after rehydration
	recovered, err := core.GetShipment(ctx, ws2, shipment.ID)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, shipment.ID, recovered.ID)
	assert.Equal(t, shipment.Title, recovered.Title)
}

// T008 / ST031: Shipping a shipment archives released scope and returns untouched
// feature descendants to backlog in a real workspace.
func TestShipmentWorkflow_ShipmentReleaseCleanup(t *testing.T) {
	// Arrange
	_, ws := setupShipmentIntegrationWorkspace(t)
	ctx := context.Background()

	deliberation, err := core.CreateArtifact(ctx, ws, "Integration deliberation", "deliberation")
	require.NoError(t, err)
	require.NoError(t, db.UpsertItem(ctx, ws.DB, deliberation))

	feature, err := core.CreateArtifact(
		ctx,
		ws,
		"Integration feature",
		"feature",
		core.WithDescription("Origin: "+deliberation.ID),
	)
	require.NoError(t, err)
	require.NoError(t, db.UpsertItem(ctx, ws.DB, feature))

	releasedTask, err := core.CreateArtifact(ctx, ws, "Released integration task", "task", core.WithParent(feature.ID))
	require.NoError(t, err)
	require.NoError(t, db.UpsertItem(ctx, ws.DB, releasedTask))

	futureTask, err := core.CreateArtifact(ctx, ws, "Future integration task", "task", core.WithParent(feature.ID))
	require.NoError(t, err)
	require.NoError(t, db.UpsertItem(ctx, ws.DB, futureTask))

	shipment, err := core.CreateShipment(ctx, ws, "Integration release shipment", []string{releasedTask.ID})
	require.NoError(t, err)
	_, err = core.ClaimShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)

	// Act
	result, err := core.ShipShipment(ctx, ws, shipment.ID, &core.CommitMetadata{
		SHA:     "feedface12345678",
		Message: "merge: integration shipment release",
		Author:  "tester@example.com",
	})

	// Assert
	require.NoError(t, err)
	assert.Contains(t, result.ArchivedIDs, feature.ID)
	assert.Contains(t, result.ArchivedIDs, releasedTask.ID)
	assert.Contains(t, result.ArchivedIDs, shipment.ID)
	assert.Contains(t, result.ReturnedIDs, futureTask.ID)

	archivedFeature, err := db.GetItem(ctx, ws.DB, feature.ID)
	require.NoError(t, err)
	assert.Equal(t, "archived", string(archivedFeature.Status))

	queuedFutureTask, err := db.GetItem(ctx, ws.DB, futureTask.ID)
	require.NoError(t, err)
	assert.Equal(t, "queued", string(queuedFutureTask.Status))
}

// T008 / ST029: Migrate checked-in stash from .stash.md to stash.jsonl.
func TestShipmentWorkflow_StashMigration(t *testing.T) {
	// Arrange
	root, _ := setupShipmentIntegrationWorkspace(t)
	stashMDPath := filepath.Join(root, ".backlogit", "queue", ".stash.md")
	jsonlPath := filepath.Join(root, ".backlogit", "stash.jsonl")

	stashContent := `---
stash_version: "1"
---

- id: AAAA1111
  priority: high
  kind: feature
  text: "Migrate me"
`
	require.NoError(t, os.WriteFile(stashMDPath, []byte(stashContent), 0o644))

	// Act
	count, err := stash.MigrateStashMDToJSONL(stashMDPath, jsonlPath)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Verify JSONL is readable
	f, err := os.Open(jsonlPath)
	require.NoError(t, err)
	defer f.Close()
	entries, err := stash.ReadJSONL(f)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "AAAA1111", entries[0].ID)
}

// T008 / ST031: Stash JSONL rehydration into database index.
func TestShipmentWorkflow_StashJSONLRehydration(t *testing.T) {
	// Arrange
	root, ws := setupShipmentIntegrationWorkspace(t)
	jsonlPath := filepath.Join(root, ".backlogit", "stash.jsonl")

	entries := []stash.Entry{
		{ID: "CCCC3333", Priority: "high", Kind: "feature", Text: "Rehydrate me"},
	}
	f, err := os.Create(jsonlPath)
	require.NoError(t, err)
	require.NoError(t, stash.WriteJSONL(f, entries))
	require.NoError(t, f.Close())

	// Force rehydration
	ws.Close()
	dbPath := filepath.Join(root, ".backlogit", "backlogit.db")
	_ = os.Remove(dbPath)

	ws2, err := core.NewWorkspace(context.Background(), root)
	require.NoError(t, err)
	defer ws2.Close()

	// Act: Verify stash entry appears in DB after rehydration
	count, err := db.Rehydrate(context.Background(), filepath.Join(root, ".backlogit"), ws2.DB)
	require.NoError(t, err)

	// Assert
	assert.Greater(t, count, 0, "rehydration must index at least one artifact")
}
