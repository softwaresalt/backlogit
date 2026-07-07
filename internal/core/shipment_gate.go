package core

import (
	"context"
	stderrors "errors"
	"fmt"
	"log/slog"

	"github.com/softwaresalt/backlogit/internal/core/gate"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/models"
)

// gateShipmentCompletion enforces the shipment-level two-level gate before a
// shipment is marked shipped (082-F ST4.2). It runs only when a gate broker is
// wired AND gates are enforceable; under enabled:false or a fail-open (auto with
// no usable autoharness / no resolvable base) it returns nil so the pre-gate ship
// behavior is preserved.
//
// Two independent checks, BOTH of which must pass:
//
//  1. member-evidence: every task/subtask member in the release scope MUST already
//     be terminal AND carry a passing (or forced) pre-task-completion gate
//     evidence event. This is the reconciliation guarantee — the ship path never
//     auto-completes an ungated member through completeReleaseScope, so release
//     finalization can never become a gate bypass.
//  2. shipment-diff: a shipment-level `autoharness gate check` over the full
//     shipment diff (no --task) must return a proceed decision.
//
// On refusal it returns a typed gate error and performs NO shipment state change.
func gateShipmentCompletion(ctx context.Context, ws *Workspace, shipmentID string, releaseScope []string) error {
	if ws == nil || ws.GateBroker == nil {
		return nil // gate disabled (enabled:false) or unwired.
	}

	// Shipment-level aggregate gate check over the full diff. NoCount: this
	// aggregate invocation is advisory to autoharness's per-task failure counter
	// (the per-task completion path is authoritative; we never stack a second
	// breaker at the shipment level).
	ev, err := ws.GateBroker.Evaluate(ctx, gate.Request{
		WorkspaceRoot: ws.RootPath,
		NoCount:       true,
	})
	if err != nil {
		class := shipmentGateErrorClass(err)
		ws.appendGateErrorEvidence(ctx, shipmentID, class, err.Error(), nil, nil)
		ge := gateErrorFromClass(class, shipmentID, nil, nil)
		ge.Message = fmt.Sprintf("shipment %s gate check: %s", shipmentID, err.Error())
		return ge
	}
	if !ev.Enforced {
		// Gates are not enforceable in this environment (auto fail-open): do not
		// impose member-evidence or shipment-diff enforcement.
		return nil
	}

	// (1) member-evidence validation (cheap log scan, no state change).
	if merr := validateMemberGateEvidence(ctx, ws, releaseScope, ws.headSHA(ctx)); merr != nil {
		return merr
	}

	// (2) shipment-diff decision. Redirects have no meaning at the shipment level,
	// so every non-proceed decision collapses to a blocked refusal that leaves
	// shipment state unchanged.
	if ev.Decision.Kind != gate.DecisionProceed {
		be := &blerrors.GateBlockedError{
			ItemID:       shipmentID,
			OldStatus:    string(models.StatusActive),
			NewStatus:    string(models.StatusActive),
			Outcome:      "blocked",
			StateChanged: false,
			BaseRef:      ev.Base.Ref,
			HeadRef:      ev.HeadRef,
			ExitCode:     ev.Decision.ExitCode,
			ReportJSON:   ev.Decision.ReportJSON,
			Stderr:       ev.Decision.Stderr,
			Repeated:     toErrRepeated(ev.Decision.RepeatedFailure),
		}
		if aerr := ws.appendGateEvent(ctx, shipmentID, EventGateBlocked, map[string]any{
			"level":    "shipment",
			"outcome":  "blocked",
			"base_ref": ev.Base.Ref,
			"head_ref": ev.HeadRef,
		}); aerr != nil {
			// Best-effort audit on a refusal path: the ship is already blocked, so a
			// failed evidence append must not mask the GateBlockedError below.
			slog.WarnContext(ctx, "shipment gate: failed to append blocked evidence", "shipment", shipmentID, "error", aerr)
		}
		return fmt.Errorf("shipment %s blocked by shipment-level gate check: %w", shipmentID, be)
	}

	// Both checks passed: record shipment-level passing evidence.
	if aerr := ws.appendGateEvent(ctx, shipmentID, EventGatePassed, map[string]any{
		"level":    "shipment",
		"outcome":  "passed",
		"base_ref": ev.Base.Ref,
		"head_ref": ev.HeadRef,
		"ran":      ev.Ran,
	}); aerr != nil && ws.gateConfig.EvidenceRequiredValue() {
		return fmt.Errorf("shipment %s gate evidence append failed, refusing ship: %w", shipmentID, aerr)
	}
	return nil
}

// validateMemberGateEvidence verifies every task/subtask member in the release
// scope is terminal AND carries passing (or forced) gate evidence. When
// shipmentHead is non-empty, the member's latest evidence must match that head
// SHA — evidence recorded at a prior head is rejected as stale. Non-gated member
// types (feature/other) are skipped.
func validateMemberGateEvidence(ctx context.Context, ws *Workspace, releaseScope []string, shipmentHead string) error {
	logsRoot := WorkspaceLogsRoot(ws.RootPath)
	for _, id := range releaseScope {
		item, err := loadArtifact(ctx, ws, id)
		if err != nil {
			return fmt.Errorf("validate member evidence: load %s: %w", id, err)
		}
		if item.ArtifactType != "task" && item.ArtifactType != "subtask" {
			continue
		}
		// A non-terminal gated member has not been completed through the gate;
		// shipping MUST NOT silently auto-complete it (release finalization is not
		// a gate bypass).
		if !isTerminalReleaseStatus(item.Status) {
			return shipmentMemberEvidenceError(id, fmt.Sprintf("is %s (not completed through the gate)", item.Status))
		}
		evs, rerr := events.ReadAllEvents(ctx, logsRoot, id)
		if rerr != nil {
			return fmt.Errorf("validate member evidence: read events for %s: %w", id, rerr)
		}
		latest := latestGatePassEvidence(evs)
		if latest == nil {
			return shipmentMemberEvidenceError(id, "missing passing gate evidence")
		}
		if shipmentHead != "" {
			if h, _ := latest.Delta["head_sha"].(string); h != "" && h != shipmentHead {
				return shipmentMemberEvidenceError(id, "gate evidence is stale (recorded at a prior head)")
			}
		}
	}
	return nil
}

// latestGatePassEvidence returns the most recent gate evidence event that
// satisfies the composed member-evidence predicate (082-F F4 hardening,
// 083.002-T): an EventGateForced (unconditional audited break-glass) OR an
// EventGatePassed whose delta records ran==true. A fail-open EventGatePassed
// no-run (ran missing/false) is NOT valid evidence and is skipped, so an earlier
// forced/ran==true event is promoted to the returned latest (the head_sha
// staleness check then runs against that event — intended per Decision 2).
// Returns nil when no qualifying event is present.
func latestGatePassEvidence(evs []events.Event) *events.Event {
	var latest *events.Event
	for i := range evs {
		switch evs[i].EventType {
		case EventGateForced:
			e := evs[i]
			latest = &e
		case EventGatePassed:
			// Comma-ok read: a missing or non-bool "ran" yields false, correctly
			// treated as not-ran (mirrors the head_sha idiom above).
			if ran, _ := evs[i].Delta["ran"].(bool); ran {
				e := evs[i]
				latest = &e
			}
		}
	}
	return latest
}

// shipmentMemberEvidenceError builds a typed blocked refusal for a member that
// cannot be released, wrapping the member context so callers route via errors.As
// while surfacing the offending member and reason.
func shipmentMemberEvidenceError(memberID, reason string) error {
	be := &blerrors.GateBlockedError{
		ItemID:       memberID,
		Outcome:      "blocked",
		OldStatus:    string(models.StatusActive),
		NewStatus:    string(models.StatusActive),
		StateChanged: false,
	}
	return fmt.Errorf("shipment refused: member %s %s: %w", memberID, reason, be)
}

// shipmentGateErrorClass classifies a broker Evaluate error (probe / base-ref
// resolution) into a gate error class for evidence and typed-error construction.
func shipmentGateErrorClass(err error) string {
	var ge *blerrors.GateError
	if stderrors.As(err, &ge) {
		return ge.Class
	}
	switch {
	case stderrors.Is(err, blerrors.ErrGateSetup):
		return "setup"
	case stderrors.Is(err, blerrors.ErrGateTimeout):
		return "timeout"
	default:
		return "config"
	}
}
