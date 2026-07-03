package core

// Shipment State Integrity harness (feature 061-F).
//
// 061.002-T — Rollback partial shipment activation: a claim that fails
// mid-flight must restore the shipment and every partially activated item to
// queued, leaving no torn/partial activation behind.
//
// 061.001-T — Clear stale returned-item blocked metadata: when a previously
// blocked item re-enters the backlog, its stale custom_fields.blocked_reason
// must be removed so queued items reflect current availability accurately.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bldb "github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/models"
)

// TestClaimShipment_RollsBackOnMidFlightActivationFailure asserts that when a
// claim activates one item and then fails on a later item, the shipment, the
// partially activated item, and any cascade-activated parent all revert to
// queued (DB and on-disk file agree). (061.002-T)
func TestClaimShipment_RollsBackOnMidFlightActivationFailure(t *testing.T) {
	// Arrange
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	feat, err := CreateArtifact(ctx, ws, "Rollback feature", "feature")
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, feat))
	taskA, err := CreateArtifact(ctx, ws, "Rollback task A", "task", WithParent(feat.ID))
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, taskA))

	shipment, err := CreateShipment(ctx, ws, "Rollback shipment", []string{taskA.ID})
	require.NoError(t, err)

	// Inject an unloadable item ID after the valid one to force a mid-flight
	// failure: taskA activates successfully, then the bogus ID fails to load.
	injected, err := GetShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)
	injected.CustomFields["items"] = []string{taskA.ID, "999.999-T"}
	require.NoError(t, persistArtifact(ctx, ws, injected, false))

	// Act
	_, claimErr := ClaimShipment(ctx, ws, shipment.ID)

	// Assert — claim reports an error
	require.Error(t, claimErr, "claim must fail when a manifest item cannot be activated")

	// Assert — shipment restored to queued (DB + file agree)
	dbShipment, err := bldb.GetItem(ctx, ws.DB, shipment.ID)
	require.NoError(t, err)
	assert.Equal(t, models.StatusQueued, dbShipment.Status, "DB: shipment must revert to queued on rollback")
	fileShipment, err := loadArtifact(ctx, ws, shipment.ID)
	require.NoError(t, err)
	assert.Equal(t, models.StatusQueued, fileShipment.Status, "file: shipment must revert to queued on rollback")

	// Assert — the partially activated item reverted to queued (DB + file agree)
	dbTask, err := bldb.GetItem(ctx, ws.DB, taskA.ID)
	require.NoError(t, err)
	assert.Equal(t, models.StatusQueued, dbTask.Status, "DB: partially activated item must revert to queued")
	fileTask, err := loadArtifact(ctx, ws, taskA.ID)
	require.NoError(t, err)
	assert.Equal(t, models.StatusQueued, fileTask.Status, "file: partially activated item must revert to queued")

	// Assert — the cascade-activated parent reverted to queued (DB + file agree)
	dbFeat, err := bldb.GetItem(ctx, ws.DB, feat.ID)
	require.NoError(t, err)
	assert.Equal(t, models.StatusQueued, dbFeat.Status, "DB: cascade-activated parent must revert to queued")
	fileFeat, err := loadArtifact(ctx, ws, feat.ID)
	require.NoError(t, err)
	assert.Equal(t, models.StatusQueued, fileFeat.Status, "file: cascade-activated parent must revert to queued")
}

// TestClaimShipment_SuccessActivatesAllItems guards the normal claim path: a
// claim with no failures activates the shipment and all of its queued items.
// (061.002-T regression guard)
func TestClaimShipment_SuccessActivatesAllItems(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	feat, err := CreateArtifact(ctx, ws, "Happy feature", "feature")
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, feat))
	task, err := CreateArtifact(ctx, ws, "Happy task", "task", WithParent(feat.ID))
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, task))

	shipment, err := CreateShipment(ctx, ws, "Happy shipment", []string{feat.ID, task.ID})
	require.NoError(t, err)

	claimed, err := ClaimShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)
	assert.Equal(t, models.StatusActive, claimed.Status, "shipment must be active after a successful claim")
	// The returned shipment is the already-loaded snapshot (no post-activation
	// read-back); guard that it is still a complete shipment carrying its full
	// manifest so the read-back elimination cannot silently drop items.
	assert.ElementsMatch(t, []string{feat.ID, task.ID}, NormalizeShipmentItems(claimed),
		"returned shipment must carry its full manifest")

	dbTask, err := bldb.GetItem(ctx, ws.DB, task.ID)
	require.NoError(t, err)
	assert.Equal(t, models.StatusActive, dbTask.Status, "queued item must be active after a successful claim")
}

// TestUpdateArtifact_ClearsStaleBlockedReasonOnReentry asserts that re-activating
// a blocked item via UpdateArtifact (the hook-validated blocked->active
// transition an operator uses to pick the item back up) clears the stale
// blocked_reason metadata (DB + file agree). (061.001-T)
func TestUpdateArtifact_ClearsStaleBlockedReasonOnReentry(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	feat, err := CreateArtifact(ctx, ws, "Reentry feature", "feature")
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, feat))
	task, err := CreateArtifact(ctx, ws, "Reentry task", "task", WithParent(feat.ID))
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, task))

	shipment, err := CreateShipment(ctx, ws, "Reentry shipment", []string{task.ID})
	require.NoError(t, err)
	_, err = ClaimShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)

	// Return the task blocked → records a blocked_reason.
	require.NoError(t, ReturnBlockedItem(ctx, ws, shipment.ID, task.ID, "waiting on upstream"))
	blocked, err := bldb.GetItem(ctx, ws.DB, task.ID)
	require.NoError(t, err)
	require.Equal(t, models.StatusBlocked, blocked.Status)
	require.Equal(t, "waiting on upstream", blocked.CustomFields["blocked_reason"])

	// Act — the item leaves blocked (blocked->active is the only hook-allowed exit).
	updated, err := UpdateArtifact(ctx, ws, task.ID, map[string]any{"status": string(models.StatusActive)})
	require.NoError(t, err)

	// Assert — returned value cleared the stale reason.
	assert.Equal(t, models.StatusActive, updated.Status)
	_, hasReason := updated.CustomFields["blocked_reason"]
	assert.False(t, hasReason, "blocked_reason must be cleared when an item leaves blocked")

	// Assert — DB + file agree the reason is gone.
	dbTask, err := bldb.GetItem(ctx, ws.DB, task.ID)
	require.NoError(t, err)
	_, dbHasReason := dbTask.CustomFields["blocked_reason"]
	assert.False(t, dbHasReason, "DB: blocked_reason must be cleared")
	fileTask, err := loadArtifact(ctx, ws, task.ID)
	require.NoError(t, err)
	_, fileHasReason := fileTask.CustomFields["blocked_reason"]
	assert.False(t, fileHasReason, "file: blocked_reason must be cleared")
}

// TestSetArtifactStatus_ClearsStaleBlockedReason asserts the shipment-lifecycle
// internal status setter also clears stale blocked metadata when an item leaves
// blocked status (the returnUnreleasedFeatureItems path). (061.001-T)
func TestSetArtifactStatus_ClearsStaleBlockedReason(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	feat, err := CreateArtifact(ctx, ws, "Lifecycle reentry feature", "feature")
	require.NoError(t, err)
	feat.Status = models.StatusBlocked
	if feat.CustomFields == nil {
		feat.CustomFields = map[string]any{}
	}
	feat.CustomFields["blocked_reason"] = "stale lifecycle reason"
	require.NoError(t, persistArtifact(ctx, ws, feat, true))

	requeued, err := setArtifactStatus(ctx, ws, feat.ID, models.StatusQueued, "returned to backlog after release")
	require.NoError(t, err)

	assert.Equal(t, models.StatusQueued, requeued.Status)
	_, hasReason := requeued.CustomFields["blocked_reason"]
	assert.False(t, hasReason, "blocked_reason must be cleared by setArtifactStatus when leaving blocked")
}

// TestCascadeParentStatus_ClearsStaleBlockedReason asserts that when a parent's
// rolled-up status is recomputed away from blocked by the cascade (its last
// blocked child is unblocked), the parent's stale blocked_reason is also
// cleared. (061.001-T — cascade choke point)
func TestCascadeParentStatus_ClearsStaleBlockedReason(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	feat, err := CreateArtifact(ctx, ws, "Cascade reentry feature", "feature")
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, feat))
	task, err := CreateArtifact(ctx, ws, "Cascade reentry task", "task", WithParent(feat.ID))
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, task))

	// Force the parent into a blocked state carrying a stale reason.
	feat.Status = models.StatusBlocked
	if feat.CustomFields == nil {
		feat.CustomFields = map[string]any{}
	}
	feat.CustomFields["blocked_reason"] = "stale parent reason"
	require.NoError(t, persistArtifact(ctx, ws, feat, true))

	// Activating the child recomputes the parent off blocked via the cascade.
	_, err = setArtifactStatus(ctx, ws, task.ID, models.StatusActive, "child picked up")
	require.NoError(t, err)

	dbFeat, err := bldb.GetItem(ctx, ws.DB, feat.ID)
	require.NoError(t, err)
	require.NotEqual(t, models.StatusBlocked, dbFeat.Status, "parent must have left blocked via cascade")
	_, hasReason := dbFeat.CustomFields["blocked_reason"]
	assert.False(t, hasReason, "cascade must clear the parent's stale blocked_reason")
}

// TestUpdateArtifact_KeepsBlockedReasonWhileStillBlocked asserts the cleanup is
// scoped: an item that remains blocked keeps its blocked_reason. (061.001-T
// negative guard)
func TestUpdateArtifact_KeepsBlockedReasonWhileStillBlocked(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	feat, err := CreateArtifact(ctx, ws, "Still blocked feature", "feature")
	require.NoError(t, err)
	feat.Status = models.StatusBlocked
	if feat.CustomFields == nil {
		feat.CustomFields = map[string]any{}
	}
	feat.CustomFields["blocked_reason"] = "still waiting"
	require.NoError(t, persistArtifact(ctx, ws, feat, true))

	// A title-only update that does not change status must not strip the reason.
	updated, err := UpdateArtifact(ctx, ws, feat.ID, map[string]any{"title": "Still blocked feature (renamed)"})
	require.NoError(t, err)
	assert.Equal(t, models.StatusBlocked, updated.Status)
	assert.Equal(t, "still waiting", updated.CustomFields["blocked_reason"],
		"blocked_reason must persist while the item is still blocked")
}
