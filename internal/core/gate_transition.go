package core

import (
	"context"
	stderrors "errors"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core/gate"
	bkerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/models"
)

// ForceSource identifies where a gate force override originated. Force is an
// operator-only, CLI-only privilege (force_cli_only); an MCP-originated force is
// rejected before any gate runs.
type ForceSource int

const (
	// ForceSourceNone means no force was requested.
	ForceSourceNone ForceSource = iota
	// ForceSourceCLI means the force came from the operator via the CLI.
	ForceSourceCLI
)

// TransitionOptions carries the operator-influenced inputs to a gated transition.
// The zero value is a normal, non-forced transition with auto base resolution.
type TransitionOptions struct {
	// GateBase is the operator-only --gate-base override. Never set from MCP.
	GateBase string
	// Force passes --force to autoharness (which audits it) and records a
	// backlogit forced event. Operator-only via the CLI.
	Force bool
	// ForceReason is the mandatory operator justification recorded in evidence.
	ForceReason string
	// ForceSource identifies the origin of a force request (CLI only is honored).
	ForceSource ForceSource
}

// GateOutcome is the structured result of a gated transition, returned alongside
// the (possibly refused) artifact so CLI/MCP callers can render machine output
// without re-parsing the gate report.
type GateOutcome struct {
	ItemID          string
	OldStatus       string
	NewStatus       string
	Outcome         string // "passed" | "blocked" | "requeued" | "escalated" | "error"
	StateChanged    bool
	BaseRef         string
	HeadRef         string
	HeadSHA         string
	GateReportHash  string
	RepeatedFailure *gate.RepeatedFailure
	Forced          bool
	Ran             bool
}

// buildGateBroker constructs the default os/exec-backed gate broker from the
// normalized gate config. It is called at workspace construction unless the gate
// is disabled (enabled:false).
func buildGateBroker(root string, cfg config.PreTaskCompletionGateConfig) *gate.Broker {
	env := gate.MinimalEnv()
	return &gate.Broker{
		Runner:         gate.ExecRunner{Binary: cfg.AutoharnessBinary},
		Git:            gate.ExecGitRunner{Dir: root, Env: env},
		Version:        gate.ExecVersionRunner{Binary: cfg.AutoharnessBinary, Dir: root, Env: env},
		Enabled:        gate.EnabledMode(cfg.Enabled),
		ConfigBaseRef:  cfg.BaseRef,
		TimeoutSeconds: cfg.TimeoutSeconds,
	}
}

// UpdateArtifactWithGate updates an artifact, engaging the pre-task-completion
// gate broker (082-F) when the transition is a task/subtask entering a terminal
// status from a non-terminal one and a broker is wired. All other transitions run
// the ungated path unchanged. On a gated completion it returns a GateOutcome; on a
// refusal it returns a typed *GateBlockedError or *GateError so cli/mcp map exit
// codes and structured errors via errors.As/errors.Is.
func UpdateArtifactWithGate(ctx context.Context, ws *Workspace, id string, updates map[string]any, opts TransitionOptions) (*models.Artifact, *GateOutcome, error) {
	if _, hasID := updates["id"]; hasID {
		return nil, nil, fmt.Errorf("field %q is immutable and cannot be changed", "id")
	}

	// Cheap peek (no lock) to decide whether the gate applies.
	peek, err := findArtifact(ctx, ws, id)
	if err != nil {
		return nil, nil, fmt.Errorf("find artifact %s: %w", id, err)
	}
	if !ws.gateApplies(peek, updates) {
		artifact, uErr := updateArtifactUngated(ctx, ws, id, updates)
		return artifact, nil, uErr
	}
	return ws.runGatedCompletion(ctx, id, updates, opts)
}

// gateApplies reports whether the pre-task-completion gate must run for this
// transition: a broker is wired, the item is a task or subtask, the update sets a
// terminal status, and the item is not already terminal.
func (ws *Workspace) gateApplies(a *models.Artifact, updates map[string]any) bool {
	if ws == nil || ws.GateBroker == nil || a == nil {
		return false
	}
	if a.ArtifactType != "task" && a.ArtifactType != "subtask" {
		return false
	}
	newStatus, ok := updates["status"].(string)
	if !ok || newStatus == "" {
		return false
	}
	if !ws.isGateTerminalStatus(newStatus) {
		return false
	}
	// Never re-gate an item that is already terminal (idempotent completions).
	return !ws.isGateTerminalStatus(string(a.Status))
}

// isGateTerminalStatus reports whether status is one of the gate's configured
// terminal statuses (default ["done"]).
func (ws *Workspace) isGateTerminalStatus(status string) bool {
	terms := ws.gateConfig.TerminalStatuses
	if len(terms) == 0 {
		terms = []string{"done"}
	}
	for _, t := range terms {
		if t == status {
			return true
		}
	}
	return false
}

// runGatedCompletion runs the full gated completion under the per-task lock:
// bounded-wait lock -> reread (authoritative old_status) -> broker.Evaluate ->
// apply exactly one durable write (or refuse) -> record evidence. Evidence is
// appended BEFORE the durable write so that under evidence_required no completion
// can persist without its audit record (no rollback path is needed).
func (ws *Workspace) runGatedCompletion(ctx context.Context, id string, updates map[string]any, opts TransitionOptions) (*models.Artifact, *GateOutcome, error) {
	requestedStatus, _ := updates["status"].(string)

	// Enforce force_cli_only: reject a force that did not originate from the CLI.
	if opts.Force && ws.gateConfig.ForceCLIOnlyValue() && opts.ForceSource != ForceSourceCLI {
		return nil, nil, &bkerrors.GateError{
			Class:   "config",
			ItemID:  id,
			Message: "gate force is operator-only via the CLI (force_cli_only)",
			Err:     bkerrors.ErrGateConfig,
		}
	}

	path, err := FindArtifactPath(ctx, ws, id)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve artifact path %s: %w", id, err)
	}

	unlock, err := lockTaskFileWithHeartbeat(ctx, path, defaultGateLockBoundedWait, defaultGateLockHeartbeat)
	if err != nil {
		if stderrors.Is(err, bkerrors.ErrGateInProgress) {
			return nil, nil, &bkerrors.GateError{
				Class:   "in_progress",
				ItemID:  id,
				Message: err.Error(),
				Err:     bkerrors.ErrGateInProgress,
			}
		}
		return nil, nil, err
	}
	defer func() { _ = unlock() }()

	// Reread under the lock: this status is the authoritative old_status.
	current, err := findArtifact(ctx, ws, id)
	if err != nil {
		return nil, nil, fmt.Errorf("reread artifact %s: %w", id, err)
	}
	oldStatus := string(current.Status)

	// Another writer may have completed the item between the peek and the lock.
	if ws.isGateTerminalStatus(oldStatus) {
		artifact, uErr := updateArtifactUngated(ctx, ws, id, updates)
		return artifact, nil, uErr
	}

	ev, evalErr := ws.GateBroker.Evaluate(ctx, gate.Request{
		ItemID:        id,
		WorkspaceRoot: ws.RootPath,
		GateBase:      opts.GateBase,
		Force:         opts.Force,
		NoCount:       false, // the completion path is the authoritative counter.
	})
	if evalErr != nil {
		return ws.handleGateSetupError(ctx, id, oldStatus, evalErr)
	}

	// F1 (083.001-T): advisory only. When a pinned config base_ref shadows an
	// operator --gate-base, the config base wins (config-first precedence is
	// unchanged) but the operator's override is silently ignored — surface a
	// warning so the shadowing is visible. No behavior/precedence change.
	if ev.Base.OverrideShadowed {
		slog.WarnContext(ctx, "gate base override shadowed: --gate-base is ignored because config base_ref is pinned (config-first precedence)",
			"item_id", id, "resolved_base", ev.Base.Ref, "gate_base", strings.TrimSpace(opts.GateBase))
	}

	// Audit an operator base override before applying the decision. Fires for any
	// non-default resolved base AND whenever the operator explicitly passed
	// --gate-base (even if it happens to equal the default ref), so a privileged
	// break-glass never escapes the audit trail.
	if ev.Base.NonDefault || strings.TrimSpace(opts.GateBase) != "" {
		if oErr := ws.recordGateBaseOverride(ctx, id, ev, opts); oErr != nil {
			slog.WarnContext(ctx, "append gate base-override evidence", "item_id", id, "error", oErr)
		}
	}

	switch ev.Decision.Kind {
	case gate.DecisionProceed:
		return ws.completeGatePass(ctx, id, updates, requestedStatus, oldStatus, ev, opts)
	case gate.DecisionRedirectQueued:
		return ws.redirectGate(ctx, id, current, oldStatus, "queued", "requeued", EventGateRequeued, ev)
	case gate.DecisionRedirectBlocked:
		return ws.redirectGate(ctx, id, current, oldStatus, "blocked", "escalated", EventGateEscalated, ev)
	case gate.DecisionBlock:
		return ws.blockGate(ctx, id, oldStatus, ev)
	case gate.DecisionError:
		return ws.errorGate(ctx, id, oldStatus, ev.Decision)
	default:
		return nil, nil, fmt.Errorf("unknown gate decision kind %d", ev.Decision.Kind)
	}
}

// completeGatePass applies a passing completion: evidence first, then the durable
// write via the ungated update (which validates the terminal transition and emits
// standard lifecycle events).
func (ws *Workspace) completeGatePass(ctx context.Context, id string, updates map[string]any, requestedStatus, oldStatus string, ev gate.Evaluation, opts TransitionOptions) (*models.Artifact, *GateOutcome, error) {
	outcome := ws.newOutcome(ctx, id, oldStatus, requestedStatus, "passed", true, ev)
	outcome.Forced = opts.Force

	if err := ws.appendGateEvidence(ctx, id, EventGatePassed, outcome, &opts); err != nil {
		if ws.gateConfig.EvidenceRequiredValue() {
			return nil, nil, fmt.Errorf("gate evidence append failed, refusing completion of %s: %w", id, err)
		}
		slog.WarnContext(ctx, "gate pass evidence append failed (evidence not required)", "item_id", id, "error", err)
	}
	if opts.Force {
		if err := ws.appendGateEvidence(ctx, id, EventGateForced, outcome, &opts); err != nil {
			// Force is the operator break-glass; its audit record is the whole point
			// of forcing. Under evidence_required, a failed forced-audit append must
			// refuse the completion rather than silently persist a forced transition
			// with no audit trail (parity with the pass-evidence path above).
			if ws.gateConfig.EvidenceRequiredValue() {
				return nil, nil, fmt.Errorf("forced-gate evidence append failed, refusing completion of %s: %w", id, err)
			}
			slog.WarnContext(ctx, "gate forced evidence append failed (evidence not required)", "item_id", id, "error", err)
		}
	}

	artifact, err := updateArtifactUngated(ctx, ws, id, updates)
	if err != nil {
		return nil, nil, err
	}
	return artifact, outcome, nil
}

// redirectGate performs an authoritative requeue/escalate: the item moves to the
// redirect target (queued or blocked) and the caller receives a *GateBlockedError.
// The redirect write bypasses the user-facing transition validator because the
// gate is the completion authority deciding this backward move.
func (ws *Workspace) redirectGate(ctx context.Context, id string, current *models.Artifact, oldStatus, target, outcomeName, eventType string, ev gate.Evaluation) (*models.Artifact, *GateOutcome, error) {
	outcome := ws.newOutcome(ctx, id, oldStatus, target, outcomeName, true, ev)

	if err := ws.appendGateEvidence(ctx, id, eventType, outcome, nil); err != nil {
		if ws.gateConfig.EvidenceRequiredValue() {
			return nil, nil, fmt.Errorf("gate evidence append failed, refusing redirect of %s: %w", id, err)
		}
		slog.WarnContext(ctx, "gate redirect evidence append failed (evidence not required)", "item_id", id, "error", err)
	}

	artifact, err := ws.writeStatusDirect(ctx, current, oldStatus, target)
	if err != nil {
		return nil, nil, err
	}
	return artifact, outcome, ws.blockedError(id, oldStatus, target, outcomeName, true, ev)
}

// blockGate refuses a below-threshold completion. No durable write occurs; the
// item retains its old status. Evidence is best-effort here (there is no state
// change to protect).
func (ws *Workspace) blockGate(ctx context.Context, id, oldStatus string, ev gate.Evaluation) (*models.Artifact, *GateOutcome, error) {
	outcome := ws.newOutcome(ctx, id, oldStatus, oldStatus, "blocked", false, ev)
	if err := ws.appendGateEvidence(ctx, id, EventGateBlocked, outcome, nil); err != nil {
		slog.WarnContext(ctx, "gate block evidence append failed", "item_id", id, "error", err)
	}
	return nil, outcome, ws.blockedError(id, oldStatus, oldStatus, "blocked", false, ev)
}

// errorGate refuses a completion due to a setup/config/timeout-class decision. No
// durable write occurs.
func (ws *Workspace) errorGate(ctx context.Context, id, oldStatus string, dec gate.GateDecision) (*models.Artifact, *GateOutcome, error) {
	class := string(dec.ErrorClass)
	if class == "" {
		class = "config"
	}
	ws.appendGateErrorEvidence(ctx, id, class, "", dec.ReportJSON, dec.Stderr)
	outcome := &GateOutcome{ItemID: id, OldStatus: oldStatus, NewStatus: oldStatus, Outcome: "error", StateChanged: false}
	return nil, outcome, gateErrorFromClass(class, id, dec.ReportJSON, dec.Stderr)
}

// handleGateSetupError converts a broker probe/base-resolution error into a typed
// *GateError and records error evidence. No durable write occurs.
func (ws *Workspace) handleGateSetupError(ctx context.Context, id, oldStatus string, evalErr error) (*models.Artifact, *GateOutcome, error) {
	var ge *bkerrors.GateError
	if stderrors.As(evalErr, &ge) {
		if ge.ItemID == "" {
			ge.ItemID = id
		}
		ws.appendGateErrorEvidence(ctx, id, ge.Class, ge.Error(), ge.ReportJSON, ge.Stderr)
		outcome := &GateOutcome{ItemID: id, OldStatus: oldStatus, NewStatus: oldStatus, Outcome: "error", StateChanged: false}
		return nil, outcome, ge
	}

	class := "config"
	switch {
	case stderrors.Is(evalErr, bkerrors.ErrGateSetup):
		class = "setup"
	case stderrors.Is(evalErr, bkerrors.ErrGateTimeout):
		class = "timeout"
	}
	ws.appendGateErrorEvidence(ctx, id, class, evalErr.Error(), nil, nil)
	outcome := &GateOutcome{ItemID: id, OldStatus: oldStatus, NewStatus: oldStatus, Outcome: "error", StateChanged: false}
	ge = gateErrorFromClass(class, id, nil, nil)
	ge.Message = evalErr.Error()
	return nil, outcome, ge
}

// writeStatusDirect applies a status-only durable write bypassing the transition
// validator (used for authoritative gate redirects). It preserves field/schema
// validation and status-directory relocation.
func (ws *Workspace) writeStatusDirect(ctx context.Context, a *models.Artifact, oldStatus, newStatus string) (*models.Artifact, error) {
	a.Status = models.ArtifactStatus(newStatus)
	a.UpdatedAt = time.Now()
	clearStaleBlockedReason(a, models.ArtifactStatus(oldStatus))

	if err := requireHeaderDef(ws); err != nil {
		return nil, err
	}
	if err := ValidateArtifactFields(a, ws.HeaderDef); err != nil {
		return nil, fmt.Errorf("validate artifact fields: %w", err)
	}
	if err := a.Validate(); err != nil {
		return nil, fmt.Errorf("validate artifact: %w", err)
	}
	relocate := shouldRelocateOnStatusChange(models.ArtifactStatus(oldStatus), models.ArtifactStatus(newStatus))
	if err := persistArtifact(ctx, ws, a, relocate); err != nil {
		return nil, fmt.Errorf("persist artifact %s: %w", a.ID, err)
	}
	return a, nil
}

// newOutcome builds a GateOutcome from an evaluation, computing the best-effort
// HEAD SHA and report hash.
func (ws *Workspace) newOutcome(ctx context.Context, id, oldStatus, newStatus, outcomeName string, stateChanged bool, ev gate.Evaluation) *GateOutcome {
	return &GateOutcome{
		ItemID:          id,
		OldStatus:       oldStatus,
		NewStatus:       newStatus,
		Outcome:         outcomeName,
		StateChanged:    stateChanged,
		BaseRef:         ev.Base.Ref,
		HeadRef:         ev.HeadRef,
		HeadSHA:         ws.headSHA(ctx),
		GateReportHash:  gateReportHash(ev.Decision.ReportJSON),
		RepeatedFailure: ev.Decision.RepeatedFailure,
		Ran:             ev.Ran,
	}
}

// blockedError builds the typed *GateBlockedError returned on block/redirect.
func (ws *Workspace) blockedError(id, oldStatus, newStatus, outcomeName string, stateChanged bool, ev gate.Evaluation) *bkerrors.GateBlockedError {
	return &bkerrors.GateBlockedError{
		ItemID:       id,
		OldStatus:    oldStatus,
		NewStatus:    newStatus,
		Outcome:      outcomeName,
		StateChanged: stateChanged,
		BaseRef:      ev.Base.Ref,
		HeadRef:      ev.HeadRef,
		ExitCode:     ev.Decision.ExitCode,
		ReportJSON:   ev.Decision.ReportJSON,
		Stderr:       ev.Decision.Stderr,
		Repeated:     toErrRepeated(ev.Decision.RepeatedFailure),
	}
}

// appendGateEvidence records a durable gate evidence event for a state-changing
// outcome. It returns the append error so evidence_required callers can refuse.
func (ws *Workspace) appendGateEvidence(ctx context.Context, id, eventType string, outcome *GateOutcome, opts *TransitionOptions) error {
	delta := map[string]any{
		"outcome":       outcome.Outcome,
		"old_status":    outcome.OldStatus,
		"new_status":    outcome.NewStatus,
		"state_changed": outcome.StateChanged,
		"base_ref":      outcome.BaseRef,
		"head_ref":      outcome.HeadRef,
		"ran":           outcome.Ran,
	}
	if outcome.HeadSHA != "" {
		delta["head_sha"] = outcome.HeadSHA
	}
	if outcome.GateReportHash != "" {
		delta["gate_report_hash"] = outcome.GateReportHash
	}
	if rf := outcome.RepeatedFailure; rf != nil {
		delta["repeated_failure"] = map[string]any{
			"count":     rf.Count,
			"threshold": rf.Threshold,
			"reached":   rf.Reached,
			"action":    rf.Action,
		}
	}
	if opts != nil && opts.Force {
		delta["forced"] = true
		if opts.ForceReason != "" {
			delta["force_reason"] = opts.ForceReason
		}
	}
	return ws.appendGateEvent(ctx, id, eventType, delta)
}

// appendGateErrorEvidence records a best-effort error-class gate event.
func (ws *Workspace) appendGateErrorEvidence(ctx context.Context, id, class, message string, report, stderrOut []byte) {
	delta := map[string]any{"class": class}
	if message != "" {
		delta["message"] = message
	}
	if h := gateReportHash(report); h != "" {
		delta["gate_report_hash"] = h
	}
	if len(stderrOut) > 0 {
		delta["stderr"] = truncateStderr(stderrOut)
	}
	if err := ws.appendGateEvent(ctx, id, EventGateError, delta); err != nil {
		slog.WarnContext(ctx, "append gate error evidence", "item_id", id, "error", err)
	}
}

// recordGateBaseOverride records the privileged base-override audit event.
func (ws *Workspace) recordGateBaseOverride(ctx context.Context, id string, ev gate.Evaluation, opts TransitionOptions) error {
	delta := map[string]any{
		"base_ref": ev.Base.Ref,
		"source":   ev.Base.Source,
	}
	if opts.GateBase != "" {
		delta["gate_base"] = opts.GateBase
	}
	if opts.ForceReason != "" {
		delta["reason"] = opts.ForceReason
	}
	return ws.appendGateEvent(ctx, id, EventGateBaseOverride, delta)
}

// headSHA returns the current HEAD commit SHA best-effort (empty on any error).
func (ws *Workspace) headSHA(ctx context.Context) string {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "HEAD")
	cmd.Dir = ws.RootPath
	cmd.Env = gate.MinimalEnv()
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// gateErrorFromClass builds a typed *GateError wrapping the matching sentinel.
func gateErrorFromClass(class, id string, report, stderrOut []byte) *bkerrors.GateError {
	var sentinel error
	switch class {
	case "setup":
		sentinel = bkerrors.ErrGateSetup
	case "timeout":
		sentinel = bkerrors.ErrGateTimeout
	case "in_progress":
		sentinel = bkerrors.ErrGateInProgress
	default:
		class = "config"
		sentinel = bkerrors.ErrGateConfig
	}
	return &bkerrors.GateError{
		Class:      class,
		ItemID:     id,
		ReportJSON: report,
		Stderr:     stderrOut,
		Err:        sentinel,
	}
}

// toErrRepeated converts the gate repeated-failure value into the errors-leaf copy.
func toErrRepeated(rf *gate.RepeatedFailure) *bkerrors.GateRepeatedFailure {
	if rf == nil {
		return nil
	}
	return &bkerrors.GateRepeatedFailure{
		Count:     rf.Count,
		Threshold: rf.Threshold,
		Reached:   rf.Reached,
		Action:    rf.Action,
	}
}

// truncateStderr bounds human-facing stderr in evidence deltas. The full stderr is
// preserved for machine callers on the typed error, not here.
func truncateStderr(b []byte) string {
	const max = 2048
	if len(b) <= max {
		return string(b)
	}
	return string(b[:max]) + "...[truncated]"
}
