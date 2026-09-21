package core

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/models"
)

func TestUNonMemberRollupSafe_ShipWritesOnlyMembersAndPreservesListedFeatureControl(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	nonMember, err := CreateArtifact(ctx, ws, "Non-member rollup feature", "feature")
	require.NoError(t, err)
	first, err := CreateArtifact(ctx, ws, "Non-member first child", "task", WithParent(nonMember.ID))
	require.NoError(t, err)
	second, err := CreateArtifact(ctx, ws, "Non-member second child", "task", WithParent(nonMember.ID))
	require.NoError(t, err)
	shipment, err := CreateShipment(ctx, ws, "Non-member rollup shipment", []string{first.ID, second.ID})
	require.NoError(t, err)
	_, err = ClaimShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)
	forceFlatScopeStatus(t, ws, nonMember.ID, models.StatusQueued)
	baselineEvents := flatScopeEventCount(t, ws, nonMember.ID)

	result, err := ShipShipment(ctx, ws, shipment.ID, nil)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, models.StatusQueued, flatScopeStatus(t, ws, nonMember.ID),
		"SCOPE-RED-E: non-member ancestor status must remain unchanged")
	reasons := flatScopeStatusReasonsSince(t, ws, nonMember.ID, baselineEvents)
	assert.NotContains(t, reasons, "child status rollup",
		"SCOPE-RED-E: ship must not roll up an unlisted ancestor")
	assert.NotContains(t, reasons, "reverted unintended rollup from partial-feature ship",
		"SCOPE-RED-E: ship must not restore an ancestor it never had authority to snapshot")

	listedFeature, err := CreateArtifact(ctx, ws, "Listed feature control", "feature")
	require.NoError(t, err)
	listedTask, err := CreateArtifact(ctx, ws, "Listed feature control task", "task", WithParent(listedFeature.ID))
	require.NoError(t, err)
	controlShipment, err := CreateShipment(ctx, ws, "Listed feature control shipment", []string{listedFeature.ID, listedTask.ID})
	require.NoError(t, err)
	_, err = ClaimShipment(ctx, ws, controlShipment.ID)
	require.NoError(t, err)
	controlResult, err := ShipShipment(ctx, ws, controlShipment.ID, nil)
	require.NoError(t, err)
	assert.Contains(t, controlResult.ArchivedIDs, listedFeature.ID,
		"SCOPE-RED-E control: explicitly-listed feature must retain governed completion")
}

func TestUNonMemberRollupSafe_ConcurrentAncestorMutationSurvivesSuccessfulShip(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	ancestor, err := CreateArtifact(ctx, ws, "Concurrent success ancestor", "feature")
	require.NoError(t, err)
	first, err := CreateArtifact(ctx, ws, "Concurrent success first", "task", WithParent(ancestor.ID))
	require.NoError(t, err)
	second, err := CreateArtifact(ctx, ws, "Concurrent success second", "task", WithParent(ancestor.ID))
	require.NoError(t, err)
	shipment, err := CreateShipment(ctx, ws, "Concurrent success shipment", []string{first.ID, second.ID})
	require.NoError(t, err)
	_, err = ClaimShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)
	forceFlatScopeStatus(t, ws, ancestor.ID, models.StatusQueued)

	mutated := false
	persistArtifactPreLockHook = func(id string) {
		if id != first.ID || mutated {
			return
		}
		mutated = true
		forceFlatScopeStatus(t, ws, ancestor.ID, models.StatusActive)
	}
	t.Cleanup(func() { persistArtifactPreLockHook = nil })

	_, err = ShipShipment(ctx, ws, shipment.ID, nil)
	require.NoError(t, err)
	require.True(t, mutated, "SCOPE-RED-E fixture must inject mutation at a listed-member boundary")
	assert.Equal(t, models.StatusActive, flatScopeStatus(t, ws, ancestor.ID),
		"SCOPE-RED-E: successful ship must not overwrite a concurrent non-member mutation")
}

func TestUNonMemberRollupSafe_ConcurrentAncestorMutationSurvivesRollback(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	ancestor, err := CreateArtifact(ctx, ws, "Concurrent rollback ancestor", "feature")
	require.NoError(t, err)
	first, err := CreateArtifact(ctx, ws, "Concurrent rollback first", "task", WithParent(ancestor.ID))
	require.NoError(t, err)
	second, err := CreateArtifact(ctx, ws, "Concurrent rollback second", "task", WithParent(ancestor.ID))
	require.NoError(t, err)
	shipment, err := CreateShipment(ctx, ws, "Concurrent rollback shipment", []string{first.ID, second.ID})
	require.NoError(t, err)
	_, err = ClaimShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)
	forceFlatScopeStatus(t, ws, ancestor.ID, models.StatusQueued)

	mutated := false
	persistArtifactPreLockHook = func(id string) {
		if id != first.ID || mutated {
			return
		}
		mutated = true
		forceFlatScopeStatus(t, ws, ancestor.ID, models.StatusActive)
	}
	injectedErr := errors.New("SCOPE-RED-E injected shipment persist failure")
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
	require.True(t, mutated, "SCOPE-RED-E fixture must inject mutation before compensation")
	assert.Equal(t, models.StatusActive, flatScopeStatus(t, ws, ancestor.ID),
		"SCOPE-RED-E: rollback must not overwrite a concurrent non-member mutation")
}
