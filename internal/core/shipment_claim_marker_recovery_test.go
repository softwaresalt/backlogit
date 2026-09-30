package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/models"
)

func TestClaimMarkerRecoveryRed_CrashAfterMarkingRollsBack(t *testing.T) {
	fixture := newClaimMarkerRecoveryFixture(t)
	persistClaimMarkerRecoveryState(t, fixture, true, map[string]string{
		fixture.journal.Preimage.Members[0].ID: fixture.shipmentID,
		fixture.journal.Preimage.Members[1].ID: fixture.shipmentID,
	})

	candidates, err := memberRecoveryCandidates(fixture.journal, fixture.journal.Preimage.Members[0], nil)
	if assert.NoError(t, err, "claim recovery candidates must be available for a queued preimage") && assert.NotEmpty(t, candidates) {
		lastCandidate := candidates[len(candidates)-1].artifact
		marker, present := lastCandidate.CustomFields["scheduler_baseline_claim"]
		assert.True(t, present, "last claim recovery candidate must include scheduler_baseline_claim")
		assert.Equal(t, fixture.shipmentID, marker, "last claim recovery candidate must carry the journal shipment ID")
	}

	recovered, err := reopenClaimMarkerRecoveryWorkspace(t, fixture)
	if !assert.NoError(t, err, "fresh workspace open must roll back a claim interrupted after marker writes") {
		assert.ErrorIs(t, err, blerrors.ErrShipmentConflict, "the expected RED is the current marker recovery conflict")
		return
	}
	if !assert.NotNil(t, recovered, "fresh workspace open must return the recovered workspace") {
		return
	}

	assertClaimMarkerRecoveryPreimages(t, recovered, fixture)
	_, err = ClaimShipment(context.Background(), recovered, fixture.shipmentID)
	assert.NoError(t, err, "a claim must succeed after recovery restores the shipment preimage")
}

func TestClaimMarkerRecoveryRed_DoubleFaultPartialCompensationConverges(t *testing.T) {
	fixture := newClaimMarkerRecoveryFixture(t)
	persistClaimMarkerRecoveryState(t, fixture, false, map[string]string{
		fixture.journal.Preimage.Members[1].ID: fixture.shipmentID,
	})

	shipment, err := loadArtifact(context.Background(), fixture.workspace, fixture.shipmentID)
	require.NoError(t, err)
	require.Equal(t, models.StatusQueued, shipment.Status, "partial compensation fixture keeps the shipment at its preimage")
	restoredMember, err := loadArtifact(context.Background(), fixture.workspace, fixture.journal.Preimage.Members[0].ID)
	require.NoError(t, err)
	require.Equal(t, models.StatusQueued, restoredMember.Status, "partial compensation fixture restores one member")
	remainingMember, err := loadArtifact(context.Background(), fixture.workspace, fixture.journal.Preimage.Members[1].ID)
	require.NoError(t, err)
	require.Equal(t, models.StatusActive, remainingMember.Status, "partial compensation fixture leaves one member active")

	recovered, err := reopenClaimMarkerRecoveryWorkspace(t, fixture)
	if !assert.NoError(t, err, "fresh workspace open must converge a partially compensated claim") {
		assert.ErrorIs(t, err, blerrors.ErrShipmentConflict, "the expected RED is the current marker recovery conflict")
		return
	}
	if !assert.NotNil(t, recovered, "fresh workspace open must return the recovered workspace") {
		return
	}
	assertClaimMarkerRecoveryPreimages(t, recovered, fixture)
}

func TestClaimMarkerRecoveryCharacterization_DivergedMarkerFailsClosed(t *testing.T) {
	fixture := newClaimMarkerRecoveryFixture(t)
	persistClaimMarkerRecoveryState(t, fixture, true, map[string]string{
		fixture.journal.Preimage.Members[0].ID: "FOREIGN-S",
	})

	recovered, err := reopenClaimMarkerRecoveryWorkspace(t, fixture)
	assert.Nil(t, recovered, "recovery must not return a workspace after a diverged marker")
	assert.ErrorIs(t, err, blerrors.ErrShipmentConflict,
		"a member marker that differs from both its preimage and the journal shipment ID must fail closed")
}

type claimMarkerRecoveryFixture struct {
	root            string
	workspace       *Workspace
	workspaceClosed bool
	journal         shipmentLifecycleJournal
	shipmentID      string
	preimageFiles   map[string][]byte
}

func newClaimMarkerRecoveryFixture(t *testing.T) *claimMarkerRecoveryFixture {
	t.Helper()
	root := t.TempDir()
	storageRoot := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(storageRoot, 0o755))
	require.NoError(t, config.WriteDefaults(storageRoot))

	ctx := context.Background()
	ws, err := NewWorkspace(ctx, root)
	require.NoError(t, err)
	fixture := &claimMarkerRecoveryFixture{
		root:      root,
		workspace: ws,
	}
	t.Cleanup(func() {
		if !fixture.workspaceClosed {
			require.NoError(t, fixture.workspace.Close())
		}
	})

	feature, err := CreateArtifact(ctx, ws, "Claim recovery marker feature", "feature")
	require.NoError(t, err)
	staleMarker, err := CreateArtifact(ctx, ws, "Claim recovery stale marker member", "task", WithParent(feature.ID))
	require.NoError(t, err)
	staleMarker.CustomFields = map[string]any{"scheduler_baseline_claim": "OLD-S", "retained": "preimage"}
	require.NoError(t, persistArtifact(ctx, ws, staleMarker, false))
	emptyFields, err := CreateArtifact(ctx, ws, "Claim recovery nil custom-fields member", "task", WithParent(feature.ID))
	require.NoError(t, err)

	shipment, err := CreateShipment(ctx, ws, "Claim recovery marker shipment", []string{staleMarker.ID, emptyFields.ID})
	require.NoError(t, err)
	fixture.shipmentID = shipment.ID
	require.Equal(t, []string{staleMarker.ID, emptyFields.ID}, NormalizeShipmentItems(shipment))

	members := make([]*models.Artifact, 0, 2)
	for _, memberID := range NormalizeShipmentItems(shipment) {
		member, loadErr := loadArtifact(ctx, ws, memberID)
		require.NoError(t, loadErr)
		members = append(members, cloneArtifact(member))
	}
	fixture.journal = shipmentLifecycleJournal{
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
	fixture.preimageFiles = make(map[string][]byte, len(members)+1)
	fixture.preimageFiles[shipment.ID] = claimMarkerRecoveryArtifactFileBytes(t, ws, shipment.ID)
	for _, member := range members {
		fixture.preimageFiles[member.ID] = claimMarkerRecoveryArtifactFileBytes(t, ws, member.ID)
	}
	_, err = writeShipmentLifecycleJournalForWorkspace(
		ws,
		shipmentLifecycleJournalName(fixture.journal.CorrelationID),
		fixture.journal,
	)
	require.NoError(t, err)
	return fixture
}

func persistClaimMarkerRecoveryState(
	t *testing.T,
	fixture *claimMarkerRecoveryFixture,
	activeShipment bool,
	memberMarkers map[string]string,
) {
	t.Helper()
	operationCtx := withShipmentOperation(context.Background(), fixture.journal.CorrelationID)
	if activeShipment {
		shipment := cloneArtifact(fixture.journal.Preimage.Shipment)
		shipment.Status = models.StatusActive
		shipment.UpdatedAt = models.NowUTC()
		governedCtx := context.WithValue(operationCtx, artifactWriteEnvelopeContextKey{}, artifactWriteEnvelope{
			correlationID:                 fixture.journal.CorrelationID,
			operation:                     "move_shipment_status",
			changes:                       map[string]any{"status": string(ShipmentActive)},
			allowGovernedShipmentMutation: true,
		})
		require.NoError(t, persistArtifact(governedCtx, fixture.workspace, shipment, true))
	}

	for _, preimage := range fixture.journal.Preimage.Members {
		marker, marked := memberMarkers[preimage.ID]
		if !marked {
			continue
		}
		member := cloneArtifact(preimage)
		member.Status = models.StatusActive
		member.UpdatedAt = models.NowUTC()
		if member.CustomFields == nil {
			member.CustomFields = map[string]any{}
		}
		member.CustomFields["scheduler_baseline_claim"] = marker
		require.NoError(t, persistArtifact(operationCtx, fixture.workspace, member, true))
	}
}

func reopenClaimMarkerRecoveryWorkspace(t *testing.T, fixture *claimMarkerRecoveryFixture) (*Workspace, error) {
	t.Helper()
	if !fixture.workspaceClosed {
		require.NoError(t, fixture.workspace.Close())
		fixture.workspaceClosed = true
	}
	recovered, err := NewWorkspace(context.Background(), fixture.root)
	if recovered != nil {
		t.Cleanup(func() {
			require.NoError(t, recovered.Close())
		})
	}
	return recovered, err
}

func assertClaimMarkerRecoveryPreimages(
	t *testing.T,
	ws *Workspace,
	fixture *claimMarkerRecoveryFixture,
) {
	t.Helper()
	for artifactID, want := range fixture.preimageFiles {
		assert.Equal(t, want, claimMarkerRecoveryArtifactFileBytes(t, ws, artifactID),
			"recovery must restore artifact %s byte-for-byte to its preimage", artifactID)
	}

	staleMember, err := loadArtifact(context.Background(), ws, fixture.journal.Preimage.Members[0].ID)
	require.NoError(t, err)
	marker, present := staleMember.CustomFields["scheduler_baseline_claim"]
	assert.True(t, present, "recovery must restore the stale preimage marker")
	assert.Equal(t, "OLD-S", marker, "recovery must restore the stale preimage marker value")

	nilFieldsMember, err := loadArtifact(context.Background(), ws, fixture.journal.Preimage.Members[1].ID)
	require.NoError(t, err)
	assert.Nil(t, nilFieldsMember.CustomFields, "recovery must restore nil custom_fields when nil was in the preimage")
}

func claimMarkerRecoveryArtifactFileBytes(t *testing.T, ws *Workspace, artifactID string) []byte {
	t.Helper()
	path, err := FindArtifactPath(context.Background(), ws, artifactID)
	require.NoError(t, err)
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	return content
}
