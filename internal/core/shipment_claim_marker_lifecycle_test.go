package core

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bldb "github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/models"
)

// u4ActiveShipmentID returns the ID of the sole active shipment recorded in the
// index, or "" when no shipment is active.
func u4ActiveShipmentID(ctx context.Context, t *testing.T, ws *Workspace) string {
	t.Helper()
	shipments, err := bldb.QueryItems(ctx, ws.DB, bldb.QueryFilters{Type: "shipment", Status: string(models.StatusActive)})
	require.NoError(t, err)
	require.LessOrEqual(t, len(shipments), 1, "at most one shipment may be active")
	if len(shipments) == 0 {
		return ""
	}
	return shipments[0].ID
}

// u4IsClaimActivated applies the three-part option A predicate: the item is
// active, its scheduler_baseline_claim equals the currently active shipment ID,
// and the item is listed in that shipment's current items manifest.
func u4IsClaimActivated(ctx context.Context, t *testing.T, ws *Workspace, itemID string) bool {
	t.Helper()
	activeID := u4ActiveShipmentID(ctx, t, ws)
	if activeID == "" {
		return false
	}
	item, err := loadArtifact(ctx, ws, itemID)
	require.NoError(t, err)
	if item.Status != models.StatusActive {
		return false
	}
	marker, _ := item.CustomFields[schedulerBaselineClaimKey].(string)
	if marker != activeID {
		return false
	}
	shipment, err := loadArtifact(ctx, ws, activeID)
	require.NoError(t, err)
	return containsString(NormalizeShipmentItems(shipment), itemID)
}

// u4RequireMarker asserts the marker value on both the Markdown frontmatter
// and the index row.
func u4RequireMarker(ctx context.Context, t *testing.T, ws *Workspace, itemID, want string) {
	t.Helper()
	path, err := FindArtifactPath(ctx, ws, itemID)
	require.NoError(t, err)
	frontmatter, _, err := parseFile(path)
	require.NoError(t, err)
	require.Equal(t, want, frontmatter.CustomFields[schedulerBaselineClaimKey],
		"frontmatter marker for %s", itemID)

	indexed, err := bldb.GetItem(ctx, ws.DB, itemID)
	require.NoError(t, err)
	require.Equal(t, want, indexed.CustomFields[schedulerBaselineClaimKey],
		"index marker for %s", itemID)
}

func u4NewClaimedShipment(ctx context.Context, t *testing.T, ws *Workspace, title string) (*models.Artifact, []string) {
	t.Helper()
	feature, err := CreateArtifact(ctx, ws, title+" feature", "feature")
	require.NoError(t, err)
	member1, err := CreateArtifact(ctx, ws, title+" member 1", "task", WithParent(feature.ID))
	require.NoError(t, err)
	member2, err := CreateArtifact(ctx, ws, title+" member 2", "task", WithParent(feature.ID))
	require.NoError(t, err)

	memberIDs := []string{member1.ID, member2.ID}
	shipment, err := CreateShipment(ctx, ws, title+" shipment", memberIDs)
	require.NoError(t, err)
	_, err = ClaimShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)
	return shipment, memberIDs
}

func TestU4_OptionALifecycleInvariant(t *testing.T) {
	ctx := context.Background()
	ws := setupShipmentWorkspace(t)
	shipment, memberIDs := u4NewClaimedShipment(ctx, t, ws, "U4 lifecycle")
	returnedID, keptID := memberIDs[0], memberIDs[1]

	requireAllMarked := func(stage string, ids ...string) {
		t.Helper()
		for _, id := range ids {
			u4RequireMarker(ctx, t, ws, id, shipment.ID)
			t.Logf("%s: marker intact for %s", stage, id)
		}
	}

	// claim
	requireAllMarked("claim", memberIDs...)
	for _, id := range memberIDs {
		assert.True(t, u4IsClaimActivated(ctx, t, ws, id), "claimed member %s must be claim-activated", id)
	}

	// block
	_, err := BlockShipment(ctx, ws, shipment.ID, BlockOptions{Reason: "U4 lifecycle block", BlockedBy: "u4-test"})
	require.NoError(t, err)
	requireAllMarked("block", memberIDs...)
	assert.Empty(t, u4ActiveShipmentID(ctx, t, ws), "a blocked shipment is not the active shipment")
	for _, id := range memberIDs {
		assert.False(t, u4IsClaimActivated(ctx, t, ws, id),
			"no member is claim-activated while no shipment is active (%s)", id)
	}

	// unblock(active)
	_, err = UnblockShipment(ctx, ws, shipment.ID, UnblockOptions{
		Target: ShipmentActive, Confirm: true, UnblockedBy: "u4-test",
	})
	require.NoError(t, err)
	requireAllMarked("unblock", memberIDs...)
	for _, id := range memberIDs {
		restored, loadErr := loadArtifact(ctx, ws, id)
		require.NoError(t, loadErr)
		assert.Equal(t, models.StatusActive, restored.Status, "unblock(active) must restore member %s", id)
		assert.True(t, u4IsClaimActivated(ctx, t, ws, id), "restored member %s must be claim-activated", id)
	}

	// ReturnBlockedItem of one member
	require.NoError(t, ReturnBlockedItem(ctx, ws, shipment.ID, returnedID, "U4 returned member"))
	requireAllMarked("return", memberIDs...)

	returned, err := loadArtifact(ctx, ws, returnedID)
	require.NoError(t, err)
	assert.Equal(t, models.StatusBlocked, returned.Status)
	updatedShipment, err := loadArtifact(ctx, ws, shipment.ID)
	require.NoError(t, err)
	assert.NotContains(t, NormalizeShipmentItems(updatedShipment), returnedID,
		"the returned member must leave the current items manifest")
	assert.False(t, u4IsClaimActivated(ctx, t, ws, returnedID),
		"the returned (blocked, out-of-manifest) member must not be claim-activated")
	assert.True(t, u4IsClaimActivated(ctx, t, ws, keptID),
		"the member that stayed in the manifest remains claim-activated")
}

func TestU4_TerminalAndGenericPathsKeepMarker(t *testing.T) {
	ctx := context.Background()

	t.Run("ShipShipment", func(t *testing.T) {
		ws := setupShipmentWorkspace(t)
		shipment, memberIDs := u4NewClaimedShipment(ctx, t, ws, "U4 ship")

		_, err := ShipShipment(ctx, ws, shipment.ID, &CommitMetadata{
			SHA: "deadbeefcafebabe", Message: "merge: u4", Author: "tester@example.com",
		})
		require.NoError(t, err)

		for _, id := range memberIDs {
			u4RequireMarker(ctx, t, ws, id, shipment.ID)
			assert.False(t, u4IsClaimActivated(ctx, t, ws, id),
				"after shipping no active shipment exists, so %s is not claim-activated", id)
		}
		assert.Empty(t, u4ActiveShipmentID(ctx, t, ws))
	})

	t.Run("ShipmentAbandoned", func(t *testing.T) {
		ws := setupShipmentWorkspace(t)
		shipment, memberIDs := u4NewClaimedShipment(ctx, t, ws, "U4 abandon")

		require.NoError(t, MoveShipmentStatus(ctx, ws, shipment.ID, ShipmentAbandoned))

		for _, id := range memberIDs {
			u4RequireMarker(ctx, t, ws, id, shipment.ID)
			member, loadErr := loadArtifact(ctx, ws, id)
			require.NoError(t, loadErr)
			assert.Equal(t, shipment.ID, member.CustomFields[schedulerBaselineClaimKey])
			assert.False(t, u4IsClaimActivated(ctx, t, ws, id),
				"after abandonment no active shipment exists, so %s is not claim-activated", id)
		}
		assert.Empty(t, u4ActiveShipmentID(ctx, t, ws))
	})

	t.Run("GenericUpdateAndMove", func(t *testing.T) {
		ws := setupShipmentWorkspace(t)
		shipment, memberIDs := u4NewClaimedShipment(ctx, t, ws, "U4 generic")
		target := memberIDs[0]

		updated, err := UpdateArtifact(ctx, ws, target, map[string]any{"priority": "low"})
		require.NoError(t, err)
		assert.Equal(t, "low", updated.Priority, "the generic update must take effect")
		u4RequireMarker(ctx, t, ws, target, shipment.ID)

		moved, err := setArtifactStatus(ctx, ws, target, models.StatusBlocked, "u4 generic move")
		require.NoError(t, err)
		assert.Equal(t, models.StatusBlocked, moved.Status)
		u4RequireMarker(ctx, t, ws, target, shipment.ID)
		assert.False(t, u4IsClaimActivated(ctx, t, ws, target), "a blocked member is not claim-activated")

		moved, err = setArtifactStatus(ctx, ws, target, models.StatusActive, "u4 generic move back")
		require.NoError(t, err)
		assert.Equal(t, models.StatusActive, moved.Status)
		u4RequireMarker(ctx, t, ws, target, shipment.ID)
		assert.True(t, u4IsClaimActivated(ctx, t, ws, target), "the restored marked member is claim-activated")

		// Marker leg of the predicate: a stale marker must not classify as claim-activated.
		other, err := loadArtifact(ctx, ws, memberIDs[1])
		require.NoError(t, err)
		other.CustomFields[schedulerBaselineClaimKey] = "OLD-S"
		require.NoError(t, persistArtifact(ctx, ws, other, false))
		assert.False(t, u4IsClaimActivated(ctx, t, ws, other.ID),
			"an active in-manifest member with a stale marker is not claim-activated")
	})
}
