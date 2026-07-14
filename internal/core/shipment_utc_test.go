package core_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
)

// TestMoveShipmentStatus_EmitsUTCUpdatedAt proves the shipment status-transition
// writer restamps the shipment's updated_at in canonical UTC even under a
// non-UTC local zone (site: shipment.go moveShipmentStatusWithTopLevel).
func TestMoveShipmentStatus_EmitsUTCUpdatedAt(t *testing.T) {
	withNonUTCLocal(t)
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	feat, err := core.CreateArtifact(ctx, ws, "Ship status feature", "feature")
	require.NoError(t, err)
	task, err := core.CreateArtifact(ctx, ws, "Ship status task", "task", core.WithParent(feat.ID))
	require.NoError(t, err)
	shipment, err := core.CreateShipment(ctx, ws, "Status delivery", []string{task.ID})
	require.NoError(t, err)

	require.NoError(t, core.MoveShipmentStatus(ctx, ws, shipment.ID, core.ShipmentActive))

	content := readArtifactContent(t, ctx, ws, shipment.ID)
	assertFrontmatterUTC(t, content, "updated_at")
}

// TestAddItemToShipment_EmitsUTCUpdatedAt proves adding an item restamps the
// shipment's updated_at in canonical UTC (site: shipment.go AddItemToShipment).
func TestAddItemToShipment_EmitsUTCUpdatedAt(t *testing.T) {
	withNonUTCLocal(t)
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	feat, err := core.CreateArtifact(ctx, ws, "Ship add feature", "feature")
	require.NoError(t, err)
	taskOne, err := core.CreateArtifact(ctx, ws, "Ship add task 1", "task", core.WithParent(feat.ID))
	require.NoError(t, err)
	taskTwo, err := core.CreateArtifact(ctx, ws, "Ship add task 2", "task", core.WithParent(feat.ID))
	require.NoError(t, err)
	shipment, err := core.CreateShipment(ctx, ws, "Add delivery", []string{taskOne.ID})
	require.NoError(t, err)

	require.NoError(t, core.AddItemToShipment(ctx, ws, shipment.ID, taskTwo.ID))

	content := readArtifactContent(t, ctx, ws, shipment.ID)
	assertFrontmatterUTC(t, content, "updated_at")
}

// TestReturnBlockedItem_EmitsUTCUpdatedAt proves returning a blocked item
// restamps both the shipment and the item updated_at in canonical UTC
// (sites: shipment.go ReturnBlockedItem shipment + item restamps).
func TestReturnBlockedItem_EmitsUTCUpdatedAt(t *testing.T) {
	withNonUTCLocal(t)
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	feat, err := core.CreateArtifact(ctx, ws, "Ship return feature", "feature")
	require.NoError(t, err)
	task, err := core.CreateArtifact(ctx, ws, "Ship return task", "task", core.WithParent(feat.ID))
	require.NoError(t, err)
	shipment, err := core.CreateShipment(ctx, ws, "Return delivery", []string{task.ID})
	require.NoError(t, err)

	require.NoError(t, core.ReturnBlockedItem(ctx, ws, shipment.ID, task.ID, "blocked for test"))

	assertFrontmatterUTC(t, readArtifactContent(t, ctx, ws, shipment.ID), "updated_at")
	assertFrontmatterUTC(t, readArtifactContent(t, ctx, ws, task.ID), "updated_at")
}
