package core

import (
	"context"
	stderrors "errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core/gate"
	bkerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/models"
)

// TestShipmentGate_FailOpenAuto_FormalEnforcementRefuses mirrors
// TestShipmentGate_FailOpenAuto_ShipsWithoutEvidence but with formal gate
// evidence enforced: the ordinary auto fail-open early return
// (gateShipmentCompletion's "!ev.Enforced") must refuse rather than silently
// let a shipment ship with no enforceable gate at all (106-F F1/U6).
func TestShipmentGate_FailOpenAuto_FormalEnforcementRefuses(t *testing.T) {
	t.Setenv("BACKLOGIT_FORMAL_GATE_REQUIRED", "true")
	ws := newGateTestWorkspace(t)
	ws.Config.FormalGate = &config.FormalGateConfig{Enabled: true, KeyID: "k1"}
	// enabled:auto + unresolvable binary -> not enforced by the ordinary gate,
	// but formal admission is required, so this must now refuse.
	injectBroker(ws, gate.EnabledAuto, &fakeGateRunner{}, fakeVersion{err: bkerrors.ErrGateBinaryNotFound})
	ctx := context.Background()

	_, taskID, shipmentID := newGatedShipment(t, ws)
	require.Equal(t, "active", statusOf(t, ws, taskID))

	_, err := ShipShipment(ctx, ws, shipmentID, nil)
	require.Error(t, err, "auto fail-open must refuse to ship when formal gate evidence is enforced")
	require.True(t, stderrors.Is(err, bkerrors.ErrFormalGateRequired), "err = %v, want ErrFormalGateRequired", err)

	sh, gErr := GetShipment(ctx, ws, shipmentID)
	require.NoError(t, gErr)
	require.Equal(t, models.StatusActive, sh.Status, "shipment must not ship on a formal-enforcement refusal")
}

// TestShipmentGate_NilBroker_FormalEnforcementRefuses verifies that a nil (or
// unwired/disabled) gate broker refuses shipment completion when formal gate
// evidence is enforced, rather than silently preserving pre-gate ship
// behavior (106-F F1/U6).
func TestShipmentGate_NilBroker_FormalEnforcementRefuses(t *testing.T) {
	t.Setenv("BACKLOGIT_FORMAL_GATE_REQUIRED", "true")
	ws := newGateTestWorkspace(t)
	ws.Config.FormalGate = &config.FormalGateConfig{Enabled: true, KeyID: "k1"}
	ws.GateBroker = nil // disabled/unwired
	ctx := context.Background()

	_, taskID, shipmentID := newGatedShipment(t, ws)
	require.Equal(t, "active", statusOf(t, ws, taskID))

	_, err := ShipShipment(ctx, ws, shipmentID, nil)
	require.Error(t, err, "a nil gate broker must refuse to ship when formal gate evidence is enforced")
	require.True(t, stderrors.Is(err, bkerrors.ErrFormalGateRequired), "err = %v, want ErrFormalGateRequired", err)

	sh, gErr := GetShipment(ctx, ws, shipmentID)
	require.NoError(t, gErr)
	require.Equal(t, models.StatusActive, sh.Status, "shipment must not ship on a formal-enforcement refusal")
}

// TestShipmentGate_NilBroker_NoFormalEnforcement_PreservesLegacyBehavior
// characterizes the unchanged, pre-existing behavior: a nil broker without
// formal gate enforcement still silently preserves pre-gate ship behavior.
func TestShipmentGate_NilBroker_NoFormalEnforcement_PreservesLegacyBehavior(t *testing.T) {
	ws := newGateTestWorkspace(t)
	ws.GateBroker = nil
	ctx := context.Background()

	_, _, shipmentID := newGatedShipment(t, ws)

	result, err := ShipShipment(ctx, ws, shipmentID, nil)
	require.NoError(t, err, "a nil broker without formal enforcement must preserve pre-gate ship behavior")
	require.NotNil(t, result)
}
