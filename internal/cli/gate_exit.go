package cli

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/softwaresalt/backlogit/internal/core"
	corerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// Versioned gate exit codes (082-F, ST3.1). Verified NOT to collide with the
// doctor --target 0-4 contract:
//   - 6: gate blocked (exit-1 refusal; item retained or redirected).
//   - 7: gate configuration/setup error (missing/incompatible binary under
//     enabled:true, autoharness exit 2, malformed repeated_failure contract).
//   - 8: gate retryable (lock contention / gate in progress, or timeout).
const (
	ExitGateBlocked   = 6
	ExitGateConfig    = 7
	ExitGateRetryable = 8
)

// gateExitError maps a gate typed error to an *ExitError carrying the versioned
// gate exit code, or nil when err is not a gate error. Callers set
// cmd.SilenceErrors and return the mapped *ExitError so main resolves the code
// via ExitCodeFor instead of collapsing to the generic 1.
func gateExitError(err error) *ExitError {
	var blocked *corerrors.GateBlockedError
	if errors.As(err, &blocked) {
		return &ExitError{Code: ExitGateBlocked, Msg: blocked.Error()}
	}
	var ge *corerrors.GateError
	if errors.As(err, &ge) {
		if ge.Retryable() {
			return &ExitError{Code: ExitGateRetryable, Msg: ge.Error()}
		}
		return &ExitError{Code: ExitGateConfig, Msg: ge.Error()}
	}
	return nil
}

// gateJSONPayload is the machine-readable outcome contract emitted by CLI
// move/update --json for gated transitions. It carries enough for an agent to
// decide synchronously whether the item moved and whether to retry or stop.
type gateJSONPayload struct {
	ID              string          `json:"id"`
	Outcome         string          `json:"outcome"` // passed|blocked|requeued|escalated|error
	OldStatus       string          `json:"old_status"`
	NewStatus       string          `json:"new_status"`
	StateChanged    bool            `json:"state_changed"`
	BaseRef         string          `json:"base_ref,omitempty"`
	HeadRef         string          `json:"head_ref,omitempty"`
	HeadSHA         string          `json:"head_sha,omitempty"`
	GateReportHash  string          `json:"gate_report_hash,omitempty"`
	Forced          bool            `json:"forced,omitempty"`
	RepeatedFailure *repeatedFailJS `json:"repeated_failure,omitempty"`
	GateReport      json.RawMessage `json:"gate_report,omitempty"`
	Error           string          `json:"error,omitempty"`
	Retryable       bool            `json:"retryable,omitempty"`
	AllowedNext     []string        `json:"allowed_next_actions,omitempty"`
}

type repeatedFailJS struct {
	Count     int    `json:"count"`
	Threshold int    `json:"threshold"`
	Reached   bool   `json:"reached"`
	Action    string `json:"action"`
}

// renderGatePassJSON marshals the machine payload for a passing gated completion.
func renderGatePassJSON(id string, outcome *core.GateOutcome) (string, error) {
	p := gateJSONPayload{
		ID:             id,
		Outcome:        outcome.Outcome,
		OldStatus:      outcome.OldStatus,
		NewStatus:      outcome.NewStatus,
		StateChanged:   outcome.StateChanged,
		BaseRef:        outcome.BaseRef,
		HeadRef:        outcome.HeadRef,
		HeadSHA:        outcome.HeadSHA,
		GateReportHash: outcome.GateReportHash,
		Forced:         outcome.Forced,
	}
	if rf := outcome.RepeatedFailure; rf != nil {
		p.RepeatedFailure = &repeatedFailJS{Count: rf.Count, Threshold: rf.Threshold, Reached: rf.Reached, Action: rf.Action}
	}
	data, err := json.Marshal(p)
	if err != nil {
		return "", fmt.Errorf("marshal gate outcome: %w", err)
	}
	return string(data), nil
}

// renderGateBlockedJSON marshals the machine payload for a block/redirect refusal
// from the typed *GateBlockedError.
func renderGateBlockedJSON(id string, be *corerrors.GateBlockedError) (string, error) {
	p := gateJSONPayload{
		ID:           id,
		Outcome:      be.Outcome,
		OldStatus:    be.OldStatus,
		NewStatus:    be.NewStatus,
		StateChanged: be.StateChanged,
		BaseRef:      be.BaseRef,
		HeadRef:      be.HeadRef,
		Error:        be.Error(),
	}
	// Parity with the MCP surface (gateBlockedResult): the next-action menu is
	// offered only for a plain block, where the item retained its terminal-bound
	// status. For requeued/escalated the broker has already redirected the item to
	// a non-terminal status, so move_to_non_terminal is non-actionable.
	if be.Outcome == "blocked" {
		p.AllowedNext = []string{"repair_and_retry", "move_to_non_terminal"}
	}
	if len(be.ReportJSON) > 0 && json.Valid(be.ReportJSON) {
		p.GateReport = json.RawMessage(be.ReportJSON)
	}
	if rf := be.Repeated; rf != nil {
		p.RepeatedFailure = &repeatedFailJS{Count: rf.Count, Threshold: rf.Threshold, Reached: rf.Reached, Action: rf.Action}
	}
	data, err := json.Marshal(p)
	if err != nil {
		return "", fmt.Errorf("marshal gate blocked outcome: %w", err)
	}
	return string(data), nil
}

// gateHumanMessage renders the human-facing one-line summary for a gated outcome,
// reporting the ACTUAL retained/post-transition status (never a hard-coded
// literal) since a block can occur from active OR review and redirects change it.
func gateHumanMessage(id string, be *corerrors.GateBlockedError) string {
	switch be.Outcome {
	case "requeued":
		return fmt.Sprintf("%s: gate blocked — task moved to queued (repeated gate failure)", id)
	case "escalated":
		return fmt.Sprintf("%s: gate blocked — task moved to blocked (escalated)", id)
	default:
		return fmt.Sprintf("%s: gate blocked — task remains %s", id, be.OldStatus)
	}
}
