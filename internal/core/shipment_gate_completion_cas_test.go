package core

import (
	stderrors "errors"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"context"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core/gate"
	bkerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/hooks"
	"github.com/softwaresalt/backlogit/internal/models"
)

// TestShipmentGate_HeadDriftBetweenGateCheckAndPersist_Refuses verifies the
// 106.033-T repository-ref CAS/guard: gateShipmentCompletion's own pre/post
// headDriftError bracket only covers ITS OWN evaluation window (the
// shipment-level gate Evaluate call plus the member-evidence scan). A
// concurrent commit that lands AFTER that bracket returns, but BEFORE
// ShipShipment's final status-transition persist (moveShipmentStatusWithTopLevel),
// was previously invisible: the signed manifest-binding proof would still
// attest to the reviewed head (B), while the shipment's own declared status
// transition completed with the repository's real HEAD having already
// advanced to an unreviewed commit (divergent).
//
// The drift is injected via a HookMoveShipmentStatus pre-hook -- the last
// synchronous extension point that runs immediately before the persist --
// mirroring the existing headChangingRunner pattern used in
// gate_evidence_formal_test.go to simulate mid-evaluation drift at the
// task-completion level.
func TestShipmentGate_HeadDriftBetweenGateCheckAndPersist_Refuses(t *testing.T) {
	t.Setenv("BACKLOGIT_GATE_EVIDENCE_KEY", validFormalTestKey)
	t.Setenv("BACKLOGIT_FORMAL_GATE_REQUIRED", "true")

	ws := newGateTestWorkspace(t)
	ws.Config.FormalGate = &config.FormalGateConfig{Enabled: true, KeyID: "k1"}
	runner := &taskAwareRunner{
		taskRes:     gate.GateResult{ExitCode: 0, Stdout: []byte(validFormalTestReport)},
		shipmentRes: gate.GateResult{ExitCode: 0, Stdout: []byte(`{}`)},
	}
	injectBroker(ws, gate.EnabledTrue, runner, fakeVersion{v: okVersion})
	ctx := context.Background()

	// Real git repo checked out on main (HEAD == head/"B") BEFORE the member
	// task is completed through the gated path, so its recorded gate evidence
	// binds to the real, resolvable HEAD.
	_, _, divergent := initGitRepoWithCommits(t, ws.RootPath)

	_, taskID, shipmentID := newGatedShipment(t, ws)
	_, _, err := UpdateArtifactWithGate(ctx, ws, taskID, map[string]any{"status": "done"}, TransitionOptions{})
	require.NoError(t, err, "member task must complete cleanly through the gate before the ship attempt")
	require.Equal(t, "done", statusOf(t, ws, taskID))

	// Simulate a concurrent commit landing in the residual window: as soon as
	// ShipShipment's own status-transition pre-hook fires (the last
	// synchronous point before the persist), check out the divergent branch,
	// advancing HEAD out from under the completing shipment.
	ws.HookRunner = hooks.NewHookRunner()
	ws.HookRunner.Register(hooks.HookMoveShipmentStatus, hooks.PhasePre, hooks.HookRegistration{
		Name: "simulate-concurrent-commit-before-persist",
		Fn: func(_ context.Context, hc hooks.HookContext) error {
			if hc.TopLevel || hc.NewValues["status"] != string(ShipmentShipped) {
				// Only the nested ShipShipment->Shipped transition is the
				// target of this guard; claim (queued->active, top-level)
				// must be left alone.
				return nil
			}
			cmd := exec.Command("git", "checkout", divergent)
			cmd.Dir = ws.RootPath
			_ = cmd.Run() // best-effort: the test asserts on ShipShipment's outcome, not this checkout
			return nil
		},
	})

	result, err := ShipShipment(ctx, ws, shipmentID, nil)
	require.Error(t, err, "a HEAD advance landing between gateShipmentCompletion's post-check and the final persist must refuse the ship")
	assert.Nil(t, result)

	var blocked *bkerrors.GateBlockedError
	require.True(t, stderrors.As(err, &blocked), "err = %v, want *GateBlockedError", err)
	assert.False(t, blocked.StateChanged)

	shipment, getErr := GetShipment(ctx, ws, shipmentID)
	require.NoError(t, getErr)
	assert.Equal(t, models.StatusActive, shipment.Status, "shipment must remain active, not transition to shipped, on a late head-drift refusal")
}

// TestShipmentGate_NoHeadDriftBeforePersist_ShipsCleanly is the paired
// happy-path regression for the 106.033-T guard: an ordinary ship, with a
// real git repo present and no interference before the persist, must still
// succeed. This guards against the new final pre-persist re-check
// introducing a false-positive refusal on the common, uneventful path.
func TestShipmentGate_NoHeadDriftBeforePersist_ShipsCleanly(t *testing.T) {
	t.Setenv("BACKLOGIT_GATE_EVIDENCE_KEY", validFormalTestKey)
	t.Setenv("BACKLOGIT_FORMAL_GATE_REQUIRED", "true")

	ws := newGateTestWorkspace(t)
	ws.Config.FormalGate = &config.FormalGateConfig{Enabled: true, KeyID: "k1"}
	runner := &taskAwareRunner{
		taskRes:     gate.GateResult{ExitCode: 0, Stdout: []byte(validFormalTestReport)},
		shipmentRes: gate.GateResult{ExitCode: 0, Stdout: []byte(`{}`)},
	}
	injectBroker(ws, gate.EnabledTrue, runner, fakeVersion{v: okVersion})
	ctx := context.Background()

	initGitRepoWithCommits(t, ws.RootPath)

	_, taskID, shipmentID := newGatedShipment(t, ws)
	_, _, err := UpdateArtifactWithGate(ctx, ws, taskID, map[string]any{"status": "done"}, TransitionOptions{})
	require.NoError(t, err)

	result, err := ShipShipment(ctx, ws, shipmentID, nil)
	require.NoError(t, err, "ship must succeed when HEAD never moves before the final persist")
	require.NotNil(t, result)
	assert.Equal(t, string(ShipmentShipped), result.ShipmentStatus)
}
