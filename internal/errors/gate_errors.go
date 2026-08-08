package errors

import "errors"

// Gate broker sentinel errors (082-F, Pre-Task-Completion Gate Broker).
//
// These classify the non-pass outcomes of an `autoharness gate check` invocation
// on the task/subtask/shipment completion path. They live in the errors leaf so
// the CLI (internal/cli) and MCP (internal/mcp) adapters couple only to this
// package and route via errors.Is / errors.As, never importing internal/core.
var (
	// ErrGateBinaryNotFound indicates the configured autoharness binary could not
	// be resolved/executed. Under enabled:auto this is fail-open (proceed); under
	// enabled:true it is a fail-closed setup error.
	ErrGateBinaryNotFound = errors.New("backlogit: autoharness gate binary not found")

	// ErrGateSetup indicates the gate could not be enforced because the external
	// autoharness install is missing or version-incompatible while gates are
	// strictly enabled (enabled:true). Non-retryable; escalate to an operator.
	ErrGateSetup = errors.New("backlogit: gate setup error")

	// ErrGateConfig indicates an invalid gate configuration or a contract
	// violation: autoharness exit 2 (invalid args/gate config), a malformed or
	// missing repeated_failure contract under enabled:true, or an unverifiable
	// base/HEAD ref while enforcing. Non-retryable; escalate to an operator.
	ErrGateConfig = errors.New("backlogit: gate configuration error")

	// ErrGateTimeout indicates the gate run exceeded timeout_seconds and was
	// killed via context deadline. Distinct from a blocked (exit 1) outcome and
	// never derived from the platform-dependent killed-process exit code.
	// Retryable.
	ErrGateTimeout = errors.New("backlogit: gate timed out")

	// ErrGateInProgress indicates the per-task advisory lock could not be acquired
	// within the bounded wait because another operation holds it (a gate is in
	// progress for the same item). Retryable.
	ErrGateInProgress = errors.New("backlogit: gate in progress for item")

	// ErrGateKeyAbsent indicates BACKLOGIT_GATE_EVIDENCE_KEY is unset or empty
	// when a formal-gate-evidence key resolution was attempted (106-F F1). The
	// key is never sourced from config or a CLI flag.
	ErrGateKeyAbsent = errors.New("backlogit: formal gate evidence key not set (BACKLOGIT_GATE_EVIDENCE_KEY)")

	// ErrGateKeyInvalid indicates BACKLOGIT_GATE_EVIDENCE_KEY is set but fails to
	// decode as strict base64 or hex, or decodes to fewer than 32 bytes (106-F F1).
	ErrGateKeyInvalid = errors.New("backlogit: formal gate evidence key invalid (must be base64 or hex, >= 32 decoded bytes)")

	// ErrFormalGateRequired indicates formal-admission enforcement is required
	// (anchored by BACKLOGIT_FORMAL_GATE_REQUIRED, never lowerable by workspace
	// config) but the operation cannot proceed under that requirement — e.g. the
	// key could not be resolved. The operation refuses; there is no
	// unauthenticated fallback (106-F F1).
	ErrFormalGateRequired = errors.New("backlogit: formal gate evidence required but could not be satisfied")

	// ErrProofInvalid indicates a gate-evidence proof is definitively wrong: a
	// structural violation (unknown schema, wrong purpose/manifest_digest
	// combination), a malformed MAC encoding, or an HMAC mismatch (tampered
	// field or wrong key) (106-F F1/U3).
	ErrProofInvalid = errors.New("backlogit: gate evidence proof invalid")

	// ErrProofUnverifiable indicates a gate-evidence proof could not be
	// evaluated at all — e.g. the envelope could not be canonicalized to
	// compute the expected MAC — distinct from a proof that was evaluated and
	// found wrong (106-F F1/U3).
	ErrProofUnverifiable = errors.New("backlogit: gate evidence proof unverifiable")
)

// GateRepeatedFailure mirrors the autoharness `gate check --json`
// repeated_failure object. It is defined in the errors leaf (rather than
// imported from internal/core/gate) so GateBlockedError stays dependency-free
// and the errors package remains a leaf.
type GateRepeatedFailure struct {
	Count     int    `json:"count"`
	Threshold int    `json:"threshold"`
	Reached   bool   `json:"reached"`
	Action    string `json:"action"`
}

// GateBlockedError is returned when `autoharness gate check` refuses a completion
// transition (exit 1). It carries the full machine-readable report so CLI --json
// and the MCP structured error can convey retry-vs-stop guidance synchronously.
//
// The three block-family outcomes share this type, distinguished by Outcome:
//   - "blocked":   below repeated-failure threshold; item RETAINS OldStatus.
//   - "requeued":  threshold reached + action=block; item moved to queued.
//   - "escalated": threshold reached + action=escalate; item moved to blocked.
type GateBlockedError struct {
	ItemID       string
	OldStatus    string
	NewStatus    string // equals OldStatus on a plain block; queued/blocked on redirect
	Outcome      string // "blocked" | "requeued" | "escalated"
	StateChanged bool
	BaseRef      string
	HeadRef      string
	ExitCode     int
	ReportJSON   []byte
	Stderr       []byte
	Repeated     *GateRepeatedFailure
}

// Error implements the error interface.
func (e *GateBlockedError) Error() string {
	if e == nil {
		return "backlogit: gate blocked"
	}
	switch e.Outcome {
	case "requeued":
		return "gate blocked: " + e.ItemID + " moved to queued (repeated gate failure)"
	case "escalated":
		return "gate blocked: " + e.ItemID + " moved to blocked (escalated)"
	default:
		return "gate blocked: " + e.ItemID + " remains " + e.OldStatus
	}
}

// Is reports whether the target matches the ErrGateBlocked sentinel so callers
// can route with errors.Is in addition to errors.As.
func (e *GateBlockedError) Is(target error) bool {
	return target == ErrGateBlocked
}

// ErrGateBlocked is the sentinel matched by *GateBlockedError.Is.
var ErrGateBlocked = errors.New("backlogit: gate blocked")

// GateError carries a non-blocking gate refusal class (setup, config, timeout,
// in_progress) plus the machine-readable context needed to render a CLI exit
// code and an MCP structured error. It wraps the matching sentinel so callers
// route via errors.Is (class) and errors.As (context).
type GateError struct {
	Class        string // "setup" | "config" | "timeout" | "in_progress"
	ItemID       string
	Message      string
	ReportJSON   []byte
	Stderr       []byte
	RetryAfterMs int
	Err          error // wrapped sentinel (ErrGateSetup/ErrGateConfig/ErrGateTimeout/ErrGateInProgress)
}

// Error implements the error interface.
func (e *GateError) Error() string {
	if e == nil {
		return "backlogit: gate error"
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return "backlogit: gate error"
}

// Unwrap returns the wrapped sentinel so errors.Is resolves the class.
func (e *GateError) Unwrap() error { return e.Err }

// Retryable reports whether the caller may retry the transition unchanged.
func (e *GateError) Retryable() bool {
	return e.Class == "timeout" || e.Class == "in_progress"
}
