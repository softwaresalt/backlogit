package mcp

import (
	"errors"

	mcplib "github.com/mark3labs/mcp-go/mcp"

	corerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// minFormalGateKeyBytes mirrors config.minFormalGateKeyBytes for MCP-facing
// remediation text without importing internal/config into this leaf.
const minFormalGateKeyBytes = 32

// formalGateErrorResult maps a formal-gate-evidence refusal (106-F F1/U8) —
// ErrFormalGateRequired, ErrProofInvalid, or ErrProofUnverifiable — to a
// structured, machine-readable MCP error. It returns (nil, false) for any
// other error so callers fall back to the existing gate/domain error paths.
//
// Every refusal here is deliberately non-retryable: under formal-gate
// enforcement there is no unauthenticated fallback, so retrying without
// fixing the underlying cause (missing/invalid key, invalid report, or a
// verification failure) would just fail again identically.
func formalGateErrorResult(err error) (*mcplib.CallToolResult, bool) {
	switch {
	case errors.Is(err, corerrors.ErrGateKeyAbsent):
		return marshalGateError(map[string]any{
			"error":           "formal_gate_key_absent",
			"message":         err.Error(),
			"missing_env_var": "BACKLOGIT_GATE_EVIDENCE_KEY",
			"min_key_bytes":   minFormalGateKeyBytes,
			"retryable":       false,
			"remediation":     "Set BACKLOGIT_GATE_EVIDENCE_KEY to a base64- or hex-encoded key of at least 32 decoded bytes before attempting the completion again.",
		}), true
	case errors.Is(err, corerrors.ErrGateKeyInvalid):
		return marshalGateError(map[string]any{
			"error":           "formal_gate_key_invalid",
			"message":         err.Error(),
			"missing_env_var": "BACKLOGIT_GATE_EVIDENCE_KEY",
			"min_key_bytes":   minFormalGateKeyBytes,
			"retryable":       false,
			"remediation":     "BACKLOGIT_GATE_EVIDENCE_KEY must decode (base64 or hex) to at least 32 bytes. Fix the value before attempting the completion again.",
		}), true
	case errors.Is(err, corerrors.ErrFormalReportInvalid):
		return marshalGateError(map[string]any{
			"error":       "formal_gate_report_invalid",
			"message":     err.Error(),
			"retryable":   false,
			"remediation": "The gate report must be valid JSON with at least one reviewers[] entry carrying a non-blank persona and decision. Re-run the gate with a conforming report before attempting the completion again.",
		}), true
	case errors.Is(err, corerrors.ErrProofInvalid):
		return marshalGateError(map[string]any{
			"error":       "formal_gate_proof_invalid",
			"message":     err.Error(),
			"retryable":   false,
			"remediation": "The recorded formal-gate-evidence proof did not verify (tampered field, wrong key, replayed counter, a superseding later event, or — for a shipment — a manifest that changed after evidence was signed). Investigate the cause; a bare resubmission will not resolve a tampered, replayed, or stale proof.",
		}), true
	case errors.Is(err, corerrors.ErrProofUnverifiable):
		return marshalGateError(map[string]any{
			"error":       "formal_gate_proof_unverifiable",
			"message":     err.Error(),
			"retryable":   false,
			"remediation": "The formal-gate-evidence proof could not be evaluated (missing proof fields or a canonicalization failure). Verify BACKLOGIT_GATE_EVIDENCE_KEY is set correctly and the evidence was recorded under formal enforcement before attempting the completion again.",
		}), true
	case errors.Is(err, corerrors.ErrFormalGateRequired):
		// A recognized formal-gate refusal without one of the more specific
		// causes above (e.g. a counter-lock or manifest-digest computation
		// failure) still gets a distinct, actionable error_type rather than
		// falling through to a generic internal error.
		return marshalGateError(map[string]any{
			"error":       "formal_gate_required",
			"message":     err.Error(),
			"retryable":   false,
			"remediation": "Formal gate evidence is required but could not be produced or verified. See the message for the specific cause before attempting the completion again.",
		}), true
	default:
		return nil, false
	}
}
