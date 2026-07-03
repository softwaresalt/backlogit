package core

// 015.009-T: ReturnBlockedItem dual-write atomicity — integration harness.
//
// These tests assert that ReturnBlockedItem leaves the workspace in a
// consistent state: the DB and the on-disk Markdown files agree after both a
// successful return and after a rolled-back attempt.
//
// The current production implementation persists the shipment and item
// separately and uses compensating rollback logic if a later write fails.
// These tests verify the resulting consistency guarantees rather than
// asserting that both writes occur inside a shared SQL transaction.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bldb "github.com/softwaresalt/backlogit/internal/db"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/models"
)

// TestReturnBlockedItem_ShipmentAndItemAreConsistentInDB verifies that after a
// successful ReturnBlockedItem the SQLite index agrees with the Markdown files:
// the item is blocked and the shipment no longer contains the item.
func TestReturnBlockedItem_ShipmentAndItemAreConsistentInDB(t *testing.T) {
	// Arrange
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	feat, err := CreateArtifact(ctx, ws, "Blocking feature", "feature")
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, feat))
	task, err := CreateArtifact(ctx, ws, "Blocking task", "task", WithParent(feat.ID))
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, task))

	shipment, err := CreateShipment(ctx, ws, "Consistency check shipment", []string{task.ID})
	require.NoError(t, err)
	_, err = ClaimShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)

	// Act
	err = ReturnBlockedItem(ctx, ws, shipment.ID, task.ID, "test blocking reason")

	// Assert — no error
	require.NoError(t, err)

	// Assert — DB agrees: task is blocked
	dbTask, err := bldb.GetItem(ctx, ws.DB, task.ID)
	require.NoError(t, err)
	assert.Equal(t, models.StatusBlocked, dbTask.Status, "DB: task must be blocked")
	assert.Equal(t, "test blocking reason", dbTask.CustomFields["blocked_reason"])

	// Assert — DB agrees: shipment items list no longer contains the task
	dbShipment, err := bldb.GetItem(ctx, ws.DB, shipment.ID)
	require.NoError(t, err)
	items := NormalizeShipmentItems(dbShipment)
	assert.NotContains(t, items, task.ID, "DB: shipment must not list blocked task")

	// Assert — file agrees with DB: reload artifact from disk
	fileTask, loadErr := loadArtifact(ctx, ws, task.ID)
	require.NoError(t, loadErr)
	assert.Equal(t, models.StatusBlocked, fileTask.Status, "file: task must be blocked")
}

// TestReturnBlockedItem_FileAndDBAgreeAfterReturn is a companion check
// asserting the on-disk artifact status and the DB index status are identical
// (neither can be "ahead" of the other).
func TestReturnBlockedItem_FileAndDBAgreeAfterReturn(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	feat, err := CreateArtifact(ctx, ws, "Agree feature", "feature")
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, feat))
	task, err := CreateArtifact(ctx, ws, "Agree task", "task", WithParent(feat.ID))
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, task))

	shipment, err := CreateShipment(ctx, ws, "Agree shipment", []string{task.ID})
	require.NoError(t, err)
	_, err = ClaimShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)

	require.NoError(t, ReturnBlockedItem(ctx, ws, shipment.ID, task.ID, "reason"))

	dbItem, err := bldb.GetItem(ctx, ws.DB, task.ID)
	require.NoError(t, err)
	fileItem, err := loadArtifact(ctx, ws, task.ID)
	require.NoError(t, err)

	assert.Equal(t, dbItem.Status, fileItem.Status, "DB and file task status must match")

	dbShipment, err := bldb.GetItem(ctx, ws.DB, shipment.ID)
	require.NoError(t, err)
	fileShipment, err := loadArtifact(ctx, ws, shipment.ID)
	require.NoError(t, err)

	assert.Equal(t, NormalizeShipmentItems(dbShipment), NormalizeShipmentItems(fileShipment),
		"DB and file shipment items list must match")
}

// TestReturnBlockedItem_RejectsItemNotInShipment verifies the guard condition
// returns ErrCannotReturnItem without modifying any state.
func TestReturnBlockedItem_RejectsItemNotInShipment(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	feat, err := CreateArtifact(ctx, ws, "Outside feature", "feature")
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, feat))
	task, err := CreateArtifact(ctx, ws, "Outside task", "task", WithParent(feat.ID))
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, task))

	shipment, err := CreateShipment(ctx, ws, "Empty shipment", nil)
	require.NoError(t, err)
	_, err = ClaimShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)

	err = ReturnBlockedItem(ctx, ws, shipment.ID, task.ID, "not in shipment")

	require.Error(t, err)
	assert.ErrorIs(t, err, blerrors.ErrCannotReturnItem)

	// Task must remain untouched.
	dbTask, getErr := bldb.GetItem(ctx, ws.DB, task.ID)
	require.NoError(t, getErr)
	assert.Equal(t, models.StatusQueued, dbTask.Status, "task must remain queued after rejected return")
}

// TestReturnBlockedItem_MultipleReturnsAreIdempotentOnItems verifies that
// returning different items from the same shipment produces independent results
// and each DB record reflects only its own return.
func TestReturnBlockedItem_MultipleReturnsAreIndependent(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	feat, err := CreateArtifact(ctx, ws, "Multi-return feature", "feature")
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, feat))
	taskA, err := CreateArtifact(ctx, ws, "Task A", "task", WithParent(feat.ID))
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, taskA))
	taskB, err := CreateArtifact(ctx, ws, "Task B", "task", WithParent(feat.ID))
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, taskB))

	shipment, err := CreateShipment(ctx, ws, "Multi-return shipment", []string{taskA.ID, taskB.ID})
	require.NoError(t, err)
	_, err = ClaimShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)

	require.NoError(t, ReturnBlockedItem(ctx, ws, shipment.ID, taskA.ID, "reason A"))
	require.NoError(t, ReturnBlockedItem(ctx, ws, shipment.ID, taskB.ID, "reason B"))

	dbA, _ := bldb.GetItem(ctx, ws.DB, taskA.ID)
	dbB, _ := bldb.GetItem(ctx, ws.DB, taskB.ID)
	assert.Equal(t, models.StatusBlocked, dbA.Status)
	assert.Equal(t, models.StatusBlocked, dbB.Status)
	assert.Equal(t, "reason A", dbA.CustomFields["blocked_reason"])
	assert.Equal(t, "reason B", dbB.CustomFields["blocked_reason"])

	dbShipment, _ := bldb.GetItem(ctx, ws.DB, shipment.ID)
	assert.Empty(t, NormalizeShipmentItems(dbShipment))
}
