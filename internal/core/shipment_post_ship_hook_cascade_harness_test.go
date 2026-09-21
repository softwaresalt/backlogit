package core

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/hooks"
	"github.com/softwaresalt/backlogit/internal/models"
)

func TestUPostShipHookCascadeGlobal_UnrelatedHookMutationRetainsGlobalCascade(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	ws.HookRunner = hooks.NewHookRunner()

	nonMember, err := CreateArtifact(ctx, ws, "Ship non-member ancestor", "feature")
	require.NoError(t, err)
	first, err := CreateArtifact(ctx, ws, "Ship member first", "task", WithParent(nonMember.ID))
	require.NoError(t, err)
	second, err := CreateArtifact(ctx, ws, "Ship member second", "task", WithParent(nonMember.ID))
	require.NoError(t, err)
	shipment, err := CreateShipment(ctx, ws, "Post-ship cascade shipment", []string{first.ID, second.ID})
	require.NoError(t, err)
	_, err = ClaimShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)
	forceFlatScopeStatus(t, ws, nonMember.ID, models.StatusQueued)
	baselineEvents := flatScopeEventCount(t, ws, nonMember.ID)

	unrelatedParent, err := CreateArtifact(ctx, ws, "Unrelated cascade parent", "feature")
	require.NoError(t, err)
	unrelatedChild, err := CreateArtifact(ctx, ws, "Unrelated cascade child", "task", WithParent(unrelatedParent.ID))
	require.NoError(t, err)
	unrelatedSibling, err := CreateArtifact(ctx, ws, "Unrelated terminal sibling", "task", WithParent(unrelatedParent.ID))
	require.NoError(t, err)
	forceFlatScopeStatus(t, ws, unrelatedParent.ID, models.StatusActive)
	forceFlatScopeStatus(t, ws, unrelatedChild.ID, models.StatusActive)
	forceFlatScopeStatus(t, ws, unrelatedSibling.ID, models.StatusDone)

	hookFired := false
	var hookErr error
	ws.HookRunner.Register(hooks.HookShipShipment, hooks.PhasePost, hooks.HookRegistration{
		Name:     "wave_7_global_cascade_probe",
		Priority: 100,
		Fn: func(hookCtx context.Context, _ hooks.HookContext) error {
			hookFired = true
			_, hookErr = setArtifactStatus(
				hookCtx,
				ws,
				unrelatedChild.ID,
				models.StatusDone,
				"unrelated post-ship hook update",
			)
			return hookErr
		},
	})

	result, err := ShipShipment(ctx, ws, shipment.ID, nil)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, hookFired, "SCOPE-RED-F fixture must execute the post-ship hook")
	require.NoError(t, hookErr)

	assert.Equal(t, models.StatusQueued, flatScopeStatus(t, ws, nonMember.ID),
		"SCOPE-RED-F: hierarchy-derived non-member ancestor must remain untouched")
	reasons := flatScopeStatusReasonsSince(t, ws, nonMember.ID, baselineEvents)
	assert.NotContains(t, reasons, "child status rollup",
		"SCOPE-RED-F: shipment completion must not roll up a non-member ancestor")
	assert.NotContains(t, reasons, "reverted unintended rollup from partial-feature ship",
		"SCOPE-RED-F: shipment completion must not restore a non-member ancestor")
	assert.Equal(t, models.StatusDone, flatScopeStatus(t, ws, unrelatedParent.ID),
		"SCOPE-RED-F: post-ship hook context must retain the normal global parent cascade")
}
