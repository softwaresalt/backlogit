package gate

import (
	"context"
	"strings"
	"time"
)

// Request carries the per-transition inputs the broker needs to evaluate a gate.
type Request struct {
	// ItemID is the backlog item being completed (interpolated as --task).
	ItemID string
	// WorkspaceRoot is the repository root passed as --workspace and cmd.Dir.
	WorkspaceRoot string
	// GateBase is the operator-only --gate-base override (never set from MCP).
	GateBase string
	// Force passes --force to autoharness (operator-only; autoharness audits it).
	Force bool
	// NoCount uses the advisory --no-count mode (never on the authoritative
	// completion path; reserved for advisory callers).
	NoCount bool
}

// Evaluation is the broker's verdict for a transition.
type Evaluation struct {
	// Decision is the typed decision to apply (proceed/redirect/block/error).
	Decision GateDecision
	// Base is the resolved base ref (Base.NonDefault triggers an audit event).
	Base ResolvedBase
	// HeadRef is always the fixed HEAD constant.
	HeadRef string
	// Ran reports whether the gate process actually executed (false on a
	// fail-open no-run under auto).
	Ran bool
	// Enforced reports whether gates were enforceable (probe passed).
	Enforced bool
}

// Broker is the autoharness integration orchestrator. It owns probe -> base
// resolution -> run -> parse -> decide, returning a typed Evaluation. It performs
// NO durable state writes, locking, or evidence persistence — those belong to
// package core, which drives this boundary.
type Broker struct {
	Runner         GateRunner
	Git            GitRunner
	Version        VersionRunner
	Enabled        EnabledMode
	ConfigBaseRef  string
	TimeoutSeconds int
	// EnvFn supplies the allowlisted subprocess environment. Defaults to MinimalEnv.
	EnvFn func() []string
}

// Mode reports the configured enablement.
func (b *Broker) Mode() EnabledMode { return b.Enabled }

// Evaluate runs the gate pipeline and returns the decision. A non-nil error is a
// typed setup/config refusal (from the probe or base-ref resolver); the caller
// records error evidence and refuses the transition without any state change.
func (b *Broker) Evaluate(ctx context.Context, req Request) (Evaluation, error) {
	ev := Evaluation{HeadRef: HeadRef}
	if b.Enabled == EnabledFalse {
		ev.Decision = GateDecision{Kind: DecisionProceed}
		return ev, nil
	}

	// Bound EVERY child process by the configured timeout — the version probe and
	// the git base-ref resolution as well as the gate check itself. The completion
	// path holds the per-task lock across this call and refreshes its heartbeat, so
	// an unbounded probe or git hang would pin the live lock indefinitely (a
	// denial of service on the item). Deriving the deadline before the probe caps
	// that: under auto a wedged probe times out and fails open; under strict it
	// fails closed.
	runCtx := ctx
	if b.TimeoutSeconds > 0 {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, time.Duration(b.TimeoutSeconds)*time.Second)
		defer cancel()
	}

	enforce, err := Probe(runCtx, b.Version, b.Enabled)
	if err != nil {
		// Setup-class refusal under enabled:true.
		return ev, err
	}
	if !enforce {
		// enabled:auto with an unresolvable/incompatible binary — fail open.
		ev.Decision = GateDecision{Kind: DecisionProceed}
		return ev, nil
	}
	ev.Enforced = true

	base, err := ResolveBaseRef(runCtx, b.Git, BaseRefInput{ConfigBaseRef: b.ConfigBaseRef, GateBase: req.GateBase})
	if err != nil {
		// An explicit operator base override (config base_ref != auto, or a
		// --gate-base) that cannot be verified is ALWAYS a config-class refusal,
		// regardless of mode: silently failing open on a mistyped privileged
		// override would suppress gating (attempt-2 Security P1).
		if hasExplicitBase(b.ConfigBaseRef, req.GateBase) {
			return ev, err
		}
		// Auto-discovery failure (no resolvable default branch / not a git repo /
		// no commits). Under strict enforcement this is a config error (fail
		// closed). Under auto the environment cannot support gating, so fail open
		// (proceed) — this preserves the non-negotiable invariant that a repo with
		// no configured gates still allows -> done.
		if b.Enabled == EnabledTrue {
			return ev, err
		}
		ev.Enforced = false
		ev.Decision = GateDecision{Kind: DecisionProceed}
		return ev, nil
	}
	ev.Base = base

	args := BuildArgs(GateCheckRequest{
		ItemID:        req.ItemID,
		BaseRef:       base.Ref,
		HeadRef:       HeadRef,
		WorkspaceRoot: req.WorkspaceRoot,
		Force:         req.Force,
		NoCount:       req.NoCount,
	})
	res, runErr := b.Runner.Run(runCtx, args, req.WorkspaceRoot, b.env())

	var rf *RepeatedFailure
	if report, ok := ParseReport(res.Stdout); ok {
		rf = report.RepeatedFailure
	}
	ev.Decision = Decide(b.Enabled, res, runErr, rf)
	ev.Ran = true
	return ev, nil
}

func (b *Broker) env() []string {
	if b.EnvFn != nil {
		return b.EnvFn()
	}
	return MinimalEnv()
}

// hasExplicitBase reports whether an operator supplied a non-auto base override
// through config base_ref or a caller --gate-base. An explicit override that
// fails verification is a config error in every mode; only auto-discovery
// failures fail open under auto.
func hasExplicitBase(configBaseRef, gateBase string) bool {
	if cb := strings.TrimSpace(configBaseRef); cb != "" && cb != "auto" {
		return true
	}
	return strings.TrimSpace(gateBase) != ""
}
