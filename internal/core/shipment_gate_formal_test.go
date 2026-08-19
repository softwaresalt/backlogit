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
	ws.Config.FormalGate = &config.FormalGateConfig{Enabled: true, KeyID: "k1"}
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
			"proof_schema":  gateproof.SchemaLegacy,
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
	ws.Config.FormalGate = &config.FormalGateConfig{Enabled: true, KeyID: "k1"}
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

// TestShipmentGate_EvidenceRequiredFalse_FormalEnforcement_AppendFailureRefuses
// verifies that under formal-gate enforcement, a failure durably appending the
// FINAL shipment-level EventGatePassed record (the durable write of an
// ALREADY-signed passDelta) is NEVER downgraded to a warning by the UNRELATED
// evidence_required:false config knob — mirroring the analogous, already-fixed
// task-level guarantee (mustRefuseGateEvidenceFailure; see
// TestGateEvidence_FormalGateRequired_EvidenceNotRequired_StillRefuses).
// Before this fix, gateShipmentCompletion's final append check only consulted
// ws.gateConfig.EvidenceRequiredValue(), so evidence_required:false let a
// formally-enforced ship proceed to completion even though its signed pass
// proof was never durably recorded at all — a shipment-level parity gap with
// the task-level fix, and a genuine authenticity/audit-trail hole for a
// feature whose entire purpose is guaranteeing a durable, verifiable pass
// record (106-F F1 review finding, round 3).
func TestShipmentGate_EvidenceRequiredFalse_FormalEnforcement_AppendFailureRefuses(t *testing.T) {
	t.Setenv("BACKLOGIT_GATE_EVIDENCE_KEY", validFormalTestKey)
	t.Setenv("BACKLOGIT_FORMAL_GATE_REQUIRED", "true")

	ws := newGateTestWorkspace(t)
	ws.Config.FormalGate = &config.FormalGateConfig{Enabled: true, KeyID: "k1"}

	runner := &taskAwareRunner{
		taskRes:     gate.GateResult{ExitCode: 0, Stdout: []byte(validFormalTestReport)},
		shipmentRes: gate.GateResult{ExitCode: 0, Stdout: []byte(`{}`)},
	}
	// injectBroker OVERWRITES ws.gateConfig wholesale (fresh Normalize()), so
	// the evidence_required override MUST be applied AFTER injectBroker or it
	// is silently discarded (matches the ordering every other
	// evidence_required:false test in this package already uses).
	injectBroker(ws, gate.EnabledTrue, runner, fakeVersion{v: okVersion})
	notRequired := false
	ws.gateConfig.EvidenceRequired = &notRequired
	ctx := context.Background()

	_, taskID, shipmentID := newGatedShipment(t, ws)
	_, _, err := UpdateArtifactWithGate(ctx, ws, taskID, map[string]any{"status": "done"}, TransitionOptions{})
	require.NoError(t, err, "task completion must succeed and produce properly signed formal-gate evidence")

	// Inject a plain durable-storage failure for ONLY the shipment-level
	// passed-event append. The member task's own evidence append (above,
	// during UpdateArtifactWithGate) already ran through the real appender, so
	// its evidence is genuinely signed; this seam only intercepts the later
	// shipment-level append gateShipmentCompletion performs.
	ws.gateEvidenceAppend = func(ctx context.Context, w *Workspace, itemID, eventType string, delta map[string]any) error {
		if itemID == shipmentID && eventType == EventGatePassed {
			return stderrors.New("injected shipment evidence append failure")
		}
		return appendItemEventErr(ctx, w, itemID, eventType, delta)
	}

	_, err = ShipShipment(ctx, ws, shipmentID, nil)
	require.Error(t, err, "a failed shipment gate-pass evidence append must refuse the ship under formal enforcement even when evidence_required is false")

	sh, gErr := GetShipment(ctx, ws, shipmentID)
	require.NoError(t, gErr)
	require.Equal(t, models.StatusActive, sh.Status, "shipment must not ship when its signed pass evidence was never durably recorded")
}

// TestValidateMemberGateEvidence_FormalEnforcement_LineageUsesAuthenticatedEvent
// verifies that under formal-gate enforcement, the per-member LINEAGE check
// uses the event FormalAdmit actually authenticated (res.Event), never the
// legacy Latest-selected event (latestGatePassEvidence). Latest treats
// EventGateForced as unconditionally qualifying (any ran, no proof required)
// and always prefers whichever qualifying event is chronologically LATEST,
// while FormalAdmit deliberately never admits EventGateForced and does not
// treat a LATER Forced event as invalidating a prior genuinely-signed pass
// either (only Blocked/Requeued/Escalated invalidate). This means an actor
// who can append JSONL entries can add a LATER, completely unsigned
// EventGateForced carrying an ARBITRARY forged head_sha — e.g. one crafted to
// exactly equal the shipment head, trivially satisfying the lineage check's
// fast equality path — even though the actual AUTHENTICATED pass evidence was
// signed for a genuinely DIVERGENT (non-ancestor) commit. Before this fix,
// the lineage check read head_sha from `latest` (the Forced-preferring Latest
// selection), so this forged, unsigned entry silently overrode the real,
// cryptographically-verified commit binding (106-F F1 review finding, round
// 4).
func TestValidateMemberGateEvidence_FormalEnforcement_LineageUsesAuthenticatedEvent(t *testing.T) {
	t.Setenv("BACKLOGIT_GATE_EVIDENCE_KEY", validFormalTestKey)
	t.Setenv("BACKLOGIT_FORMAL_GATE_REQUIRED", "true")

	ws := newGateTestWorkspace(t)
	ws.Config.FormalGate = &config.FormalGateConfig{Enabled: true, KeyID: "k1"}
	ctx := context.Background()

	_, head, divergent := initGitRepoWithCommits(t, ws.RootPath)

	id := newActiveTask(t, ws)
	_, err := updateArtifactUngated(ctx, ws, id, map[string]any{"status": "done"})
	require.NoError(t, err)

	// Genuine, properly signed evidence -- for the DIVERGENT (non-ancestor)
	// commit: real, authenticated lineage that MUST be refused once correctly
	// checked.
	key, keyErr := config.ResolveFormalGateKey()
	require.NoError(t, keyErr)
	env := gateproof.Envelope{
		Magic:        gateproof.Magic,
		Purpose:      gateproof.PurposeTask,
		Schema:       gateproof.SchemaLegacy,
		Alg:          gateproof.AlgHMACSHA256,
		KeyID:        "k1",
		WorkspaceID:  workspaceIdentity(ws.RootPath),
		ItemID:       id,
		EventType:    EventGatePassed,
		Ran:          true,
		Actor:        "backlogit",
		TimestampUTC: "2026-08-08T00:00:00Z",
		HeadSHA:      divergent,
		ReportDigest: "digest123",
		Counter:      1,
	}
	proof, signErr := gateproof.Sign(env, key)
	require.NoError(t, signErr)
	require.NoError(t, appendItemEventErr(ctx, ws, id, EventGatePassed, map[string]any{
		"ran":           true,
		"proof":         proof,
		"key_id":        env.KeyID,
		"proof_schema":  env.Schema,
		"counter":       env.Counter,
		"head_sha":      env.HeadSHA,
		"report_digest": env.ReportDigest,
		"timestamp_utc": env.TimestampUTC,
	}))

	// A LATER, completely unsigned Forced event -- no proof/key_id/counter at
	// all -- carrying a FORGED head_sha crafted to exactly equal the shipment
	// head, so an unfixed lineage check trivially "passes" via the fast
	// equality path regardless of what was actually authenticated.
	require.NoError(t, appendItemEventErr(ctx, ws, id, EventGateForced, map[string]any{
		"outcome": "forced", "ran": true, "head_sha": head, "forced": true,
	}))

	err = validateMemberGateEvidence(ctx, ws, []string{id}, head)
	require.Error(t, err, "lineage must be checked against the AUTHENTICATED pass event's head_sha (divergent, non-ancestor), not a later unsigned Forced event's forged head_sha")
	assert.Contains(t, err.Error(), "divergent", "the refusal must reflect the authenticated evidence's real (divergent) lineage")
}

// TestUpdateArtifactWithGate_ShipmentToShippedBypassesFormalEnforcement_Refused
// verifies that a shipment cannot be moved directly to "shipped" through the
// GENERAL UpdateArtifactWithGate entry point (the shared choke point behind
// both the backlogit_move_item and backlogit_update_item MCP tools, and their
// CLI equivalents) while formal-gate evidence is enforced. gateApplies (and
// its nil-broker sibling gateWouldApplyButForBroker) are hardcoded to
// task/subtask artifact types only, so a shipment ALWAYS fails that check
// and falls straight through to updateArtifactUngated — completely bypassing
// ShipShipment's member-evidence verification, manifest-binding signing, and
// membership locking entirely. This is a materially more complete bypass
// than even an operator --force: it requires no force flag at all, just
// calling a DIFFERENT, pre-existing, general-purpose tool instead of
// ship_shipment (106-F F1 review finding, round 8). The fix requires
// callers to use ShipShipment for this specific transition when formal
// enforcement is active, rather than attempting to replicate its
// verification logic inline in the general update path.
func TestUpdateArtifactWithGate_ShipmentToShippedBypassesFormalEnforcement_Refused(t *testing.T) {
	t.Setenv("BACKLOGIT_GATE_EVIDENCE_KEY", validFormalTestKey)
	t.Setenv("BACKLOGIT_FORMAL_GATE_REQUIRED", "true")

	ws := newGateTestWorkspace(t)
	ws.Config.FormalGate = &config.FormalGateConfig{Enabled: true, KeyID: "k1"}
	ctx := context.Background()

	_, _, shipmentID := newGatedShipment(t, ws) // newGatedShipment already claims the shipment to active

	_, _, err := UpdateArtifactWithGate(ctx, ws, shipmentID, map[string]any{"status": "shipped"}, TransitionOptions{})
	require.Error(t, err, "a direct shipment-to-shipped update must be refused, not silently completed ungated")
	// 144-F: guard 1 is now unconditional; ErrShipmentShippedRequiresEnvelope
	// supersedes ErrFormalGateRequired for this call path.
	require.True(t, stderrors.Is(err, bkerrors.ErrShipmentShippedRequiresEnvelope), "err = %v, want ErrShipmentShippedRequiresEnvelope", err)

	sh, gErr := GetShipment(ctx, ws, shipmentID)
	require.NoError(t, gErr)
	require.Equal(t, models.StatusActive, sh.Status, "shipment must remain active: only ShipShipment may transition it to shipped under formal enforcement")
}
