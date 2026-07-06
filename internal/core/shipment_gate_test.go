package core

import (
	"context"
	stderrors "errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core/gate"
	bkerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/models"
)

// taskAwareRunner returns a passing result for per-task gate checks (args contain
// --task) and a caller-configured result for the shipment-level aggregate check
// (no --task). This lets a test complete member tasks cleanly while forcing the
// shipment-diff check to block.
type taskAwareRunner struct {
	taskRes     gate.GateResult
	shipmentRes gate.GateResult
	lastCmd     []string
}

func (r *taskAwareRunner) Run(_ context.Context, args []string, _ string, _ []string) (gate.GateResult, error) {
	r.lastCmd = args
	for _, a := range args {
		if a == "--task" {
			return r.taskRes, nil
		}
	}
	return r.shipmentRes, nil
}

// newGatedFeatureTask creates a feature+task, wraps them in a claimed shipment,
// and returns the ids. The task is left active (claimed), ready to be completed
// through the gated path by the caller.
func newGatedShipment(t *testing.T, ws *Workspace) (featureID, taskID, shipmentID string) {
	t.Helper()
	ctx := context.Background()
	feat, err := CreateArtifact(ctx, ws, "Ship gate feature", "feature")
	require.NoError(t, err)
	task, err := CreateArtifact(ctx, ws, "Ship gate task", "task", WithParent(feat.ID))
	require.NoError(t, err)
	shipment, err := CreateShipment(ctx, ws, "Ship gate shipment", []string{task.ID})
	require.NoError(t, err)
	_, err = ClaimShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)
	return feat.ID, task.ID, shipment.ID
}

func TestShipmentGate_AllMembersHaveEvidence_Ships(t *testing.T) {
	ws := newGateTestWorkspace(t)
	runner := &fakeGateRunner{res: gate.GateResult{ExitCode: 0, Stdout: []byte(`{}`)}}
	injectBroker(ws, gate.EnabledTrue, runner, fakeVersion{v: okVersion})
	ctx := context.Background()

	_, taskID, shipmentID := newGatedShipment(t, ws)
	// Complete the member task through the gated path -> records passing evidence.
	_, _, err := UpdateArtifactWithGate(ctx, ws, taskID, map[string]any{"status": "done"}, TransitionOptions{})
	require.NoError(t, err)
	require.Equal(t, "done", statusOf(t, ws, taskID))

	result, err := ShipShipment(ctx, ws, shipmentID, nil)
	require.NoError(t, err, "ship must succeed when members carry evidence and the shipment gate passes")
	require.NotNil(t, result)
	assert.Equal(t, string(ShipmentShipped), result.ShipmentStatus)
}

func TestShipmentGate_MemberMissingEvidence_Refused(t *testing.T) {
	ws := newGateTestWorkspace(t)
	runner := &fakeGateRunner{res: gate.GateResult{ExitCode: 0, Stdout: []byte(`{}`)}}
	injectBroker(ws, gate.EnabledTrue, runner, fakeVersion{v: okVersion})
	ctx := context.Background()

	_, taskID, shipmentID := newGatedShipment(t, ws)
	// Leave the task active (never completed through the gate).
	require.Equal(t, "active", statusOf(t, ws, taskID))

	_, err := ShipShipment(ctx, ws, shipmentID, nil)
	require.Error(t, err, "ship must refuse a member that was never completed through the gate")
	assert.Contains(t, err.Error(), taskID)

	var blocked *bkerrors.GateBlockedError
	require.True(t, stderrors.As(err, &blocked), "want *GateBlockedError, got %T", err)

	// Reconciliation guarantee: the ungated member was NOT auto-completed, and the
	// shipment remains active.
	assert.Equal(t, "active", statusOf(t, ws, taskID), "ungated member must not be auto-completed")
	sh, gErr := GetShipment(ctx, ws, shipmentID)
	require.NoError(t, gErr)
	assert.Equal(t, models.StatusActive, sh.Status, "shipment state must be unchanged on refusal")
}

func TestShipmentGate_ShipmentDiffBlocked_Refused(t *testing.T) {
	ws := newGateTestWorkspace(t)
	// Per-task checks pass (exit 0); the shipment-level aggregate check blocks.
	report := `{"repeated_failure":{"count":1,"threshold":3,"reached":false,"action":"block"}}`
	runner := &taskAwareRunner{
		taskRes:     gate.GateResult{ExitCode: 0, Stdout: []byte(`{}`)},
		shipmentRes: gate.GateResult{ExitCode: 1, Stdout: []byte(report)},
	}
	injectBroker(ws, gate.EnabledTrue, runner, fakeVersion{v: okVersion})
	ctx := context.Background()

	_, taskID, shipmentID := newGatedShipment(t, ws)
	_, _, err := UpdateArtifactWithGate(ctx, ws, taskID, map[string]any{"status": "done"}, TransitionOptions{})
	require.NoError(t, err)

	_, err = ShipShipment(ctx, ws, shipmentID, nil)
	require.Error(t, err, "ship must refuse when the shipment-level gate check blocks")
	var blocked *bkerrors.GateBlockedError
	require.True(t, stderrors.As(err, &blocked))
	assert.Equal(t, shipmentID, blocked.ItemID)

	sh, gErr := GetShipment(ctx, ws, shipmentID)
	require.NoError(t, gErr)
	assert.Equal(t, models.StatusActive, sh.Status, "shipment state unchanged after a shipment-diff block")
}

func TestShipmentGate_FailOpenAuto_ShipsWithoutEvidence(t *testing.T) {
	ws := newGateTestWorkspace(t)
	// enabled:auto + unresolvable binary -> not enforced -> shipment gate fails open.
	injectBroker(ws, gate.EnabledAuto, &fakeGateRunner{}, fakeVersion{err: bkerrors.ErrGateBinaryNotFound})
	ctx := context.Background()

	_, taskID, shipmentID := newGatedShipment(t, ws)
	// The task is still active with NO gate evidence, yet a fail-open ship proceeds
	// (completeReleaseScope completes it) — enforcement is off in this environment.
	require.Equal(t, "active", statusOf(t, ws, taskID))

	result, err := ShipShipment(ctx, ws, shipmentID, nil)
	require.NoError(t, err, "auto fail-open must not enforce shipment gating")
	require.NotNil(t, result)
	assert.Equal(t, string(ShipmentShipped), result.ShipmentStatus)
}

// TestValidateMemberGateEvidence_StaleRefused exercises the staleness dimension
// directly: a terminal member whose latest evidence head SHA predates the
// shipment head is refused.
func TestValidateMemberGateEvidence_StaleRefused(t *testing.T) {
	ws := newGateTestWorkspace(t)
	runner := &fakeGateRunner{res: gate.GateResult{ExitCode: 0, Stdout: []byte(`{}`)}}
	injectBroker(ws, gate.EnabledTrue, runner, fakeVersion{v: okVersion})
	ctx := context.Background()

	id := newActiveTask(t, ws)
	// Complete ungated to done, then hand-write a passing evidence event carrying
	// an OLD head SHA so the staleness comparison has something to reject.
	_, err := updateArtifactUngated(ctx, ws, id, map[string]any{"status": "done"})
	require.NoError(t, err)
	require.NoError(t, appendItemEventErr(ctx, ws, id, EventGatePassed, map[string]any{
		"outcome": "passed", "head_sha": "oldsha0000",
	}))

	// Same head -> accepted.
	require.NoError(t, validateMemberGateEvidence(ctx, ws, []string{id}, "oldsha0000"))
	// Newer shipment head -> stale evidence refused.
	err = validateMemberGateEvidence(ctx, ws, []string{id}, "newsha1111")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stale")
	var blocked *bkerrors.GateBlockedError
	require.True(t, stderrors.As(err, &blocked))
}
