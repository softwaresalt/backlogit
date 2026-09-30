package core

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/models"
)

func TestClaimMarkerRed_ClaimMarksQueuedMembers(t *testing.T) {
	ctx := context.Background()
	ws := setupShipmentWorkspace(t)
	feature, err := CreateArtifact(ctx, ws, "Claim marker feature", "feature")
	require.NoError(t, err)

	staleMarker, err := CreateArtifact(ctx, ws, "Claim marker stale member", "task", WithParent(feature.ID))
	require.NoError(t, err)
	staleMarker.CustomFields = map[string]any{"scheduler_baseline_claim": "OLD-S"}
	require.NoError(t, persistArtifact(ctx, ws, staleMarker, false))

	customFields, err := CreateArtifact(ctx, ws, "Claim marker custom-fields member", "task", WithParent(feature.ID))
	require.NoError(t, err)
	customFields.CustomFields = map[string]any{"retained": "value", "another_key": "unchanged"}
	require.NoError(t, persistArtifact(ctx, ws, customFields, false))

	emptyFields, err := CreateArtifact(ctx, ws, "Claim marker empty-fields member", "task", WithParent(feature.ID))
	require.NoError(t, err)

	activeMember, err := CreateArtifact(ctx, ws, "Claim marker active member", "task", WithParent(feature.ID))
	require.NoError(t, err)
	activeMember, err = UpdateArtifact(ctx, ws, activeMember.ID, map[string]any{"status": string(models.StatusActive)})
	require.NoError(t, err)
	activeMember.CustomFields = map[string]any{"scheduler_baseline_claim": "OLD-A", "retained": "active"}
	require.NoError(t, persistArtifact(ctx, ws, activeMember, false))

	doneMember, err := CreateArtifact(ctx, ws, "Claim marker done member", "task", WithParent(feature.ID))
	require.NoError(t, err)
	doneMember.Status = models.StatusDone
	doneMember.CustomFields = map[string]any{"scheduler_baseline_claim": "OLD-D", "retained": "done"}
	require.NoError(t, persistArtifact(ctx, ws, doneMember, false))

	memberIDs := []string{staleMarker.ID, customFields.ID, emptyFields.ID, activeMember.ID, doneMember.ID}
	shipment, err := CreateShipment(ctx, ws, "Claim marker shipment", memberIDs)
	require.NoError(t, err)

	// Seed the normalized hierarchy path before checking claim-only mutations.
	shipment.HierarchyPath = "001"
	require.NoError(t, persistArtifact(ctx, ws, shipment, false))
	shipmentPreimage, err := GetShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)
	nonQueuedPreimages := map[string][]byte{
		activeMember.ID: claimMarkerArtifactFileBytes(t, ws, activeMember.ID),
		doneMember.ID:   claimMarkerArtifactFileBytes(t, ws, doneMember.ID),
	}

	_, err = ClaimShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)

	for _, memberID := range []string{staleMarker.ID, customFields.ID, emptyFields.ID} {
		member, loadErr := loadArtifact(ctx, ws, memberID)
		require.NoError(t, loadErr)
		assert.Equal(t, models.StatusActive, member.Status, "queued member %s must become active", memberID)
		marker, present := member.CustomFields["scheduler_baseline_claim"]
		assert.True(t, present, "ClaimShipment must write scheduler_baseline_claim for queued member %s", memberID)
		assert.Equal(t, shipment.ID, marker, "queued member %s must carry the claiming shipment ID", memberID)
	}

	staleAfter, err := loadArtifact(ctx, ws, staleMarker.ID)
	require.NoError(t, err)
	marker, present := staleAfter.CustomFields["scheduler_baseline_claim"]
	assert.True(t, present, "re-claim must retain the scheduler_baseline_claim key")
	assert.Equal(t, shipment.ID, marker, "re-claim must overwrite a stale foreign marker")

	customFieldsAfter, err := loadArtifact(ctx, ws, customFields.ID)
	require.NoError(t, err)
	for key, want := range map[string]any{"retained": "value", "another_key": "unchanged"} {
		got, present := customFieldsAfter.CustomFields[key]
		assert.True(t, present, "queued member custom_fields must retain key %q", key)
		assert.Equal(t, want, got, "queued member custom_fields key %q must be unchanged", key)
	}

	for memberID, want := range nonQueuedPreimages {
		assert.Equal(t, want, claimMarkerArtifactFileBytes(t, ws, memberID),
			"non-queued member %s must remain byte-identical to its preimage", memberID)
	}

	shipmentAfter, err := loadArtifact(ctx, ws, shipment.ID)
	require.NoError(t, err)
	normalizeShipmentArtifact(shipmentPreimage)
	normalizeShipmentArtifact(shipmentAfter)
	wantShipment := *shipmentPreimage
	wantShipment.Status = models.StatusActive
	wantShipment.UpdatedAt = shipmentAfter.UpdatedAt
	assert.Equal(t, &wantShipment, shipmentAfter, "shipment may change only status and updated_at during claim")
}

func TestClaimMarkerRed_CascadeOrderingMarksParent(t *testing.T) {
	ctx := context.Background()
	ws := setupShipmentWorkspace(t)
	parent, err := CreateArtifact(ctx, ws, "Claim marker ordering parent", "feature")
	require.NoError(t, err)
	child, err := CreateArtifact(ctx, ws, "Claim marker ordering child", "task", WithParent(parent.ID))
	require.NoError(t, err)

	shipment, err := CreateShipment(ctx, ws, "Claim marker ordering shipment", []string{child.ID, parent.ID})
	require.NoError(t, err)
	require.Equal(t, []string{child.ID, parent.ID}, NormalizeShipmentItems(shipment),
		"fixture must put the queued child before its queued member-parent")

	_, err = ClaimShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)

	for _, memberID := range []string{child.ID, parent.ID} {
		member, loadErr := loadArtifact(ctx, ws, memberID)
		require.NoError(t, loadErr)
		marker, present := member.CustomFields["scheduler_baseline_claim"]
		assert.True(t, present, "claim cascade must mark queued member %s even when the child appears first", memberID)
		assert.Equal(t, shipment.ID, marker, "claim cascade marker for member %s must match the shipment", memberID)
	}
}

func TestClaimMarkerCharacterization_NonClaimStatusWriteByteIdentical(t *testing.T) {
	ctx := context.Background()
	ws := setupShipmentWorkspace(t)
	feature, err := CreateArtifact(ctx, ws, "Non-claim marker characterization feature", "feature")
	require.NoError(t, err)
	member, err := CreateArtifact(ctx, ws, "Non-claim marker characterization member", "task", WithParent(feature.ID))
	require.NoError(t, err)
	member.Status = models.StatusBlocked
	member.CustomFields = map[string]any{"blocked_reason": "stale reason", "retained": "value"}
	require.NoError(t, persistArtifact(ctx, ws, member, false))

	moved, err := setArtifactStatus(ctx, ws, member.ID, models.StatusQueued, "queue move")
	require.NoError(t, err)
	assert.Equal(t, models.StatusQueued, moved.Status)
	_, blockedReasonPresent := moved.CustomFields["blocked_reason"]
	assert.False(t, blockedReasonPresent, "a queue move must clear stale blocked_reason metadata")
	_, markerPresent := moved.CustomFields["scheduler_baseline_claim"]
	assert.False(t, markerPresent, "a non-claim status write must not add scheduler_baseline_claim")

	beforeNoOp, err := FindArtifactPath(ctx, ws, member.ID)
	require.NoError(t, err)
	beforeBytes, err := os.ReadFile(beforeNoOp)
	require.NoError(t, err)

	_, err = setArtifactStatus(ctx, ws, member.ID, models.StatusQueued, "queue move")
	require.NoError(t, err)
	afterBytes := claimMarkerArtifactFileBytes(t, ws, member.ID)
	assert.Equal(t, beforeBytes, afterBytes, "a repeated non-claim queue move must leave frontmatter byte-identical")
}

func claimMarkerArtifactFileBytes(t *testing.T, ws *Workspace, artifactID string) []byte {
	t.Helper()
	path, err := FindArtifactPath(context.Background(), ws, artifactID)
	require.NoError(t, err)
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	return content
}
