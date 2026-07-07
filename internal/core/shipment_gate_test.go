package core

import (
	"context"
	stderrors "errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core/gate"
	bkerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
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
		"outcome": "passed", "ran": true, "head_sha": "oldsha0000",
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

// TestLatestGatePassEvidence_ComposedPredicate pins the F4 (083.002-T) composed
// member-evidence predicate: a member gate-pass event counts as valid evidence
// only when it is an EventGateForced (unconditional break-glass) OR an
// EventGatePassed with ran==true. A fail-open EventGatePassed{ran:false} no-run
// must NOT be selected as the latest passing evidence.
func TestLatestGatePassEvidence_ComposedPredicate(t *testing.T) {
	passedRan := events.Event{EventType: EventGatePassed, Delta: map[string]any{"ran": true, "head_sha": "p1"}}
	passedNoRun := events.Event{EventType: EventGatePassed, Delta: map[string]any{"ran": false, "head_sha": "pn"}}
	passedMissingRan := events.Event{EventType: EventGatePassed, Delta: map[string]any{"head_sha": "pm"}}
	forcedRan := events.Event{EventType: EventGateForced, Delta: map[string]any{"ran": true, "head_sha": "f1"}}
	forcedNoRun := events.Event{EventType: EventGateForced, Delta: map[string]any{"ran": false, "head_sha": "fn"}}
	blocked := events.Event{EventType: EventGateBlocked, Delta: map[string]any{"ran": true}}

	tests := []struct {
		name     string
		evs      []events.Event
		wantNil  bool
		wantHead string // head_sha of the expected latest event when non-nil
	}{
		{name: "passed_ran_true", evs: []events.Event{passedRan}, wantHead: "p1"},
		{name: "passed_ran_false_rejected", evs: []events.Event{passedNoRun}, wantNil: true},
		{name: "passed_missing_ran_rejected", evs: []events.Event{passedMissingRan}, wantNil: true},
		{name: "forced_ran_false_still_valid", evs: []events.Event{forcedNoRun}, wantHead: "fn"},
		{name: "forced_ran_true_valid", evs: []events.Event{forcedRan}, wantHead: "f1"},
		{name: "interleaved_forced_then_passed_norun", evs: []events.Event{forcedNoRun, passedNoRun}, wantHead: "fn"},
		{name: "passed_ran_true_then_passed_norun_keeps_ran", evs: []events.Event{passedRan, passedNoRun}, wantHead: "p1"},
		{name: "only_blocked_missing", evs: []events.Event{blocked}, wantNil: true},
		{name: "empty_missing", evs: nil, wantNil: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := latestGatePassEvidence(tt.evs)
			if tt.wantNil {
				assert.Nil(t, got, "ran=false EventGatePassed / non-pass events must not count as evidence")
				return
			}
			require.NotNil(t, got)
			h, _ := got.Delta["head_sha"].(string)
			assert.Equal(t, tt.wantHead, h)
		})
	}
}

// TestValidateMemberGateEvidence_FailOpenNoRunRejected exercises the F4 predicate
// through the shipment member scan: a terminal member whose only pass evidence is
// a fail-open EventGatePassed{ran:false} is refused as missing evidence.
func TestValidateMemberGateEvidence_FailOpenNoRunRejected(t *testing.T) {
	ws := newGateTestWorkspace(t)
	runner := &fakeGateRunner{res: gate.GateResult{ExitCode: 0, Stdout: []byte(`{}`)}}
	injectBroker(ws, gate.EnabledTrue, runner, fakeVersion{v: okVersion})
	ctx := context.Background()

	id := newActiveTask(t, ws)
	_, err := updateArtifactUngated(ctx, ws, id, map[string]any{"status": "done"})
	require.NoError(t, err)
	// A fail-open no-run: EventGatePassed with ran=false must be treated as missing.
	require.NoError(t, appendItemEventErr(ctx, ws, id, EventGatePassed, map[string]any{
		"outcome": "passed", "ran": false,
	}))

	err = validateMemberGateEvidence(ctx, ws, []string{id}, "")
	require.Error(t, err, "a ran=false EventGatePassed no-run must be treated as missing gate evidence")
	assert.Contains(t, err.Error(), "missing")
	var blocked *bkerrors.GateBlockedError
	require.True(t, stderrors.As(err, &blocked))
}

// TestValidateMemberGateEvidence_ForcedNoRunAccepted confirms the audited
// break-glass force is unconditional: an EventGateForced counts even with ran=false.
func TestValidateMemberGateEvidence_ForcedNoRunAccepted(t *testing.T) {
	ws := newGateTestWorkspace(t)
	runner := &fakeGateRunner{res: gate.GateResult{ExitCode: 0, Stdout: []byte(`{}`)}}
	injectBroker(ws, gate.EnabledTrue, runner, fakeVersion{v: okVersion})
	ctx := context.Background()

	id := newActiveTask(t, ws)
	_, err := updateArtifactUngated(ctx, ws, id, map[string]any{"status": "done"})
	require.NoError(t, err)
	require.NoError(t, appendItemEventErr(ctx, ws, id, EventGateForced, map[string]any{
		"outcome": "passed", "ran": false, "forced": true,
	}))

	require.NoError(t, validateMemberGateEvidence(ctx, ws, []string{id}, ""),
		"a forced break-glass event must satisfy member evidence regardless of ran")
}
