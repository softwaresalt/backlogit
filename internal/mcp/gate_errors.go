package mcp

import (
	"encoding/json"
	"errors"

	mcplib "github.com/mark3labs/mcp-go/mcp"

	"github.com/softwaresalt/backlogit/internal/core"
	corerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// gateErrorResult maps a gate typed error (GateBlockedError or GateError) to a
// structured MCP error result and reports whether it handled the error. It
// returns (nil, false) for non-gate errors so callers fall back to domainError.
//
// Every non-pass gate class gets a distinct, machine-actionable error_type and
// a retryable flag so an agent can decide synchronously whether to repair, move
// the item off the terminal target, or back off and retry — never collapsing to
// a generic "internal"/"conflict".
func gateErrorResult(err error, requestedStatus string) (*mcplib.CallToolResult, bool) {
	var blocked *corerrors.GateBlockedError
	if errors.As(err, &blocked) {
		return gateBlockedResult(blocked, requestedStatus), true
	}
	var ge *corerrors.GateError
	if errors.As(err, &ge) {
		return gateClassResult(ge), true
	}
	return nil, false
}

// gateBlockedResult renders the exit-1 refusal family (blocked/requeued/
// escalated). A plain block retains the current status and offers next actions;
// the redirect outcomes report state_changed:true and the new_status the broker
// moved the item to.
func gateBlockedResult(be *corerrors.GateBlockedError, requestedStatus string) *mcplib.CallToolResult {
	body := map[string]any{
		"error":            gateBlockedErrorType(be.Outcome),
		"message":          be.Error(),
		"item_id":          be.ItemID,
		"old_status":       be.OldStatus,
		"new_status":       be.NewStatus,
		"requested_status": requestedStatus,
		"outcome":          be.Outcome,
		"state_changed":    be.StateChanged,
		"retryable":        false,
	}
	if be.BaseRef != "" {
		body["base_ref"] = be.BaseRef
	}
	if be.HeadRef != "" {
		body["head_ref"] = be.HeadRef
	}
	if rf := be.Repeated; rf != nil {
		body["repeated_failure"] = map[string]any{
			"count":     rf.Count,
			"threshold": rf.Threshold,
			"reached":   rf.Reached,
			"action":    rf.Action,
		}
	}
	if len(be.ReportJSON) > 0 && json.Valid(be.ReportJSON) {
		body["gate_report"] = json.RawMessage(be.ReportJSON)
	}
	// A plain block leaves the item where it was; guide the caller to repair or
	// step the item back to a non-terminal status. Redirects already moved the
	// item, so no next-action menu is offered.
	if be.Outcome == "blocked" {
		body["allowed_next_actions"] = []string{"repair_and_retry", "move_to_non_terminal"}
	}
	return marshalGateError(body)
}

// gateClassResult renders the non-block gate classes (setup/config/timeout/
// in_progress) with retry guidance derived from the typed error.
func gateClassResult(ge *corerrors.GateError) *mcplib.CallToolResult {
	body := map[string]any{
		"error":     gateClassErrorType(ge.Class),
		"message":   ge.Error(),
		"item_id":   ge.ItemID,
		"retryable": ge.Retryable(),
	}
	if len(ge.ReportJSON) > 0 && json.Valid(ge.ReportJSON) {
		body["gate_report"] = json.RawMessage(ge.ReportJSON)
	}
	switch ge.Class {
	case "setup":
		body["remediation"] = "Install or repair the autoharness binary (min 1.4.7), or disable the gate for this workspace."
	case "config":
		body["remediation"] = "Fix the gate configuration or base/HEAD ref, then retry the completion."
	case "timeout", "in_progress":
		if ge.RetryAfterMs > 0 {
			body["retry_after_ms"] = ge.RetryAfterMs
		}
	}
	return marshalGateError(body)
}

// gateBlockedErrorType maps a block-family outcome to its stable error_type.
func gateBlockedErrorType(outcome string) string {
	switch outcome {
	case "requeued":
		return "gate_requeued"
	case "escalated":
		return "gate_escalated"
	default:
		return "gate_blocked"
	}
}

// gateClassErrorType maps a GateError class to its stable error_type.
func gateClassErrorType(class string) string {
	switch class {
	case "setup":
		return "gate_setup"
	case "config":
		return "gate_config"
	case "timeout":
		return "gate_timeout"
	case "in_progress":
		return "gate_in_progress"
	default:
		return "gate_error"
	}
}

// marshalGateError marshals a gate error body into an MCP error result, falling
// back to a minimal internal error if marshaling fails.
func marshalGateError(body map[string]any) *mcplib.CallToolResult {
	data, err := json.Marshal(body)
	if err != nil {
		errType, _ := body["error"].(string)
		msg, _ := body["message"].(string)
		return makeErrorResult(errType, msg)
	}
	return mcplib.NewToolResultError(string(data))
}

// gatePassResult augments a successful gated completion with the gate outcome
// contract so a machine caller receives gate_report_hash/head_sha and the
// authoritative post-transition status alongside the artifact. Non-gated
// completions (outcome == nil) are rendered by the caller via toolResultJSON.
func gatePassResult(artifact any, outcome *core.GateOutcome) (*mcplib.CallToolResult, error) {
	gate := map[string]any{
		"outcome":       outcome.Outcome,
		"old_status":    outcome.OldStatus,
		"new_status":    outcome.NewStatus,
		"state_changed": outcome.StateChanged,
		"forced":        outcome.Forced,
	}
	if outcome.BaseRef != "" {
		gate["base_ref"] = outcome.BaseRef
	}
	if outcome.HeadRef != "" {
		gate["head_ref"] = outcome.HeadRef
	}
	if outcome.HeadSHA != "" {
		gate["head_sha"] = outcome.HeadSHA
	}
	if outcome.GateReportHash != "" {
		gate["gate_report_hash"] = outcome.GateReportHash
	}
	if rf := outcome.RepeatedFailure; rf != nil {
		gate["repeated_failure"] = map[string]any{
			"count":     rf.Count,
			"threshold": rf.Threshold,
			"reached":   rf.Reached,
			"action":    rf.Action,
		}
	}
	// Marshal the artifact into a generic object so the gate envelope sits
	// alongside its fields without a struct-embedding dependency on models.
	raw, err := json.Marshal(artifact)
	if err != nil {
		return InternalError("marshal gated artifact: " + err.Error()), nil
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		// Artifact did not marshal to an object; return it plus a sibling gate
		// key is impossible, so fall back to the bare artifact result.
		return toolResultJSON(artifact)
	}
	obj["gate"] = gate
	return toolResultJSON(obj)
}
