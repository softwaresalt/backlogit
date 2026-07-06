// Package gate is the autoharness integration boundary for the pre-task-completion
// gate broker (082-F). It owns the command-runner exec seam, base-ref resolver,
// version/contract probe, --json report parsing, and the pure value types and
// decision mapper. It depends on NO internal/core internals — a one-way
// core -> gate edge that prevents an import cycle. Lock ownership, durable state
// writes, evidence persistence, shipment aggregation, and the doctor check all
// live in package core, which constructs adapters over its own helpers and drives
// this boundary.
package gate

// EnabledMode is the three-valued enablement of the gate broker.
type EnabledMode string

const (
	// EnabledAuto enforces gates when autoharness is resolvable and fails open
	// (proceed) when it is not. Default.
	EnabledAuto EnabledMode = "auto"
	// EnabledTrue strictly enforces gates and fails closed when autoharness is
	// unresolvable or incompatible.
	EnabledTrue EnabledMode = "true"
	// EnabledFalse disables the gate broker entirely (kill switch).
	EnabledFalse EnabledMode = "false"
)

// MinAutoharnessVersion is the code-constant version floor. 1.4.7 is the release
// where `gate check --json` emits repeated_failure {count,threshold,reached,action}
// and supports the --no-count advisory flag. It is intentionally NOT an
// operator-tunable config field.
const MinAutoharnessVersion = "1.4.7"

// HeadRef is the fixed, non-configurable head ref passed to autoharness. Pinning
// it to HEAD prevents a configurable head from selecting an empty-diff ref that
// silently passes the gate (attempt-2 Security P1).
const HeadRef = "HEAD"

// RepeatedFailure mirrors the autoharness `gate check --json` repeated_failure
// object that drives backlogit's requeue/escalate decision.
type RepeatedFailure struct {
	Count     int    `json:"count"`
	Threshold int    `json:"threshold"`
	Reached   bool   `json:"reached"`
	Action    string `json:"action"` // "block" | "escalate"
}

// GateReport is the subset of the autoharness --json correction report that the
// broker consumes.
type GateReport struct {
	RepeatedFailure *RepeatedFailure `json:"repeated_failure"`
}

// GateResult holds ONLY process-outcome fields. A failure-to-run is a returned
// error, never a struct field.
type GateResult struct {
	ExitCode int
	Stdout   []byte
	Stderr   []byte
}

// DecisionKind enumerates the broker's typed decisions.
type DecisionKind int

const (
	// DecisionProceed completes the transition to the requested terminal status.
	DecisionProceed DecisionKind = iota
	// DecisionRedirectQueued moves the item to queued (repeated failure, action=block).
	DecisionRedirectQueued
	// DecisionRedirectBlocked moves the item to blocked (repeated failure, action=escalate).
	DecisionRedirectBlocked
	// DecisionBlock refuses the transition below threshold; item retains its prior status.
	DecisionBlock
	// DecisionError refuses the transition due to a setup/config/timeout class error.
	DecisionError
)

// String renders a DecisionKind for logs and tests.
func (k DecisionKind) String() string {
	switch k {
	case DecisionProceed:
		return "proceed"
	case DecisionRedirectQueued:
		return "redirect_queued"
	case DecisionRedirectBlocked:
		return "redirect_blocked"
	case DecisionBlock:
		return "block"
	case DecisionError:
		return "error"
	default:
		return "unknown"
	}
}

// ErrorClass classifies a DecisionError so package core builds the matching typed
// error (setup/config -> exit 7; timeout -> exit 8).
type ErrorClass string

const (
	// ErrorClassSetup is a missing/incompatible autoharness binary under enabled:true.
	ErrorClassSetup ErrorClass = "setup"
	// ErrorClassConfig is autoharness exit 2 or a malformed/missing repeated_failure
	// contract under enabled:true.
	ErrorClassConfig ErrorClass = "config"
	// ErrorClassTimeout is a context-deadline kill of the gate run.
	ErrorClassTimeout ErrorClass = "timeout"
)

// GateDecision is the pure output of the exit-class mapper. It carries the data
// package core needs to apply exactly one durable write and, on a refusal, build
// the matching internal/errors typed error. The retained status on a plain block
// is NOT decided here: the pure mapper has no view of the artifact's current
// status, so retained/new_status is resolved by the core reread old_status.
type GateDecision struct {
	Kind            DecisionKind
	ErrorClass      ErrorClass // set when Kind == DecisionError
	ExitCode        int
	ReportJSON      []byte
	Stderr          []byte
	RepeatedFailure *RepeatedFailure
}
