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

func TestUNonMemberRollupSafe_MemberAboveNonMemberUsesDirectCompletion(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	memberAbove, err := CreateArtifact(ctx, ws, "Member above non-member", "feature")
	require.NoError(t, err)
	nonMemberSeed, err := CreateArtifact(ctx, ws, "Intervening non-member", "feature")
	require.NoError(t, err)
	adopted, err := AdoptItem(ctx, ws, nonMemberSeed.ID, memberAbove.ID)
	require.NoError(t, err)
	nonMemberID := adopted.NewID
	if nonMemberID == "" {
		nonMemberID = nonMemberSeed.ID
	}
	memberTask, err := CreateArtifact(ctx, ws, "Member below non-member", "task", WithParent(nonMemberID))
	require.NoError(t, err)
	shipment, err := CreateShipment(ctx, ws, "Member above non-member shipment", []string{memberAbove.ID, memberTask.ID})
	require.NoError(t, err)
	_, err = ClaimShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)

	forceFlatScopeStatus(t, ws, memberAbove.ID, models.StatusAccepted)
	forceFlatScopeStatus(t, ws, nonMemberID, models.StatusQueued)
	memberBaseline := flatScopeEventCount(t, ws, memberAbove.ID)
	nonMemberBaseline := flatScopeEventCount(t, ws, nonMemberID)

	result, err := ShipShipment(ctx, ws, shipment.ID, nil)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Contains(t, result.ArchivedIDs, memberAbove.ID,
		"listed feature above an intervening non-member must complete")
	memberReasons := flatScopeStatusReasonsSince(t, ws, memberAbove.ID, memberBaseline)
	assert.Contains(t, memberReasons, "feature released",
		"listed feature must be completed by the direct governed member write")
	assert.NotContains(t, memberReasons, "child status rollup",
		"bounded cascade must not cross the intervening non-member")
	assert.Equal(t, models.StatusQueued, flatScopeStatus(t, ws, nonMemberID),
		"intervening non-member must remain untouched")
	nonMemberReasons := flatScopeStatusReasonsSince(t, ws, nonMemberID, nonMemberBaseline)
	assert.NotContains(t, nonMemberReasons, "child status rollup")
	assert.NotContains(t, nonMemberReasons, "reverted unintended rollup from partial-feature ship")
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
