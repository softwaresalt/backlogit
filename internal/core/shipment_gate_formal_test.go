package core

import (
	"context"
	stderrors "errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core/gate"
	bkerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/gateproof"
	"github.com/softwaresalt/backlogit/internal/models"
)

// validFormalTestKey is a fixed, well-formed (66 hex chars = 33 bytes >= the
// 32-byte minimum) test-only HMAC key shared by the member-evidence formal
// enforcement tests below. It carries no production significance.
const validFormalTestKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

// validFormalTestReport is a minimal report satisfying
// gate.ValidateFormalReport's attributed-reviewer-entry requirement, so a
// per-task completion under formal enforcement produces genuinely SIGNED
// (not merely schema-empty) evidence.
const validFormalTestReport = `{"reviewers":[{"persona":"Constitution Reviewer","decision":"pass"}]}`


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

// TestShipmentGate_MemberEvidenceUnsigned_FormalEnforcementRefuses verifies
// that a member whose passing gate evidence was recorded BEFORE formal
// enforcement was turned on — so it carries ran:true and a head_sha but no
// proof/key_id/counter at all — is refused at ship time once
// BACKLOGIT_FORMAL_GATE_REQUIRED is set. Before this fix, validateMemberGateEvidence
// selected member evidence via the pre-F1 gateevidence.Latest predicate only,
// which never checks a proof and therefore never refuses unsigned evidence
// regardless of enforcement — silently defeating the shipment's own
// authenticity guarantee at the one point (ship time, per member) it matters
// most (106-F F1/U6 review finding).
func TestShipmentGate_MemberEvidenceUnsigned_FormalEnforcementRefuses(t *testing.T) {
	unsetTestEnv(t, "BACKLOGIT_FORMAL_GATE_REQUIRED")
	unsetTestEnv(t, "BACKLOGIT_GATE_EVIDENCE_KEY")

	ws := newGateTestWorkspace(t)
	runner := &taskAwareRunner{
		taskRes:     gate.GateResult{ExitCode: 0, Stdout: []byte(validFormalTestReport)},
		shipmentRes: gate.GateResult{ExitCode: 0, Stdout: []byte(`{}`)},
	}
	injectBroker(ws, gate.EnabledTrue, runner, fakeVersion{v: okVersion})
	ctx := context.Background()

	_, taskID, shipmentID := newGatedShipment(t, ws)
	// Complete the member task WITHOUT formal enforcement -> plain, unsigned
	// gate-pass evidence (ran:true, head_sha, no proof/key_id/counter fields).
	_, _, err := UpdateArtifactWithGate(ctx, ws, taskID, map[string]any{"status": "done"}, TransitionOptions{})
	require.NoError(t, err)
	require.Equal(t, "done", statusOf(t, ws, taskID))

	// Now require formal gate evidence at ship time — the member's evidence
	// predates enforcement and was never signed.
	t.Setenv("BACKLOGIT_GATE_EVIDENCE_KEY", validFormalTestKey)
	t.Setenv("BACKLOGIT_FORMAL_GATE_REQUIRED", "true")

	_, err = ShipShipment(ctx, ws, shipmentID, nil)
	require.Error(t, err, "ship must refuse a member whose evidence carries no formal proof once enforcement is required")
	// The refusal wraps the TYPED FormalAdmit cause (ErrProofInvalid or
	// ErrProofUnverifiable), not *GateBlockedError, so gateErrorResult's MCP
	// dispatch reaches the specific formal_gate_proof_* error code instead
	// of collapsing to the generic gate_blocked class (106-F F1 review
	// finding). Unsigned evidence is missing its proof fields entirely, so
	// it classifies as ErrProofUnverifiable (could not be evaluated at all).
	require.True(t, stderrors.Is(err, bkerrors.ErrProofUnverifiable), "err = %v, want ErrProofUnverifiable", err)
	assert.Contains(t, err.Error(), taskID)

	sh, gErr := GetShipment(ctx, ws, shipmentID)
	require.NoError(t, gErr)
	require.Equal(t, models.StatusActive, sh.Status, "shipment must not ship when member evidence fails formal admission")
}

// TestShipmentGate_MemberEvidenceTamperedProof_FormalEnforcementRefuses
// hand-appends a forged EventGatePassed event directly to the member's JSONL
// log — simulating an actor with write access to .backlogit/logs/ hand-authoring
// a plausible-looking pass record, the exact adversary this shipment's design
// doc names as the threat model — and verifies ship refuses it under formal
// enforcement even though gateevidence.Latest alone would have accepted it.
func TestShipmentGate_MemberEvidenceTamperedProof_FormalEnforcementRefuses(t *testing.T) {
	t.Setenv("BACKLOGIT_GATE_EVIDENCE_KEY", validFormalTestKey)
	t.Setenv("BACKLOGIT_FORMAL_GATE_REQUIRED", "true")

	ws := newGateTestWorkspace(t)
	runner := &taskAwareRunner{
		taskRes:     gate.GateResult{ExitCode: 0, Stdout: []byte(validFormalTestReport)},
		shipmentRes: gate.GateResult{ExitCode: 0, Stdout: []byte(`{}`)},
	}
	injectBroker(ws, gate.EnabledTrue, runner, fakeVersion{v: okVersion})
	ctx := context.Background()

	_, taskID, shipmentID := newGatedShipment(t, ws)
	// Move to done WITHOUT going through the gate at all, then hand-append a
	// forged pass event carrying a garbage "proof" value and plausible-looking
	// proof metadata — this never goes through gateproof.Sign.
	_, err := UpdateArtifact(ctx, ws, taskID, map[string]any{"status": "done"})
	require.NoError(t, err)

	logsDir := WorkspaceLogsRoot(ws.RootPath)
	writer := NewWorkspaceEventWriter(ws, logsDir)
	require.NoError(t, writer.AppendEvent(ctx, events.Event{
		Timestamp: time.Now(),
		Actor:     "backlogit",
		ItemID:    taskID,
		EventType: EventGatePassed,
		Delta: map[string]any{
			"ran":           true,
			"head_sha":      "",
			"proof":         "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
			"key_id":        "k1",
			"proof_schema":  gateproof.Schema,
			"counter":       int64(1),
			"timestamp_utc": time.Now().UTC().Format(time.RFC3339),
			"report_digest": "",
		},
	}))

	_, err = ShipShipment(ctx, ws, shipmentID, nil)
	require.Error(t, err, "ship must refuse forged/tampered member evidence under formal enforcement")
	// The tampered proof fails MAC verification, classifying as
	// ErrProofInvalid (definitively wrong), not *GateBlockedError — see the
	// unsigned-evidence test above for why this matters for MCP dispatch.
	require.True(t, stderrors.Is(err, bkerrors.ErrProofInvalid), "err = %v, want ErrProofInvalid", err)
	assert.Contains(t, err.Error(), taskID)

	sh, gErr := GetShipment(ctx, ws, shipmentID)
	require.NoError(t, gErr)
	require.Equal(t, models.StatusActive, sh.Status, "shipment must not ship when member evidence is a tampered/forged proof")
}

// TestShipmentGate_MemberEvidenceProperlySigned_FormalEnforcementShips is the
// positive counterpart: a member whose evidence WAS produced by the real
// signing path (augmentDeltaWithFormalProof, with enforcement already on at
// completion time) must still ship successfully under the same enforcement —
// confirming the new per-member formal check does not over-refuse genuinely
// authentic evidence.
func TestShipmentGate_MemberEvidenceProperlySigned_FormalEnforcementShips(t *testing.T) {
	t.Setenv("BACKLOGIT_GATE_EVIDENCE_KEY", validFormalTestKey)
	t.Setenv("BACKLOGIT_FORMAL_GATE_REQUIRED", "true")

	ws := newGateTestWorkspace(t)
	runner := &taskAwareRunner{
		taskRes:     gate.GateResult{ExitCode: 0, Stdout: []byte(validFormalTestReport)},
		shipmentRes: gate.GateResult{ExitCode: 0, Stdout: []byte(`{}`)},
	}
	injectBroker(ws, gate.EnabledTrue, runner, fakeVersion{v: okVersion})
	ctx := context.Background()

	_, taskID, shipmentID := newGatedShipment(t, ws)
	_, _, err := UpdateArtifactWithGate(ctx, ws, taskID, map[string]any{"status": "done"}, TransitionOptions{})
	require.NoError(t, err, "task completion must succeed and produce properly signed formal-gate evidence")
	require.Equal(t, "done", statusOf(t, ws, taskID))

	result, err := ShipShipment(ctx, ws, shipmentID, nil)
	require.NoError(t, err, "ship must succeed when member evidence is properly formally admitted")
	require.NotNil(t, result)
	assert.Equal(t, string(ShipmentShipped), result.ShipmentStatus)
}
