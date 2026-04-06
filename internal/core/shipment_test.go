package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/config"
	blerrors "github.com/backlogit/backlogit/internal/errors"
	"github.com/backlogit/backlogit/internal/models"
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
	taskOne, err := CreateArtifact(ctx, ws, "Shipment task 1", "task")
	require.NoError(t, err)
	taskTwo, err := CreateArtifact(ctx, ws, "Shipment task 2", "task")
	require.NoError(t, err)

	// Act
	shipment, err := CreateShipment(ctx, ws, "Sprint 1 delivery", []string{taskOne.ID, taskTwo.ID})

	// Assert
	require.NoError(t, err)
	require.NotNil(t, shipment)
	assert.Equal(t, "shipment", shipment.ArtifactType)
	assert.Equal(t, "queued", string(shipment.Status))
	assert.Contains(t, shipment.ID, "S")
	assert.Equal(t, "Sprint 1 delivery", shipment.Title)
}

// T002 / ST012: Reject creating a shipment with an item already assigned to an active shipment.
func TestCreateShipment_RejectsAlreadyAssignedItem(t *testing.T) {
	// Arrange
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	task, err := CreateArtifact(ctx, ws, "Assigned task", "task")
	require.NoError(t, err)
	_, err = CreateShipment(ctx, ws, "Shipment 1", []string{task.ID})
	require.NoError(t, err)

	// Act
	_, err = CreateShipment(ctx, ws, "Shipment 2", []string{task.ID})

	// Assert
	require.Error(t, err)
	assert.ErrorIs(t, err, blerrors.ErrItemAlreadyAssigned)
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

// T002 / ST013: Reject adding a missing item to a shipment.
func TestAddItemToShipment_MissingItem(t *testing.T) {
	// Arrange
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	shipment, err := CreateShipment(ctx, ws, "With items", nil)
	require.NoError(t, err)

	// Act
	err = AddItemToShipment(ctx, ws, shipment.ID, "T999")

	// Assert
	require.Error(t, err)
	assert.ErrorIs(t, err, blerrors.ErrNotFound)
}

// T002 / ST013: Allow reassigning an item after its previous shipment is shipped.
func TestAddItemToShipment_AllowsItemAfterShippedShipment(t *testing.T) {
	// Arrange
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	task, err := CreateArtifact(ctx, ws, "Reusable task", "task")
	require.NoError(t, err)
	firstShipment, err := CreateShipment(ctx, ws, "Shipment 1", nil)
	require.NoError(t, err)
	secondShipment, err := CreateShipment(ctx, ws, "Shipment 2", nil)
	require.NoError(t, err)
	require.NoError(t, AddItemToShipment(ctx, ws, firstShipment.ID, task.ID))
	require.NoError(t, MoveShipmentStatus(ctx, ws, firstShipment.ID, ShipmentActive))
	require.NoError(t, MoveShipmentStatus(ctx, ws, firstShipment.ID, ShipmentShipped))

	// Act
	err = AddItemToShipment(ctx, ws, secondShipment.ID, task.ID)

	// Assert
	require.NoError(t, err)
	updated, err := GetShipment(ctx, ws, secondShipment.ID)
	require.NoError(t, err)
	assert.Contains(t, shipmentItems(updated), task.ID)
}

// T002 / ST013: Reject adding an item to a shipped shipment.
func TestAddItemToShipment_RejectsTerminalShipment(t *testing.T) {
	// Arrange
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	task, err := CreateArtifact(ctx, ws, "Terminal shipment task", "task")
	require.NoError(t, err)
	shipment, err := CreateShipment(ctx, ws, "Terminal shipment", nil)
	require.NoError(t, err)
	require.NoError(t, MoveShipmentStatus(ctx, ws, shipment.ID, ShipmentActive))
	require.NoError(t, MoveShipmentStatus(ctx, ws, shipment.ID, ShipmentShipped))

	// Act
	err = AddItemToShipment(ctx, ws, shipment.ID, task.ID)

	// Assert
	require.Error(t, err)
	assert.ErrorIs(t, err, blerrors.ErrShipmentConflict)
}

// T002 / ST013: Allow reassigning an item after its previous shipment is archived.
func TestAddItemToShipment_AllowsItemAfterArchivedShipment(t *testing.T) {
	// Arrange
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	task, err := CreateArtifact(ctx, ws, "Archived reusable task", "task")
	require.NoError(t, err)
	firstShipment, err := CreateShipment(ctx, ws, "Archived shipment 1", nil)
	require.NoError(t, err)
	secondShipment, err := CreateShipment(ctx, ws, "Archived shipment 2", nil)
	require.NoError(t, err)
	require.NoError(t, AddItemToShipment(ctx, ws, firstShipment.ID, task.ID))
	_, err = ArchiveItem(ctx, ws.DB, ws, firstShipment.ID)
	require.NoError(t, err)

	// Act
	err = AddItemToShipment(ctx, ws, secondShipment.ID, task.ID)

	// Assert
	require.NoError(t, err)
	updated, err := GetShipment(ctx, ws, secondShipment.ID)
	require.NoError(t, err)
	assert.Contains(t, shipmentItems(updated), task.ID)
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

// T002 / ST013: Reject returning a blocked item from an archived shipment.
func TestReturnBlockedItem_RejectsTerminalShipment(t *testing.T) {
	// Arrange
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	shipment, err := CreateShipment(ctx, ws, "Archived return shipment", nil)
	require.NoError(t, err)
	task, err := CreateArtifact(ctx, ws, "Archived return task", "task")
	require.NoError(t, err)
	require.NoError(t, AddItemToShipment(ctx, ws, shipment.ID, task.ID))
	_, err = ArchiveItem(ctx, ws.DB, ws, shipment.ID)
	require.NoError(t, err)

	// Act
	err = ReturnBlockedItem(ctx, ws, shipment.ID, task.ID, "archived shipment")

	// Assert
	require.Error(t, err)
	assert.ErrorIs(t, err, blerrors.ErrShipmentConflict)
}

// T002 / ST013: Roll back shipment changes when the item update fails.
func TestPersistReturnedBlockedArtifacts_RollsBackOnItemFailure(t *testing.T) {
	// Arrange
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	shipment, err := CreateShipment(ctx, ws, "Rollback shipment", nil)
	require.NoError(t, err)
	task, err := CreateArtifact(ctx, ws, "Rollback task", "task")
	require.NoError(t, err)
	require.NoError(t, AddItemToShipment(ctx, ws, shipment.ID, task.ID))

	currentShipment, err := GetShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)
	currentItem, err := loadArtifact(ctx, ws, task.ID)
	require.NoError(t, err)

	originalShipment := cloneArtifact(currentShipment)
	originalItem := cloneArtifact(currentItem)

	currentShipment.CustomFields["items"] = removeString(shipmentItems(currentShipment), task.ID)
	currentShipment.UpdatedAt = time.Now()
	currentItem.Status = models.StatusBlocked
	currentItem.Title = ""
	currentItem.UpdatedAt = time.Now()

	// Act
	rolledBack, err := persistReturnedBlockedArtifacts(ctx, ws, originalShipment, currentShipment, originalItem, currentItem)

	// Assert
	require.Error(t, err)
	assert.True(t, rolledBack)
	restoredShipment, loadShipmentErr := GetShipment(ctx, ws, shipment.ID)
	require.NoError(t, loadShipmentErr)
	assert.Contains(t, shipmentItems(restoredShipment), task.ID)
	restoredItem, loadItemErr := loadArtifact(ctx, ws, task.ID)
	require.NoError(t, loadItemErr)
	assert.Equal(t, models.StatusQueued, restoredItem.Status)
}

// T002 / ST013: Restore the Markdown file when DB upsert fails after file write.
func TestPersistArtifact_RestoresFileOnUpsertFailure(t *testing.T) {
	// Arrange
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	shipment, err := CreateShipment(ctx, ws, "File restore shipment", nil)
	require.NoError(t, err)
	currentPath, err := FindArtifactPath(ctx, ws, shipment.ID)
	require.NoError(t, err)
	originalContent, err := os.ReadFile(currentPath)
	require.NoError(t, err)
	shipment.Title = "Broken after file write"
	shipment.UpdatedAt = time.Now()
	require.NoError(t, ws.DB.Close())

	// Act
	err = persistArtifact(ctx, ws, shipment, false)

	// Assert
	require.Error(t, err)
	restoredContent, readErr := os.ReadFile(currentPath)
	require.NoError(t, readErr)
	assert.Equal(t, string(originalContent), string(restoredContent))
}

// T002 / ST013: Recover a journaled blocked-item return on workspace reopen.
func TestNewWorkspace_RecoversPendingReturnBlockedJournal(t *testing.T) {
	// Arrange
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	shipment, err := CreateShipment(ctx, ws, "Recovered shipment", nil)
	require.NoError(t, err)
	task, err := CreateArtifact(ctx, ws, "Recovered task", "task")
	require.NoError(t, err)
	require.NoError(t, AddItemToShipment(ctx, ws, shipment.ID, task.ID))
	originalShipment, err := GetShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)
	originalItem, err := loadArtifact(ctx, ws, task.ID)
	require.NoError(t, err)
	updatedShipment := cloneArtifact(originalShipment)
	updatedShipment.CustomFields["items"] = removeString(shipmentItems(updatedShipment), task.ID)
	updatedShipment.UpdatedAt = time.Now()
	require.NoError(t, writeReturnBlockedJournal(ws.RootPath, originalShipment, originalItem))
	require.NoError(t, persistArtifact(ctx, ws, updatedShipment, false))
	rootPath := ws.RootPath
	require.NoError(t, ws.Close())

	// Act
	reopened, err := NewWorkspace(ctx, rootPath)
	require.NoError(t, err)
	defer reopened.Close()

	// Assert
	recoveredShipment, err := GetShipment(ctx, reopened, shipment.ID)
	require.NoError(t, err)
	assert.Contains(t, shipmentItems(recoveredShipment), task.ID)
	recoveredItem, err := loadArtifact(ctx, reopened, task.ID)
	require.NoError(t, err)
	assert.Equal(t, models.StatusQueued, recoveredItem.Status)
	_, statErr := os.Stat(returnBlockedJournalPath(rootPath, shipment.ID, task.ID))
	assert.ErrorIs(t, statErr, os.ErrNotExist)
}

// T002 / ST014: Verify shipment survives rehydration cycle.
func TestShipment_RehydrationConsistency(t *testing.T) {
	// Arrange
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	task, err := CreateArtifact(ctx, ws, "Rehydration task", "task")
	require.NoError(t, err)
	shipment, err := CreateShipment(ctx, ws, "Rehydration test", []string{task.ID})
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
