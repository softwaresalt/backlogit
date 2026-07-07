package core

import (
	"bytes"
	"context"
	stderrors "errors"
	"fmt"
	"log/slog"
	"os/exec"
	"regexp"
	"time"

	"github.com/softwaresalt/backlogit/internal/core/gate"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/gateevidence"
	"github.com/softwaresalt/backlogit/internal/models"
)

// ancestryCheckTimeout bounds each git lineage/head-resolution subprocess when
// the gate broker does not supply an explicit timeout. The shipment ship path is
// unbounded and holds the workspace lock across completion, so an unbounded git
// child would pin the lock indefinitely (a denial of service). Every helper that
// spawns git here derives its OWN deadline from this default (or
// GateBroker.TimeoutSeconds when configured) — it never relies on the caller
// imposing a deadline.
const ancestryCheckTimeout = 5 * time.Second

// gitObjectNameRe matches exactly the full-length object names git rev-parse can
// produce: a 40-hex SHA-1 or a 64-hex SHA-256. Abbreviations, refs, and any value
// containing a leading dash or non-hex byte are rejected, so a tampered on-disk
// head_sha can never be handed to git as an option or an ambiguous ref.
var gitObjectNameRe = regexp.MustCompile(`^([0-9a-fA-F]{40}|[0-9a-fA-F]{64})$`)

// isGitObjectName reports whether s is a full-length git object name (SHA-1 or
// SHA-256). It is the input-validation guard applied to the untrusted recorded
// member head_sha before it is passed to git (argument-injection defense: "data
// must not choose the args").
func isGitObjectName(s string) bool {
	return gitObjectNameRe.MatchString(s)
}

// isAncestor reports whether ancestor is an ancestor of (or equal to) descendant
// by running `git merge-base --is-ancestor ancestor descendant` under a mandatory
// self-derived deadline. It is a security guard on the shipment ship path, so it
// FAILS CLOSED: any timeout, cancellation, exec failure, or non-{0,1} exit code
// returns a non-nil error (never a silent pass). Exit-code semantics:
//
//	0 -> ancestor or equal          -> (true, nil)
//	1 -> definitively not-ancestor  -> (false, nil)
//	other / exec error / timeout    -> (false, error)  [fail closed]
//
// argv-array exec + gate.MinimalEnv() preserve the workspace exec trust boundary
// (no shell, allowlisted env). Both operands are expected to already satisfy
// isGitObjectName / trusted-provenance at the call site.
func (ws *Workspace) isAncestor(ctx context.Context, ancestor, descendant string) (bool, error) {
	d := ancestryCheckTimeout
	if ws.GateBroker != nil && ws.GateBroker.TimeoutSeconds > 0 {
		d = time.Duration(ws.GateBroker.TimeoutSeconds) * time.Second
	}
	runCtx, cancel := context.WithTimeout(ctx, d)
	defer cancel()

	var stderr bytes.Buffer
	cmd := exec.CommandContext(runCtx, "git", "merge-base", "--is-ancestor", ancestor, descendant)
	cmd.Dir = ws.RootPath
	cmd.Env = gate.MinimalEnv()
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	if runErr == nil {
		return true, nil // exit 0: ancestor or equal.
	}

	// A timeout OR cancellation MUST be detected before any exit code is read: a
	// context-killed git reports a platform-dependent exit code (e.g. 1 on
	// Windows, -1 on POSIX) that must never be misread as the exit-1
	// "not-an-ancestor" signal. Both DeadlineExceeded and Canceled fail closed.
	if ctxErr := runCtx.Err(); ctxErr != nil {
		return false, fmt.Errorf("ancestor check aborted (%v): %w", ctxErr, ctxErr)
	}

	var ee *exec.ExitError
	if stderrors.As(runErr, &ee) {
		if ee.ExitCode() == 1 {
			return false, nil // definitively not an ancestor.
		}
		// Exit 128 (bad object / shallow boundary) and any other non-{0,1} code
		// are unverifiable lineage -> fail closed, preserving git's diagnostic.
		return false, fmt.Errorf("git merge-base --is-ancestor exit %d: %s: %w",
			ee.ExitCode(), bytes.TrimSpace(stderr.Bytes()), runErr)
	}
	// git binary missing or any other non-ExitError failure -> fail closed.
	return false, fmt.Errorf("run git merge-base --is-ancestor: %w", runErr)
}

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
	// so every non-proceed, non-error decision collapses to a blocked refusal that
	// leaves shipment state unchanged.
	//
	// F5 (083.003-T): a setup/config/timeout-class DecisionError must preserve its
	// exit 7/8 class fidelity rather than collapsing to a GateBlockedError (exit 6).
	// This mirrors the task-level errorGate (gate_transition.go) and the broker
	// Evaluate-error branch above. A shipment-level timeout reaches here as
	// Kind==DecisionError with a nil Evaluate error, so this is the correct seam.
	if ev.Decision.Kind == gate.DecisionError {
		class := string(ev.Decision.ErrorClass)
		if class == "" {
			class = "config"
		}
		ws.appendGateErrorEvidence(ctx, shipmentID, class, "", ev.Decision.ReportJSON, ev.Decision.Stderr)
		ge := gateErrorFromClass(class, shipmentID, ev.Decision.ReportJSON, ev.Decision.Stderr)
		ge.Message = fmt.Sprintf("shipment %s gate check %s error", shipmentID, class)
		return ge
	}
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
// shipmentHead is non-empty, the member's recorded evidence head must be an
// ANCESTOR OF (or equal to) that shipment head — i.e. the gated commit is
// contained in the shipment history (verified with git merge-base
// --is-ancestor). This replaces the prior strict head_sha equality, which falsely
// rejected valid post-merge evidence (a member's build commit is an ancestor of,
// not equal to, the shipment's merge commit). A genuinely divergent (non-ancestor)
// head, a malformed head_sha, or an unverifiable lineage (git error/timeout/cancel)
// is refused (fail closed). An empty member head is bypassed unchanged (B85DAEE8
// scope). Non-gated member types (feature/other) are skipped.
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
				// h != "" preserves the empty-member-head bypass (B85DAEE8, out of
				// scope). h == shipmentHead is the equality fast-path: an equal head
				// never enters this block, so a single-commit shipment needs no repo
				// access and no subprocess.
				if !isGitObjectName(h) {
					// The recorded head comes from tamperable on-disk evidence JSONL;
					// a value that is not a git object name is never handed to git.
					slog.WarnContext(ctx, "member evidence head_sha is malformed",
						"member", id, "member_head", h, "shipment_head", shipmentHead)
					return shipmentMemberEvidenceError(id,
						"gate evidence head_sha is malformed (not a git object name)")
				}
				included, aerr := ws.isAncestor(ctx, h, shipmentHead)
				if aerr != nil {
					// A security guard must never silently pass on an unverifiable
					// lineage (git error / timeout / cancel): fail closed.
					slog.WarnContext(ctx, "member evidence lineage check failed",
						"member", id, "member_head", h, "shipment_head", shipmentHead, "error", aerr)
					return shipmentMemberEvidenceError(id,
						fmt.Sprintf("cannot verify gate evidence lineage: %v", aerr))
				}
				if !included {
					// A real, reachable, but non-ancestor head: the gated work is not
					// contained in the shipment history — genuinely stale/divergent.
					return shipmentMemberEvidenceError(id,
						"gate evidence is stale (recorded at a divergent head)")
				}
			}
		}
	}
	return nil
}

// latestGatePassEvidence returns the most recent gate evidence event that
// satisfies the composed member-evidence predicate (082-F F4 hardening,
// 083.002-T). As of Q3.0 (083.005.001-ST) the predicate is owned by the shared
// internal/gateevidence leaf so core and db derive evidence identically across
// the one-way core->db boundary; this wrapper delegates and returns the selected
// event (nil when no qualifying event is present) to preserve the existing
// caller contract (nil-check + head_sha staleness read).
func latestGatePassEvidence(evs []events.Event) *events.Event {
	return gateevidence.Latest(evs).Event
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
