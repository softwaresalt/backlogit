package core

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/models"
)

func TestP021ClaimSerialization_MembershipAndGenericWritersUseGlobalLock(t *testing.T) {
	t.Run("membership_writer_waits_and_deduplicates", func(t *testing.T) {
		ws := setupShipmentWorkspace(t)
		ctx := context.Background()
		feature, err := CreateArtifact(ctx, ws, "P021 membership feature", "feature")
		require.NoError(t, err)
		member, err := CreateArtifact(ctx, ws, "P021 membership member", "task", WithParent(feature.ID))
		require.NoError(t, err)
		shipment, err := CreateShipment(ctx, ws, "P021 membership shipment", nil)
		require.NoError(t, err)

		unlock, err := lockShipmentMembership(ctx, ws, shipmentLifecycleGlobalLockID)
		require.NoError(t, err)
		result := make(chan error, 1)
		observedCtx, attempted, acquired := p021ObserveGlobalLock(ctx)
		go func() {
			result <- AddItemToShipment(observedCtx, ws, shipment.ID, member.ID)
		}()
		<-attempted

		var early error
		completedWhileLocked := false
		select {
		case <-acquired:
			t.Fatal("membership writer acquired the workspace-global lifecycle lock while its competitor held it")
		case early = <-result:
			completedWhileLocked = true
		case <-time.After(150 * time.Millisecond):
		}
		require.NoError(t, unlock())
		if !completedWhileLocked {
			select {
			case early = <-result:
			case <-time.After(defaultGateLockBoundedWait + 2*time.Second):
				t.Fatal("membership writer did not finish after the global lifecycle lock was released")
			}
		}

		require.False(t, completedWhileLocked, "membership writer bypassed the workspace-global lifecycle lock")
		require.NoError(t, early)
		require.NoError(t, AddItemToShipment(ctx, ws, shipment.ID, member.ID))
		got, err := GetShipment(ctx, ws, shipment.ID)
		require.NoError(t, err)
		require.Equal(t, []string{member.ID}, NormalizeShipmentItems(got),
			"serialized retries must not leave duplicate or partial membership")
		require.Equal(t, models.StatusQueued, loadURCanonicalArtifact(t, ws, member.ID).Status)
	})

	writers := []struct {
		name  string
		setup func(*testing.T, *Workspace) func(context.Context) error
	}{
		{
			name: "generic_update",
			setup: func(t *testing.T, ws *Workspace) func(context.Context) error {
				t.Helper()
				shipment, err := CreateShipment(context.Background(), ws, "P021 generic update", nil)
				require.NoError(t, err)
				return func(ctx context.Context) error {
					_, updateErr := UpdateArtifact(ctx, ws, shipment.ID, map[string]any{
						"title": "P021 generic update completed",
					})
					return updateErr
				}
			},
		},
		{
			name: "bulk_update",
			setup: func(t *testing.T, ws *Workspace) func(context.Context) error {
				t.Helper()
				feature, err := CreateArtifact(context.Background(), ws, "P021 bulk feature", "feature")
				require.NoError(t, err)
				member, err := CreateArtifact(
					context.Background(),
					ws,
					"P021 bulk member",
					"task",
					WithParent(feature.ID),
				)
				require.NoError(t, err)
				return func(ctx context.Context) error {
					result, bulkErr := BulkUpdateStatus(
						ctx,
						ws.DB,
						ws,
						[]string{member.ID},
						string(models.StatusActive),
					)
					if bulkErr != nil {
						return bulkErr
					}
					if len(result.Failed) != 0 || result.Succeeded != 1 {
						return errors.New("bulk update did not complete exactly once")
					}
					return nil
				}
			},
		},
		{
			name: "cascade_update",
			setup: func(t *testing.T, ws *Workspace) func(context.Context) error {
				t.Helper()
				feature, err := CreateArtifact(context.Background(), ws, "P021 cascade feature", "feature")
				require.NoError(t, err)
				member, err := CreateArtifact(
					context.Background(),
					ws,
					"P021 cascade member",
					"task",
					WithParent(feature.ID),
					WithStatus(string(models.StatusActive)),
				)
				require.NoError(t, err)
				return func(ctx context.Context) error {
					return cascadePersistedParentStatuses(ctx, ws, member.ID)
				}
			},
		},
	}

	for _, writer := range writers {
		t.Run(writer.name+"_waits", func(t *testing.T) {
			ws := setupShipmentWorkspace(t)
			action := writer.setup(t, ws)
			ctx := context.Background()
			unlock, err := lockShipmentMembership(ctx, ws, shipmentLifecycleGlobalLockID)
			require.NoError(t, err)

			result := make(chan error, 1)
			observedCtx, attempted, acquired := p021ObserveGlobalLock(ctx)
			go func() {
				result <- action(observedCtx)
			}()
			<-attempted

			var actionErr error
			completedWhileLocked := false
			select {
			case <-acquired:
				t.Fatalf("%s acquired the workspace-global lifecycle lock while its competitor held it", writer.name)
			case actionErr = <-result:
				completedWhileLocked = true
			case <-time.After(150 * time.Millisecond):
			}
			require.NoError(t, unlock())
			if !completedWhileLocked {
				select {
				case actionErr = <-result:
				case <-time.After(defaultGateLockBoundedWait + 2*time.Second):
					t.Fatalf("%s did not finish after the global lifecycle lock was released", writer.name)
				}
			}

			require.False(t, completedWhileLocked, "%s bypassed the workspace-global lifecycle lock", writer.name)
			require.NoError(t, actionErr)
		})
	}
}

func p021ObserveGlobalLock(ctx context.Context) (context.Context, <-chan struct{}, <-chan struct{}) {
	attempted := make(chan struct{}, 1)
	acquired := make(chan struct{}, 1)
	hook := func(phase string) {
		switch phase {
		case "attempt":
			attempted <- struct{}{}
		case "acquired":
			acquired <- struct{}{}
		}
	}
	return context.WithValue(ctx, shipmentLifecycleGlobalLockHookContextKey{}, hook), attempted, acquired
}

func TestP021RecoveryCAS_RefusesDriftAndRestoresExactPreimage(t *testing.T) {
	t.Run("rollback_refuses_member_drift_without_mutation", func(t *testing.T) {
		ws := setupShipmentWorkspace(t)
		fixture := newURBlockedActiveFixture(t, ws)
		journal := p021LifecycleJournal(t, ws, "block", "rollback", fixture.shipment.ID, "")
		journalPath := p021WriteLifecycleJournal(t, ws, journal)

		drifted := cloneArtifact(journal.Preimage.Members[0])
		drifted.Status = models.StatusQueued
		drifted.Title = "post-crash member edit"
		drifted.UpdatedAt = models.NowUTC()
		forceURArtifactFixture(t, ws, drifted)
		before := snapshotURAggregate(t, ws, fixture.shipment.ID)

		err := recoverPendingShipmentOperations(context.Background(), ws)
		require.ErrorIs(t, err, blerrors.ErrShipmentConflict)
		requireURAggregateUnchanged(t, ws, before)
		require.Equal(t, "intent", p021ReadLifecycleJournal(t, journalPath).Phase)
	})

	t.Run("unblock_rollback_refuses_shipment_drift_without_mutation", func(t *testing.T) {
		ws := setupShipmentWorkspace(t)
		fixture := newURBlockedActiveFixture(t, ws)
		_, err := BlockShipment(context.Background(), ws, fixture.shipment.ID, BlockOptions{
			Reason:    "P021 blocked",
			BlockedBy: "P021",
		})
		require.NoError(t, err)
		journal := p021LifecycleJournal(t, ws, "unblock", "rollback", fixture.shipment.ID, "")
		journal.Target = string(ShipmentActive)
		journalPath := p021WriteLifecycleJournal(t, ws, journal)

		drifted := cloneArtifact(journal.Preimage.Shipment)
		drifted.Status = models.StatusActive
		drifted.Title = "post-crash shipment edit"
		delete(drifted.CustomFields, "blocked_reason")
		delete(drifted.CustomFields, "blocked_at")
		delete(drifted.CustomFields, "blocked_by")
		drifted.UpdatedAt = models.NowUTC()
		forceURArtifactFixture(t, ws, drifted)
		before := snapshotURAggregate(t, ws, fixture.shipment.ID)

		err = recoverPendingShipmentOperations(context.Background(), ws)
		require.ErrorIs(t, err, blerrors.ErrShipmentConflict)
		requireURAggregateUnchanged(t, ws, before)
		require.Equal(t, "intent", p021ReadLifecycleJournal(t, journalPath).Phase)
	})

	t.Run("normalizer_roll_forward_refuses_member_drift_without_mutation", func(t *testing.T) {
		ws := setupShipmentWorkspace(t)
		fixture := newURBlockedActiveFixture(t, ws)
		_, err := BlockShipment(context.Background(), ws, fixture.shipment.ID, BlockOptions{
			Reason:    "P021 normalize source",
			BlockedBy: "P021",
		})
		require.NoError(t, err)
		snapshotRef := p021WriteBlockedSnapshot(t, ws, fixture.shipment.ID)
		journal := p021LifecycleJournal(t, ws, "normalize", "roll_forward", fixture.shipment.ID, snapshotRef)
		journalPath := p021WriteLifecycleJournal(t, ws, journal)

		drifted := cloneArtifact(journal.Preimage.Members[0])
		drifted.Title = "post-crash normalizer edit"
		drifted.UpdatedAt = models.NowUTC()
		forceURArtifactFixture(t, ws, drifted)
		before := snapshotURAggregate(t, ws, fixture.shipment.ID)

		err = recoverPendingShipmentOperations(context.Background(), ws)
		require.ErrorIs(t, err, blerrors.ErrShipmentConflict)
		requireURAggregateUnchanged(t, ws, before)
		require.Equal(t, "intent", p021ReadLifecycleJournal(t, journalPath).Phase)
	})

	t.Run("bootstrap_roll_forward_refuses_shipment_drift_without_mutation", func(t *testing.T) {
		ws := setupShipmentWorkspace(t)
		fixture := newURBlockedActiveFixture(t, ws)
		snapshotRef := p021WriteBootstrapSnapshot(t, ws, fixture.shipment.ID)
		journal := p021LifecycleJournal(t, ws, "block", "roll_forward", fixture.shipment.ID, snapshotRef)
		journalPath := p021WriteLifecycleJournal(t, ws, journal)

		drifted := cloneArtifact(journal.Preimage.Shipment)
		drifted.Title = "post-intent bootstrap edit"
		drifted.UpdatedAt = models.NowUTC()
		forceURArtifactFixture(t, ws, drifted)
		before := snapshotURAggregate(t, ws, fixture.shipment.ID)

		err := recoverPendingShipmentOperations(context.Background(), ws)
		require.ErrorIs(t, err, blerrors.ErrShipmentConflict)
		requireURAggregateUnchanged(t, ws, before)
		require.Equal(t, "intent", p021ReadLifecycleJournal(t, journalPath).Phase)
	})

	t.Run("rollback_restores_exact_preimage_when_cas_holds", func(t *testing.T) {
		ws := setupShipmentWorkspace(t)
		fixture := newURBlockedActiveFixture(t, ws)
		journal := p021LifecycleJournal(t, ws, "block", "rollback", fixture.shipment.ID, "")
		journalPath := p021WriteLifecycleJournal(t, ws, journal)

		torn := cloneArtifact(journal.Preimage.Members[0])
		torn.Status = models.StatusQueued
		torn.UpdatedAt = models.NowUTC()
		forceURArtifactFixture(t, ws, torn)

		require.NoError(t, recoverPendingShipmentOperations(context.Background(), ws))
		assertURArtifactEqual(t, journal.Preimage.Shipment, loadURCanonicalArtifact(t, ws, fixture.shipment.ID))
		for _, member := range journal.Preimage.Members {
			assertURArtifactEqual(t, member, loadURCanonicalArtifact(t, ws, member.ID))
		}
		require.Equal(t, "compensated", p021ReadLifecycleJournal(t, journalPath).Phase)
	})
}

func p021LifecycleJournal(
	t *testing.T,
	ws *Workspace,
	operation string,
	recoveryPolicy string,
	shipmentID string,
	snapshotRef string,
) shipmentLifecycleJournal {
	t.Helper()

	shipment := cloneArtifact(loadURCanonicalArtifact(t, ws, shipmentID))
	memberIDs := NormalizeShipmentItems(shipment)
	members := make([]*models.Artifact, 0, len(memberIDs))
	for _, memberID := range memberIDs {
		members = append(members, cloneArtifact(loadURCanonicalArtifact(t, ws, memberID)))
	}
	return shipmentLifecycleJournal{
		SchemaVersion:  "shipment-operation/v1",
		CorrelationID:  "p021-" + operation,
		Phase:          "intent",
		Operation:      operation,
		RecoveryPolicy: recoveryPolicy,
		ShipmentID:     shipmentID,
		Target:         string(ShipmentBlocked),
		Reason:         "P021 recovery",
		BlockedBy:      "P021",
		SnapshotRef:    snapshotRef,
		Preimage: shipmentLifecyclePreimage{
			Shipment: shipment,
			Members:  members,
		},
	}
}

func p021WriteLifecycleJournal(t *testing.T, ws *Workspace, journal shipmentLifecycleJournal) string {
	t.Helper()

	journal.CorrelationID = p021CorrelationID(journal.Operation)
	path, err := writeShipmentLifecycleJournalForWorkspace(
		ws,
		shipmentLifecycleJournalName(journal.CorrelationID),
		journal,
	)
	require.NoError(t, err)
	return path
}

func p021CorrelationID(operation string) string {
	switch operation {
	case "block":
		return "11111111111111111111111111111111"
	case "unblock":
		return "22222222222222222222222222222222"
	case "normalize":
		return "33333333333333333333333333333333"
	default:
		return "ffffffffffffffffffffffffffffffff"
	}
}

func p021ReadLifecycleJournal(t *testing.T, path string) shipmentLifecycleJournal {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var journal shipmentLifecycleJournal
	require.NoError(t, json.Unmarshal(data, &journal))
	return journal
}

func p021WriteBootstrapSnapshot(t *testing.T, ws *Workspace, shipmentID string) string {
	t.Helper()

	shipment := loadURCanonicalArtifact(t, ws, shipmentID)
	memberStatuses := make(map[string]string)
	for _, memberID := range NormalizeShipmentItems(shipment) {
		memberStatuses[memberID] = string(loadURCanonicalArtifact(t, ws, memberID).Status)
	}
	snapshot := ShipmentBlockedSnapshot{
		SchemaVersion: ShipmentBlockedSnapshotSchemaVersion,
		ShipmentID:    shipmentID,
		Target:        ShipmentBlocked,
		BlockedReason: "P021 bootstrap recovery",
		BlockedAt:     time.Now().UTC().Format(time.RFC3339),
		BlockedBy:     "P021",
		Members:       memberStatuses,
	}
	data, err := json.Marshal(snapshot)
	require.NoError(t, err)
	relativePath := filepath.Join("bootstrap", "p021-"+shipmentID+".json")
	absolutePath := filepath.Join(WorkspaceStorageRoot(ws.RootPath), relativePath)
	require.NoError(t, os.MkdirAll(filepath.Dir(absolutePath), 0o755))
	require.NoError(t, os.WriteFile(absolutePath, data, 0o644))
	return relativePath
}

func p021WriteBlockedSnapshot(t *testing.T, ws *Workspace, shipmentID string) string {
	t.Helper()

	shipment := loadURCanonicalArtifact(t, ws, shipmentID)
	memberStatuses := statusSnapshotUR(shipment.CustomFields["member_status_snapshot"])
	snapshot := ShipmentBlockedSnapshot{
		SchemaVersion: ShipmentBlockedSnapshotSchemaVersion,
		ShipmentID:    shipmentID,
		Target:        ShipmentBlocked,
		BlockedReason: "P021 recovered blocked shipment",
		BlockedAt:     time.Now().UTC().Format(time.RFC3339),
		BlockedBy:     "P021",
		Members:       memberStatuses,
	}
	data, err := json.Marshal(snapshot)
	require.NoError(t, err)
	relativePath := filepath.Join("snapshots", "p021-"+shipmentID+".json")
	absolutePath := filepath.Join(WorkspaceStorageRoot(ws.RootPath), relativePath)
	require.NoError(t, os.MkdirAll(filepath.Dir(absolutePath), 0o755))
	require.NoError(t, os.WriteFile(absolutePath, data, 0o644))
	return filepath.ToSlash(relativePath)
}
