package core

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/models"
)

// A shipment that lists only a feature must not need the item-log lock (C) of
// the feature's unlisted descendants: the release scope is flat, the archive
// loop does not cascade, and the parent status rollup stops at non-members. A
// busy C for an unlisted descendant therefore must not delay or fail ShipShipment.
func TestShipShipment_LockedUnlistedDescendantItemLogDoesNotBlockShip(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	feature, err := CreateArtifact(ctx, ws, "Flat C feature", "feature")
	require.NoError(t, err)
	child, err := CreateArtifact(ctx, ws, "Flat C unlisted child", "task", WithParent(feature.ID))
	require.NoError(t, err)
	grandchild, err := CreateArtifact(ctx, ws, "Flat C unlisted grandchild", "subtask", WithParent(child.ID))
	require.NoError(t, err)

	shipment, err := CreateShipment(ctx, ws, "Flat C shipment", []string{feature.ID})
	require.NoError(t, err)
	_, err = ClaimShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)

	locksRoot := WorkspaceLocksRoot(ws.RootPath)
	logsDir := WorkspaceLogsRoot(ws.RootPath)
	for _, id := range []string{child.ID, grandchild.ID} {
		_, unlock, lockErr := events.LockItemLogCrossProcess(ctx, locksRoot, logsDir, id)
		require.NoError(t, lockErr)
		t.Cleanup(unlock)
	}

	// Run bounded: a regression blocks on the busy lock, so fail fast instead of hanging.
	type shipOutcome struct {
		result *ShipShipmentResult
		err    error
	}
	done := make(chan shipOutcome, 1)
	go func() {
		r, e := ShipShipment(ctx, ws, shipment.ID, nil)
		done <- shipOutcome{r, e}
	}()
	var result *ShipShipmentResult
	select {
	case out := <-done:
		result, err = out.result, out.err
	case <-time.After(20 * time.Second):
		t.Fatal("ShipShipment blocked on the item-log lock of an unlisted descendant")
	}

	require.NoError(t, err, "a locked item log of an unlisted descendant must not block ShipShipment")
	require.NotNil(t, result)
	assert.Contains(t, result.ArchivedIDs, feature.ID)
	assert.NotContains(t, result.ArchivedIDs, child.ID)
	assert.NotContains(t, result.ArchivedIDs, grandchild.ID)
	assert.NotEqual(t, models.StatusArchived, readArtifactForTest(t, ctx, ws, child.ID).Status)
}
