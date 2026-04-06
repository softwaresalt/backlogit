package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/config"
)

// setupShipmentWorkspace creates a temp workspace directory with minimal config for
// shipment testing. Returns the Workspace pointer.
func setupShipmentWorkspace(t *testing.T) *Workspace {
	t.Helper()
	root := t.TempDir()
	ws := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(ws, 0o755))
	require.NoError(t, config.WriteDefaults(ws))

	ctx := context.Background()
	workspace, err := NewWorkspace(ctx, root)
	require.NoError(t, err)
	t.Cleanup(func() { workspace.Close() })
	return workspace
}

// T002 / ST012: Create a shipment and verify it exists with queued status.
func TestCreateShipment_Success(t *testing.T) {
	// Arrange
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	// Act
	shipment, err := CreateShipment(ctx, ws, "Sprint 1 delivery", []string{"T001", "T002"})

	// Assert
	require.NoError(t, err)
	require.NotNil(t, shipment)
	assert.Equal(t, "shipment", shipment.ArtifactType)
	assert.Equal(t, "queued", string(shipment.Status))
	assert.Contains(t, shipment.ID, "S")
	assert.Equal(t, "Sprint 1 delivery", shipment.Title)
}

// T002 / ST012: Move shipment from queued to active.
func TestMoveShipmentStatus_QueuedToActive(t *testing.T) {
	// Arrange
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	shipment, err := CreateShipment(ctx, ws, "Test shipment", nil)
	require.NoError(t, err)

	// Act
	err = MoveShipmentStatus(ctx, ws, shipment.ID, ShipmentActive)

	// Assert
	require.NoError(t, err)
	updated, err := GetShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)
	assert.Equal(t, "active", string(updated.Status))
}

// T002 / ST012: Move shipment from active to shipped.
func TestMoveShipmentStatus_ActiveToShipped(t *testing.T) {
	// Arrange
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	shipment, err := CreateShipment(ctx, ws, "Deliver shipment", nil)
	require.NoError(t, err)
	require.NoError(t, MoveShipmentStatus(ctx, ws, shipment.ID, ShipmentActive))

	// Act
	err = MoveShipmentStatus(ctx, ws, shipment.ID, ShipmentShipped)

	// Assert
	require.NoError(t, err)
	updated, err := GetShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)
	assert.Equal(t, "shipped", string(updated.Status))
}

// T002 / ST012: Reject invalid status transition (queued -> shipped).
func TestMoveShipmentStatus_InvalidTransition(t *testing.T) {
	// Arrange
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	shipment, err := CreateShipment(ctx, ws, "Bad transition", nil)
	require.NoError(t, err)

	// Act
	err = MoveShipmentStatus(ctx, ws, shipment.ID, ShipmentShipped)

	// Assert
	require.Error(t, err, "queued -> shipped should be rejected")
}

// T002 / ST013: Add an item to a shipment.
func TestAddItemToShipment_Success(t *testing.T) {
	// Arrange
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	shipment, err := CreateShipment(ctx, ws, "With items", nil)
	require.NoError(t, err)

	// Create a task artifact for the item
	task, err := CreateArtifact(ctx, ws, "Test task", "task")
	require.NoError(t, err)

	// Act
	err = AddItemToShipment(ctx, ws, shipment.ID, task.ID)

	// Assert
	require.NoError(t, err)
	updated, err := GetShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)
	items, ok := updated.CustomFields["items"].([]string)
	require.True(t, ok, "shipment must have items in custom fields")
	assert.Contains(t, items, task.ID)
}

// T002 / ST013: Reject adding an item already in another shipment.
func TestAddItemToShipment_AlreadyAssigned(t *testing.T) {
	// Arrange
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	s1, err := CreateShipment(ctx, ws, "Shipment 1", nil)
	require.NoError(t, err)
	s2, err := CreateShipment(ctx, ws, "Shipment 2", nil)
	require.NoError(t, err)

	task, err := CreateArtifact(ctx, ws, "Contested task", "task")
	require.NoError(t, err)

	require.NoError(t, AddItemToShipment(ctx, ws, s1.ID, task.ID))

	// Act
	err = AddItemToShipment(ctx, ws, s2.ID, task.ID)

	// Assert
	require.Error(t, err, "item already in s1 must not be added to s2")
	_ = s2 // prevent unused
}

// T002 / ST013: Return a blocked item from shipment.
func TestReturnBlockedItem_Success(t *testing.T) {
	// Arrange
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	shipment, err := CreateShipment(ctx, ws, "Blocked test", nil)
	require.NoError(t, err)

	task, err := CreateArtifact(ctx, ws, "Blockable task", "task")
	require.NoError(t, err)
	require.NoError(t, AddItemToShipment(ctx, ws, shipment.ID, task.ID))

	// Act
	err = ReturnBlockedItem(ctx, ws, shipment.ID, task.ID, "dependency not ready")

	// Assert
	require.NoError(t, err)

	// Verify item is no longer in shipment
	updated, err := GetShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)
	if items, ok := updated.CustomFields["items"].([]string); ok {
		assert.NotContains(t, items, task.ID)
	}
}

// T002 / ST013: Reject returning an item not in the shipment.
func TestReturnBlockedItem_NotInShipment(t *testing.T) {
	// Arrange
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	shipment, err := CreateShipment(ctx, ws, "Not in shipment", nil)
	require.NoError(t, err)

	// Act
	err = ReturnBlockedItem(ctx, ws, shipment.ID, "T999", "fake reason")

	// Assert
	require.Error(t, err, "returning an item not in the shipment must fail")
}

// T002 / ST014: Verify shipment survives rehydration cycle.
func TestShipment_RehydrationConsistency(t *testing.T) {
	// Arrange
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	shipment, err := CreateShipment(ctx, ws, "Rehydration test", []string{"T001"})
	require.NoError(t, err)

	// Force rehydration by closing and reopening workspace
	ws.Close()
	ws2, err := NewWorkspace(ctx, filepath.Dir(ws.RootPath))
	require.NoError(t, err)
	defer ws2.Close()

	// Act
	recovered, err := GetShipment(ctx, ws2, shipment.ID)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, shipment.ID, recovered.ID)
	assert.Equal(t, shipment.Title, recovered.Title)
}
