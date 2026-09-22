package core

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/models"
)

func TestURollbackScopeFlat_LockSetExcludesUnlistedDescendant(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	feature, err := CreateArtifact(ctx, ws, "Rollback flat feature", "feature")
	require.NoError(t, err)
	listed, err := CreateArtifact(ctx, ws, "Rollback listed child", "task", WithParent(feature.ID))
	require.NoError(t, err)
	unlisted, err := CreateArtifact(ctx, ws, "Rollback unlisted child", "task", WithParent(feature.ID))
	require.NoError(t, err)
	shipment, err := CreateShipment(ctx, ws, "Rollback flat shipment", []string{feature.ID, listed.ID})
	require.NoError(t, err)
	_, err = ClaimShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)

	probed := false
	unlistedLocked := false
	persistArtifactPreLockHook = func(id string) {
		if id != listed.ID || probed {
			return
		}
		probed = true
		unlistedLocked = flatScopeLockProbe(t, ws, unlisted.ID)
	}
	t.Cleanup(func() { persistArtifactPreLockHook = nil })

	_, err = ShipShipment(ctx, ws, shipment.ID, nil)
	require.NoError(t, err)
	require.True(t, probed, "SCOPE-RED-C fixture must probe during listed-member persistence")
	assert.False(t, unlistedLocked,
		"SCOPE-RED-C: rollback lock set must exclude an unlisted descendant")
}

func TestURollbackScopeFlat_LockSetExcludesUnlistedAncestor(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	ancestor, err := CreateArtifact(ctx, ws, "Rollback unlisted ancestor", "feature")
	require.NoError(t, err)
	listed, err := CreateArtifact(ctx, ws, "Rollback listed descendant", "task", WithParent(ancestor.ID))
	require.NoError(t, err)
	shipment, err := CreateShipment(ctx, ws, "Rollback ancestor shipment", []string{listed.ID})
	require.NoError(t, err)
	_, err = ClaimShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)

	probed := false
	ancestorLocked := false
	persistArtifactPreLockHook = func(id string) {
		if id != listed.ID || probed {
			return
		}
		probed = true
		ancestorLocked = flatScopeLockProbe(t, ws, ancestor.ID)
	}
	t.Cleanup(func() { persistArtifactPreLockHook = nil })

	_, err = ShipShipment(ctx, ws, shipment.ID, nil)
	require.NoError(t, err)
	require.True(t, probed, "SCOPE-RED-C fixture must probe during listed-member persistence")
	assert.False(t, ancestorLocked,
		"SCOPE-RED-C: rollback lock set must exclude an unlisted ancestor")
}

func TestURollbackScopeFlat_RestoreCannotOverwriteUnlistedDescendant(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	feature, err := CreateArtifact(ctx, ws, "Rollback restore feature", "feature")
	require.NoError(t, err)
	listed, err := CreateArtifact(ctx, ws, "Rollback restore listed child", "task", WithParent(feature.ID))
	require.NoError(t, err)
	unlisted, err := CreateArtifact(ctx, ws, "Rollback restore unlisted child", "task", WithParent(feature.ID))
	require.NoError(t, err)
	shipment, err := CreateShipment(ctx, ws, "Rollback restore shipment", []string{feature.ID, listed.ID})
	require.NoError(t, err)
	_, err = ClaimShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)

	mutated := false
	persistArtifactPreLockHook = func(id string) {
		if id != shipment.ID || mutated {
			return
		}
		mutated = true
		forceFlatScopeStatus(t, ws, unlisted.ID, models.StatusReview)
	}
	injectedErr := errors.New("SCOPE-RED-C injected shipment persist failure")
	realWrite := persistArtifactWriteFn
	persistArtifactWriteFn = func(artifact *models.Artifact, filePath string, durable bool) error {
		if artifact.ID == shipment.ID {
			return injectedErr
		}
		return realWrite(artifact, filePath, durable)
	}
	t.Cleanup(func() {
		persistArtifactPreLockHook = nil
		persistArtifactWriteFn = realWrite
	})

	result, err := ShipShipment(ctx, ws, shipment.ID, nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, injectedErr)
	assert.Nil(t, result)
	require.True(t, mutated, "SCOPE-RED-C fixture must mutate the unlisted descendant after snapshot")
	assert.Equal(t, models.StatusReview, flatScopeStatus(t, ws, unlisted.ID),
		"SCOPE-RED-C: restore must not overwrite an unlisted descendant absent from {shipment}+manifest")
}
