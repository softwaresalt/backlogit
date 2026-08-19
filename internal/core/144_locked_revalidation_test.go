package core

// 144.010-T (U10) RED harness: updateArtifactUngated must refuse a shipment
// queued→shipped transition at the locked-state write path, independently of
// the cheap unlocked-peek guard in UpdateArtifactWithGate. Tests compile
// immediately (sentinel defined in U2 errors.go) but FAIL until U11 adds the
// authoritative check inside updateArtifactUngated.

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/models"
)

func TestUpdateArtifactUngated_ShipmentToShipped_Refused_LockedPath(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	// Create a shipment at queued.
	shipment, err := CreateShipment(ctx, ws, "Locked-path guard test shipment", nil)
	require.NoError(t, err)

	// Drive the locked write path directly via updateArtifactUngated.
	// Pre-implementation: no refusal → test fails on require.Error below.
	// Post-U11: refusal with ErrShipmentShippedRequiresEnvelope → test passes.
	_, err = updateArtifactUngated(ctx, ws, shipment.ID, map[string]any{
		"status": string(ShipmentShipped),
	})
	require.Error(t, err, "updateArtifactUngated must refuse a shipment queued→shipped transition")
	assert.True(t, errors.Is(err, blerrors.ErrShipmentShippedRequiresEnvelope),
		"want ErrShipmentShippedRequiresEnvelope from the locked path; got %v", err)
}

func TestUpdateArtifactUngated_ShipmentToShipped_LeavesArtifactUnmutated(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	shipment, err := CreateShipment(ctx, ws, "Immutable-on-refuse shipment", nil)
	require.NoError(t, err)

	path, err := FindArtifactPath(ctx, ws, shipment.ID)
	require.NoError(t, err)
	before, err := os.ReadFile(path)
	require.NoError(t, err)

	// Attempt the refused transition.
	_, _ = updateArtifactUngated(ctx, ws, shipment.ID, map[string]any{
		"status": string(ShipmentShipped),
	})

	// Regardless of implementation state, if a refusal occurred, the durable
	// artifact must be unmutated. After U11, the refusal is explicit.
	after, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, string(before), string(after),
		"a refused updateArtifactUngated must not write to the durable artifact")
}

func TestUpdateArtifactUngated_NonShipmentTask_Unaffected(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	feat, err := CreateArtifact(ctx, ws, "Ungated-task-feature", "feature")
	require.NoError(t, err)
	task, err := CreateArtifact(ctx, ws, "Ungated-task", "task", WithParent(feat.ID))
	require.NoError(t, err)

	// updateArtifactUngated must not block a non-shipment task update.
	updated, err := updateArtifactUngated(ctx, ws, task.ID, map[string]any{"status": "active"})
	require.NoError(t, err, "updateArtifactUngated must not block a non-shipment task transition")
	require.NotNil(t, updated)
	assert.Equal(t, models.StatusActive, updated.Status)
}
