package core

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

func TestArchiveItemGovernance_WaitsForGlobalLifecycleLock(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	artifact, err := CreateArtifact(ctx, ws, "archive global lock feature", "feature")
	require.NoError(t, err)

	unlock, err := lockShipmentMembership(ctx, ws, shipmentLifecycleGlobalLockID)
	require.NoError(t, err)
	locked := true
	t.Cleanup(func() {
		if locked {
			require.NoError(t, unlock())
		}
	})
	result := make(chan error, 1)
	observedCtx, attempted, acquired := p021ObserveGlobalLock(ctx)
	go func() {
		_, archiveErr := ArchiveItem(observedCtx, ws.DB, ws, artifact.ID)
		result <- archiveErr
	}()
	<-attempted

	select {
	case <-acquired:
		t.Fatal("archive acquired the workspace-global lifecycle lock while its competitor held it")
	case archiveErr := <-result:
		require.NoError(t, archiveErr)
		t.Fatal("archive completed while the workspace-global lifecycle lock was held")
	case <-time.After(150 * time.Millisecond):
	}

	require.NoError(t, unlock())
	locked = false
	select {
	case archiveErr := <-result:
		require.NoError(t, archiveErr)
	case <-time.After(defaultGateLockBoundedWait + 2*time.Second):
		t.Fatal("archive did not finish after the global lifecycle lock was released")
	}
}

func TestArchiveItemGovernance_WaitsForShipmentMembershipLock(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	feature, err := CreateArtifact(ctx, ws, "archive lock feature", "feature")
	require.NoError(t, err)
	member, err := CreateArtifact(ctx, ws, "archive lock member", "task", WithParent(feature.ID))
	require.NoError(t, err)
	shipment, err := CreateShipment(ctx, ws, "archive lock shipment", []string{member.ID})
	require.NoError(t, err)

	unlock, err := lockShipmentMembership(ctx, ws, shipment.ID)
	require.NoError(t, err)
	locked := true
	t.Cleanup(func() {
		if locked {
			require.NoError(t, unlock())
		}
	})
	result := make(chan error, 1)
	go func() {
		_, archiveErr := ArchiveItem(ctx, ws.DB, ws, member.ID)
		result <- archiveErr
	}()

	select {
	case archiveErr := <-result:
		require.NoError(t, archiveErr)
		t.Fatal("archive completed while the relevant shipment membership lock was held")
	case <-time.After(150 * time.Millisecond):
	}

	require.NoError(t, unlock())
	locked = false
	select {
	case archiveErr := <-result:
		require.NoError(t, archiveErr)
	case <-time.After(defaultGateLockBoundedWait + 2*time.Second):
		t.Fatal("archive did not finish after the shipment membership lock was released")
	}
}

func TestArchiveItemGovernance_BlockedShipmentMemberRefusesWithoutMutation(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	fixture := newURBlockedActiveFixture(t, ws)
	_, err := BlockShipment(ctx, ws, fixture.shipment.ID, BlockOptions{
		Reason:    "archive member must remain in blocked aggregate",
		BlockedBy: "archive governance test",
	})
	require.NoError(t, err)
	before := snapshotURAggregate(t, ws, fixture.shipment.ID)

	_, err = ArchiveItem(ctx, ws.DB, ws, fixture.members[0].ID)

	require.ErrorIs(t, err, blerrors.ErrShipmentConflict)
	requireURAggregateUnchanged(t, ws, before)
}
