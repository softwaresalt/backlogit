package core

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/models"
)

// TestShipmentClaimCascadeRecovery pins claim crash recovery when the bounded
// parent cascade moved a non-queued (done) member-parent to active.
func TestShipmentClaimCascadeRecovery(t *testing.T) {
	t.Run("rolls back child and cascaded done parent to exact preimage", func(t *testing.T) {
		fixture := newClaimCascadeRecoveryFixture(t)
		fixture.persistCrashState(t, "")

		recovered, err := reopenClaimMarkerRecoveryWorkspace(t, fixture.base)
		if !assert.NoError(t, err, "recovery must accept an unmarked active member-parent whose preimage was done") {
			assert.ErrorIs(t, err, blerrors.ErrShipmentConflict, "the expected RED is the current marker recovery conflict")
			return
		}
		require.NotNil(t, recovered)
		for artifactID, want := range fixture.base.preimageFiles {
			assert.Equal(t, want, claimMarkerRecoveryArtifactFileBytes(t, recovered, artifactID),
				"recovery must restore artifact %s byte-for-byte to its preimage", artifactID)
		}
		parent, loadErr := loadArtifact(context.Background(), recovered, fixture.parentID)
		require.NoError(t, loadErr)
		assert.Equal(t, models.StatusDone, parent.Status)
		child, loadErr := loadArtifact(context.Background(), recovered, fixture.childID)
		require.NoError(t, loadErr)
		assert.Equal(t, models.StatusQueued, child.Status)
	})

	t.Run("parent marked for a different shipment fails closed", func(t *testing.T) {
		fixture := newClaimCascadeRecoveryFixture(t)
		fixture.persistCrashState(t, "FOREIGN-S")

		recovered, err := reopenClaimMarkerRecoveryWorkspace(t, fixture.base)
		assert.Nil(t, recovered, "recovery must not return a workspace after a diverged parent marker")
		assert.ErrorIs(t, err, blerrors.ErrShipmentConflict)
	})

	t.Run("committed terminal claim leaves non-cascaded done member done", func(t *testing.T) {
		ctx := context.Background()
		_, ws := setupUR3Workspace(t)
		t.Cleanup(func() { require.NoError(t, ws.Close()) })

		feature, err := CreateArtifact(ctx, ws, "Cascade committed feature", "feature")
		require.NoError(t, err)
		child, err := CreateArtifact(ctx, ws, "Cascade committed child", "task", WithParent(feature.ID))
		require.NoError(t, err)
		doneMember, err := CreateArtifact(ctx, ws, "Cascade committed done member", "feature")
		require.NoError(t, err)
		shipment, err := CreateShipment(ctx, ws, "Cascade committed shipment", []string{child.ID, doneMember.ID})
		require.NoError(t, err)
		doneMember.Status = models.StatusDone
		require.NoError(t, persistArtifact(ctx, ws, doneMember, true))

		preShipment, err := loadArtifact(ctx, ws, shipment.ID)
		require.NoError(t, err)
		preChild, err := loadArtifact(ctx, ws, child.ID)
		require.NoError(t, err)
		preDone, err := loadArtifact(ctx, ws, doneMember.ID)
		require.NoError(t, err)
		preimage := shipmentLifecyclePreimage{
			Shipment: cloneArtifact(preShipment),
			Members:  []*models.Artifact{cloneArtifact(preChild), cloneArtifact(preDone)},
		}

		_, claimErr := ClaimShipment(ctx, ws, shipment.ID)
		require.NoError(t, claimErr)

		// Recreate the crash window after durable committed evidence but before
		// the journal commit: a lifecycle journal still at intent plus correlated
		// intent, applied, and committed events, so recovery finalizes the claim.
		const correlationID = "fedcba9876543210fedcba9876543210"
		operationCtx := withShipmentOperation(ctx, correlationID)
		for _, evidence := range []struct {
			eventType string
			phase     string
		}{
			{"shipment_lifecycle", "intent"},
			{"shipment_status_changed", "applied"},
			{"shipment_lifecycle", "committed"},
		} {
			require.NoError(t, appendItemEventWithActorErr(
				operationCtx, ws, shipment.ID, "test", evidence.eventType, map[string]any{
					"correlation_id": correlationID,
					"operation":      "claim",
					"phase":          evidence.phase,
					"target":         string(ShipmentActive),
					"status":         string(ShipmentActive),
				}))
		}

		_, err = writeShipmentLifecycleJournalForWorkspace(
			ws,
			shipmentLifecycleJournalName(correlationID),
			shipmentLifecycleJournal{
				SchemaVersion:  "shipment-operation/v1",
				CorrelationID:  correlationID,
				Phase:          "intent",
				Operation:      "claim",
				RecoveryPolicy: "rollback",
				ShipmentID:     shipment.ID,
				Target:         string(ShipmentActive),
				Preimage:       preimage,
			},
		)
		require.NoError(t, err)
		require.NoError(t, recoverPendingShipmentOperations(ctx, ws),
			"recovery must finalize the claim from committed evidence")
		recoveredDone, err := loadArtifact(ctx, ws, doneMember.ID)
		require.NoError(t, err)
		assert.Equal(t, models.StatusDone, recoveredDone.Status, "a non-cascaded done member must stay done")
		recoveredChild, err := loadArtifact(ctx, ws, child.ID)
		require.NoError(t, err)
		assert.Equal(t, models.StatusActive, recoveredChild.Status)
	})
}

type claimCascadeRecoveryFixture struct {
	base     *claimMarkerRecoveryFixture
	parentID string
	childID  string
}

// newClaimCascadeRecoveryFixture builds a shipment whose manifest lists a done
// member-parent after its queued child, with a claim journal at intent.
func newClaimCascadeRecoveryFixture(t *testing.T) *claimCascadeRecoveryFixture {
	t.Helper()
	root, ws := setupUR3Workspace(t)
	base := &claimMarkerRecoveryFixture{root: root, workspace: ws}
	t.Cleanup(func() {
		if !base.workspaceClosed {
			require.NoError(t, base.workspace.Close())
		}
	})
	ctx := context.Background()

	parent, err := CreateArtifact(ctx, ws, "Cascade member parent", "feature")
	require.NoError(t, err)
	child, err := CreateArtifact(ctx, ws, "Cascade member child", "task", WithParent(parent.ID))
	require.NoError(t, err)
	shipment, err := CreateShipment(ctx, ws, "Cascade recovery shipment", []string{child.ID, parent.ID})
	require.NoError(t, err)
	parent, err = loadArtifact(ctx, ws, parent.ID)
	require.NoError(t, err)
	parent.Status = models.StatusDone
	require.NoError(t, persistArtifact(ctx, ws, parent, true))
	base.shipmentID = shipment.ID
	shipment, err = loadArtifact(ctx, ws, shipment.ID)
	require.NoError(t, err)
	manifest := NormalizeShipmentItems(shipment)
	require.Equal(t, []string{child.ID, parent.ID}, manifest)

	members := make([]*models.Artifact, 0, len(manifest))
	for _, memberID := range manifest {
		member, loadErr := loadArtifact(ctx, ws, memberID)
		require.NoError(t, loadErr)
		members = append(members, cloneArtifact(member))
	}
	require.Equal(t, models.StatusQueued, members[0].Status)
	require.Equal(t, models.StatusDone, members[1].Status)
	base.journal = shipmentLifecycleJournal{
		SchemaVersion:  "shipment-operation/v1",
		CorrelationID:  "0123456789abcdef0123456789abcdef",
		Phase:          "intent",
		Operation:      "claim",
		RecoveryPolicy: "rollback",
		ShipmentID:     shipment.ID,
		Target:         string(ShipmentActive),
		Preimage: shipmentLifecyclePreimage{
			Shipment: cloneArtifact(shipment),
			Members:  members,
		},
	}
	base.preimageFiles = map[string][]byte{
		shipment.ID: claimMarkerRecoveryArtifactFileBytes(t, ws, shipment.ID),
	}
	for _, member := range members {
		base.preimageFiles[member.ID] = claimMarkerRecoveryArtifactFileBytes(t, ws, member.ID)
	}
	_, err = writeShipmentLifecycleJournalForWorkspace(
		ws,
		shipmentLifecycleJournalName(base.journal.CorrelationID),
		base.journal,
	)
	require.NoError(t, err)
	return &claimCascadeRecoveryFixture{base: base, parentID: parent.ID, childID: child.ID}
}

// persistCrashState simulates a claim crash after the shipment and child
// activated (child carries the journal marker) and the cascade moved the done
// parent to active. A non-empty parentMarker stamps the parent with that marker.
func (f *claimCascadeRecoveryFixture) persistCrashState(t *testing.T, parentMarker string) {
	t.Helper()
	persistClaimMarkerRecoveryState(t, f.base, true, map[string]string{
		f.childID: f.base.shipmentID,
	})
	parent := cloneArtifact(f.base.journal.Preimage.Members[1])
	parent.Status = models.StatusActive
	parent.UpdatedAt = models.NowUTC()
	if parentMarker != "" {
		if parent.CustomFields == nil {
			parent.CustomFields = map[string]any{}
		}
		parent.CustomFields["scheduler_baseline_claim"] = parentMarker
	}
	operationCtx := withShipmentOperation(context.Background(), f.base.journal.CorrelationID)
	require.NoError(t, persistArtifact(operationCtx, f.base.workspace, parent, true))
}
