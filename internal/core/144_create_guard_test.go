package core_test

// 144.003-T (U3) RED harness: CreateArtifact must refuse an initial status of
// "shipped" for artifact_type "shipment". Compiles immediately (sentinel defined
// in internal/errors) but FAILS until U4 adds the guard to CreateArtifact.

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

func TestCreateArtifact_Shipment_InitialStatusShipped_Refused(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	// Attempting to create a shipment with status: shipped must be refused.
	_, err := core.CreateArtifact(ctx, ws, "Shipped-from-birth shipment", "shipment",
		core.WithStatus("shipped"))
	require.Error(t, err, "creating a shipment with initial status shipped must be refused")
	assert.True(t, errors.Is(err, blerrors.ErrShipmentShippedRequiresEnvelope),
		"want ErrShipmentShippedRequiresEnvelope; got %v", err)
}

func TestCreateArtifact_Shipment_InitialStatusQueued_Unaffected(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	// Creating a shipment at queued (the default) must succeed.
	art, err := core.CreateArtifact(ctx, ws, "Queued shipment", "shipment")
	require.NoError(t, err, "creating a shipment at queued must succeed")
	assert.Equal(t, "queued", string(art.Status))
}

func TestCreateArtifact_NonShipment_InitialStatusShipped_Unaffected(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	// Guard 1 must not block non-shipment artifact types.
	feat, err := core.CreateArtifact(ctx, ws, "Parent feature", "feature")
	require.NoError(t, err)

	_, err = core.CreateArtifact(ctx, ws, "Task at shipped-like status", "task",
		core.WithParent(feat.ID), core.WithStatus("done"))
	require.NoError(t, err, "guard must not block non-shipment creates with non-queued status")
}
