package core

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestAdoptItem_AdoptedItem_EmitsUTCUpdatedAt proves the adopted item's own
// updated_at is restamped in canonical UTC during adoption, even under a
// non-UTC local zone (site: shipment_lifecycle.go AdoptItem).
func TestAdoptItem_AdoptedItem_EmitsUTCUpdatedAt(t *testing.T) {
	withNonUTCLocalWB(t)
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	feat1, err := CreateArtifact(ctx, ws, "Adopt origin feature", "feature")
	require.NoError(t, err)
	feat2, err := CreateArtifact(ctx, ws, "Adopt destination feature", "feature")
	require.NoError(t, err)
	task, err := CreateArtifact(ctx, ws, "Adopt task", "task", WithParent(feat1.ID))
	require.NoError(t, err)

	result, err := AdoptItem(ctx, ws, task.ID, feat2.ID)
	require.NoError(t, err)

	assertFrontmatterUTCWB(t, readArtifactContentWB(t, ctx, ws, result.NewID), "updated_at")
}

// TestClearParentID_EmitsUTCUpdatedAt proves clearParentID restamps updated_at
// in canonical UTC even under a non-UTC local zone (site:
// shipment_lifecycle.go clearParentID).
func TestClearParentID_EmitsUTCUpdatedAt(t *testing.T) {
	withNonUTCLocalWB(t)
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	feat, err := CreateArtifact(ctx, ws, "ClearParent feature", "feature")
	require.NoError(t, err)
	task, err := CreateArtifact(ctx, ws, "ClearParent task", "task", WithParent(feat.ID))
	require.NoError(t, err)

	require.NoError(t, clearParentID(ctx, ws, task.ID))

	assertFrontmatterUTCWB(t, readArtifactContentWB(t, ctx, ws, task.ID), "updated_at")
}
