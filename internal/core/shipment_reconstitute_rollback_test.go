package core

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bldb "github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/models"
)

func reconstituteRollbackJournal(fx *reconstituteFixture, corr string, created ...string) shipmentLifecycleJournal {
	return shipmentLifecycleJournal{
		SchemaVersion:  "shipment-operation/v1",
		CorrelationID:  corr,
		Phase:          "intent",
		Operation:      "reconstitute",
		RecoveryPolicy: "rollback",
		ShipmentID:     fx.source.ID,
		Target:         "reconstitute",
		Preimage:       shipmentLifecyclePreimage{Shipment: cloneArtifact(fx.source)},
		Reconstitute: &reconstituteJournalState{
			Images: []reconstituteFileImage{}, KnownLogs: []string{}, CreatedIDs: created,
			EventIDs: []string{fx.source.ID}, LockIDs: []string{fx.source.ID},
		},
	}
}

func createIndexedFeature(t *testing.T, ws *Workspace, title string) *models.Artifact {
	t.Helper()
	art, err := CreateArtifact(context.Background(), ws, title, "feature")
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(context.Background(), ws.DB, art))
	return art
}

func TestReconstituteRollbackRemovesOnlyJournaledArtifacts(t *testing.T) {
	ctx := context.Background()
	fx := newReconstituteFixture(t)
	ours := createIndexedFeature(t, fx.ws, "Created by the operation")
	unrelated := createIndexedFeature(t, fx.ws, "Concurrent unrelated create")
	oursPath, err := FindArtifactPath(ctx, fx.ws, ours.ID)
	require.NoError(t, err)
	unrelatedPath, err := FindArtifactPath(ctx, fx.ws, unrelated.ID)
	require.NoError(t, err)

	unlock, err := lockActiveStash(fx.ws)
	require.NoError(t, err)
	t.Cleanup(func() { _ = unlock() })
	require.NoError(t, rollbackReconstitute(withStashLockHeld(ctx), fx.ws,
		reconstituteRollbackJournal(fx, "0000000000000000000000000000b001", ours.ID)))

	_, statErr := os.Stat(oursPath)
	assert.True(t, os.IsNotExist(statErr), "journaled created artifact must be removed")
	_, statErr = os.Stat(unrelatedPath)
	assert.NoError(t, statErr, "unrelated artifact created in the operation window must survive rollback")
}

func TestReconstituteRecoveryUnderHeldStashLock(t *testing.T) {
	ctx := context.Background()
	fx := newReconstituteFixture(t)
	ours := createIndexedFeature(t, fx.ws, "Created by the operation")
	oursPath, err := FindArtifactPath(ctx, fx.ws, ours.ID)
	require.NoError(t, err)

	corr := "0000000000000000000000000000b002"
	journalPath, err := writeShipmentLifecycleJournalForWorkspace(fx.ws, shipmentLifecycleJournalName(corr),
		reconstituteRollbackJournal(fx, corr, ours.ID))
	require.NoError(t, err)

	unlock, err := lockActiveStash(fx.ws)
	require.NoError(t, err)
	t.Cleanup(func() { _ = unlock() })
	// Stash-holding callers (HarvestStashEntry, provenance correction) mark the
	// context before reaching the lifecycle barrier that runs recovery.
	lockedCtx, globalUnlock, err := lockShipmentLifecycleGlobal(withStashLockHeld(ctx), fx.ws)
	require.NoError(t, err, "recovery must not trip over the stash lock the caller already holds")
	_ = lockedCtx
	require.NoError(t, globalUnlock())

	_, statErr := os.Stat(oursPath)
	assert.True(t, os.IsNotExist(statErr), "pending journal must be rolled back")
	_, statErr = os.Stat(journalPath)
	assert.True(t, os.IsNotExist(statErr), "journal must be cleaned up")
}
