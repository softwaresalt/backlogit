package mcp

import (
	"encoding/json"
	"errors"
	"fmt"

	mcplib "github.com/mark3labs/mcp-go/mcp"

	"github.com/softwaresalt/backlogit/internal/core"
	corerrors "github.com/softwaresalt/backlogit/internal/errors"
)

type errorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

type workspaceRootAmbiguousResponse struct {
	Error                string   `json:"error"`
	Message              string   `json:"message"`
	Roots                []string `json:"roots"`
	SupportedResolutions []string `json:"supported_resolutions"`
	Override             string   `json:"override"`
	Retryable            bool     `json:"retryable"`
}

type mutationPartialResponse struct {
	Error             string   `json:"error"`
	Message           string   `json:"message"`
	Classification    string   `json:"classification"`
	CompletedSteps    []string `json:"completed_steps"`
	FailedStep        string   `json:"failed_step"`
	CompensationState string   `json:"compensation_state"`
	Retryable         bool     `json:"retryable"`
	Recovery          string   `json:"recovery"`
}

func makeErrorResult(errType, message string) *mcplib.CallToolResult {
	resp := errorResponse{Error: errType, Message: message}
	data, err := json.Marshal(resp)
	if err != nil {
		// Fallback to manually formatted JSON to avoid recursive error handling.
		return mcplib.NewToolResultError(fmt.Sprintf(`{"error":%q,"message":%q}`, errType, message))
	}
	return mcplib.NewToolResultError(string(data))
}

// WorkspaceNotInitialized returns an MCP error for missing workspace.
func WorkspaceNotInitialized() *mcplib.CallToolResult {
	return makeErrorResult("workspace_not_initialized", "No supported workspace directory found. Run backlogit init first.")
}

// WorkspaceRootAmbiguous returns an MCP error when both workspace roots exist.
func WorkspaceRootAmbiguous(roots []string) *mcplib.CallToolResult {
	resp := workspaceRootAmbiguousResponse{
		Error:                "workspace_root_ambiguous",
		Message:              "Both supported workspace directories exist. Set BACKLOGIT_WORKSPACE_DIR to one supported name or remove one directory.",
		Roots:                append([]string(nil), roots...),
		SupportedResolutions: core.WorkspaceRootCandidates(),
		Override:             "BACKLOGIT_WORKSPACE_DIR",
		Retryable:            false,
	}
	data, err := json.Marshal(resp)
	if err != nil {
		return InternalError(fmt.Sprintf("marshal workspace root ambiguous response: %v", err))
	}
	return mcplib.NewToolResultError(string(data))
}

// ValidationFailed returns an MCP error for parameter validation failures.
func ValidationFailed(detail string) *mcplib.CallToolResult {
	return makeErrorResult("validation_failed", detail)
}

// NotFound returns an MCP error for missing resources.
func NotFound(detail string) *mcplib.CallToolResult {
	return makeErrorResult("not_found", detail)
}

// Conflict returns an MCP error for state conflicts.
func Conflict(detail string) *mcplib.CallToolResult {
	return makeErrorResult("conflict", detail)
}

// InternalError returns an MCP error for internal failures.
func InternalError(detail string) *mcplib.CallToolResult {
	return makeErrorResult("internal", detail)
}

// domainError routes domain sentinel errors to the correct MCP error category
// and falls back to InternalError for unknown failures.
//
// Error mapping table (sentinel → MCP error type):
//
//	Sentinel                  | MCP Type           | HTTP analogue
//	--------------------------|--------------------|---------------
//	ErrNotFound               | not_found          | 404
//	ErrShipmentNotFound       | not_found          | 404
//	ErrCheckpointNotFound     | not_found          | 404
//	ErrShipmentConflict       | conflict           | 409
//	ErrItemAlreadyAssigned    | conflict           | 409
//	ErrCannotReturnItem       | conflict           | 409
//	ErrChildrenNotTerminal    | conflict           | 409
//	ErrValidation             | validation_failed  | 422
//	ErrInvalidLinkType        | validation_failed  | 422
//	ErrTelemetrySourceMissing | validation_failed  | 422
//	ErrCheckpointInvalid      | validation_failed  | 422
//	ErrCheckpointCorrupt      | validation_failed  | 422
//	ErrTelemetryParseFailed   | internal           | 500
//	(all others)              | internal           | 500
//
// op is a short camelCase description of the operation, prepended to
// InternalError messages to aid diagnosis (e.g. "archive item").
func domainError(op string, err error) *mcplib.CallToolResult {
	var partialErr *corerrors.MutationPartialError
	if errors.As(err, &partialErr) {
		return mutationPartialError(op, partialErr)
	}
	switch {
	case errors.Is(err, corerrors.ErrShipmentNotFound), errors.Is(err, corerrors.ErrNotFound),
		errors.Is(err, corerrors.ErrCheckpointNotFound):
		return NotFound(err.Error())
	case errors.Is(err, corerrors.ErrShipmentConflict),
		errors.Is(err, corerrors.ErrItemAlreadyAssigned),
		errors.Is(err, corerrors.ErrCannotReturnItem),
		errors.Is(err, core.ErrTaskBusy),
		errors.Is(err, corerrors.ErrChildrenNotTerminal):
		return Conflict(err.Error())
	case errors.Is(err, corerrors.ErrAmbiguousWorkspaceRoot):
		var ambiguous *corerrors.AmbiguousWorkspaceRootError
		if errors.As(err, &ambiguous) {
			return WorkspaceRootAmbiguous(ambiguous.Roots)
		}
		return WorkspaceRootAmbiguous(core.WorkspaceRootCandidates())
	case errors.Is(err, corerrors.ErrValidation),
		errors.Is(err, corerrors.ErrInvalidLinkType),
		errors.Is(err, corerrors.ErrTelemetrySourceMissing),
		errors.Is(err, corerrors.ErrCheckpointInvalid),
		errors.Is(err, corerrors.ErrCheckpointCorrupt):
		return ValidationFailed(err.Error())
	default:
		return InternalError(fmt.Sprintf("%s: %v", op, err))
	}
}

func mutationPartialError(op string, err *corerrors.MutationPartialError) *mcplib.CallToolResult {
	resp := mutationPartialResponse{
		Error:             "mutation_partial",
		Message:           fmt.Sprintf("%s: %s", op, err.Error()),
		Classification:    err.Class,
		CompletedSteps:    append([]string(nil), err.Completed...),
		FailedStep:        err.FailedStep,
		CompensationState: err.CompensationState,
		Retryable:         err.Class == "not-applied",
		Recovery:          mutationPartialRecovery(err.Class),
	}
	data, marshalErr := json.Marshal(resp)
	if marshalErr != nil {
		return InternalError(fmt.Sprintf("marshal mutation partial response: %v", marshalErr))
	}
	return mcplib.NewToolResultError(string(data))
}

func mutationPartialRecovery(classification string) string {
	switch classification {
	case "not-applied":
		return "safe to retry the mutation; the envelope compensated earlier steps, and doctor with check_partial_mutations can confirm the clean state"
	case "indeterminate":
		return "do not blindly retry; inspect doctor with check_partial_mutations to reconcile state"
	case "double-fault":
		return "manual recovery required; inspect doctor with check_partial_mutations and review compensation failures"
	default:
		return "inspect doctor with check_partial_mutations before retrying"
	}
}

// blockingChildrenResult returns a structured error response when a parent
// artifact cannot move to a terminal status because non-terminal children exist.
func blockingChildrenResult(children []core.ChildStatus) *mcplib.CallToolResult {
	type childEntry struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	type resp struct {
		Error    string       `json:"error"`
		Message  string       `json:"message"`
		Children []childEntry `json:"children"`
	}
	entries := make([]childEntry, len(children))
	for i, c := range children {
		entries[i] = childEntry{ID: c.ID, Status: c.Status}
	}
	r := resp{
		Error:    "blocking_children",
		Message:  fmt.Sprintf("%d non-terminal child(ren) are blocking this status transition", len(children)),
		Children: entries,
	}
	data, err := json.Marshal(r)
	if err != nil {
		return InternalError(fmt.Sprintf("marshal blocking children response: %v", err))
	}
	return mcplib.NewToolResultError(string(data))
}

// checkpointDispositionErrorResponse is the structured MCP error shape for
// checkpoint disposition (abandon/quarantine) failures. It carries a stable
// code, the offending checkpoint filename, a class-derived retryable flag,
// and an actionable remediation hint so an agent caller does not need to
// parse the error message to decide what to do next.
type checkpointDispositionErrorResponse struct {
	Error       string `json:"error"`
	Message     string `json:"message"`
	Code        string `json:"code"`
	Filename    string `json:"filename"`
	Retryable   bool   `json:"retryable"`
	Outcome     string `json:"outcome,omitempty"`
	Remediation string `json:"remediation"`
}

// checkpointDispositionError maps a checkpoint disposition sentinel error
// (internal/errors checkpoint_errors.go) to a structured MCP error result.
// A *corerrors.MutationPartialError from the underlying MutationEnvelope (the
// rewrite/move/sidecar steps) is detected first and routed through the same
// mutationPartialError formatter domainError uses, so a partial-mutation
// failure surfaces its classification, completed steps, compensation state,
// and retryable flag instead of falling through to a generic internal error.
// Any other unrecognized error also falls back to InternalError.
func checkpointDispositionError(op, filename string, err error) *mcplib.CallToolResult {
	var partialErr *corerrors.MutationPartialError
	if errors.As(err, &partialErr) {
		return mutationPartialError(op, partialErr)
	}

	resp := checkpointDispositionErrorResponse{
		Error:    "checkpoint_disposition_failed",
		Message:  fmt.Sprintf("%s: %v", op, err),
		Filename: filename,
	}
	switch {
	case errors.Is(err, corerrors.ErrCheckpointUseQuarantine):
		resp.Code = "checkpoint_use_quarantine"
		resp.Retryable = false
		resp.Remediation = "this target is malformed; call backlogit_quarantine_checkpoint instead of backlogit_abandon_checkpoint"
	case errors.Is(err, corerrors.ErrCheckpointUseAbandon):
		resp.Code = "checkpoint_use_abandon"
		resp.Retryable = false
		resp.Remediation = "this target is valid; call backlogit_abandon_checkpoint instead of backlogit_quarantine_checkpoint"
	case errors.Is(err, corerrors.ErrCheckpointNotActive):
		resp.Code = "checkpoint_not_active"
		resp.Retryable = false
		resp.Remediation = "abandon requires an active checkpoint; this checkpoint has a different status (e.g. resolved) and is not eligible"
	case errors.Is(err, corerrors.ErrCheckpointTargetUnsafe):
		resp.Code = "checkpoint_target_unsafe"
		resp.Retryable = false
		resp.Remediation = "supply a bare checkpoint filename (basename only, no path separators, no traversal, no symlink)"
	case errors.Is(err, corerrors.ErrCheckpointReasonRequired):
		resp.Code = "checkpoint_reason_required"
		resp.Retryable = false
		resp.Remediation = "retry with a non-empty reason parameter"
	case errors.Is(err, corerrors.ErrCheckpointOperatorRequired):
		resp.Code = "checkpoint_operator_required"
		resp.Retryable = false
		resp.Remediation = "retry with a non-empty operator parameter; operator is never inferred on the MCP surface"
	case errors.Is(err, corerrors.ErrCheckpointDestinationOccupied):
		resp.Code = "checkpoint_destination_occupied"
		resp.Retryable = false
		resp.Remediation = "an existing quarantined file already occupies the destination; resolve the conflict manually before retrying"
	case errors.Is(err, corerrors.ErrCheckpointAuditIndeterminate):
		resp.Code = "checkpoint_audit_indeterminate"
		resp.Retryable = false
		resp.Outcome = "indeterminate"
		resp.Remediation = "do not blindly retry; inspect the checkpoint disposition audit log to reconcile state before retrying"
	case errors.Is(err, corerrors.ErrCheckpointAuditNotApplied):
		resp.Code = "checkpoint_audit_not_applied"
		resp.Retryable = true
		resp.Remediation = "the audit append definitely did not apply and nothing was moved or rewritten; safe to retry"
	case errors.Is(err, corerrors.ErrCheckpointNotFound):
		return NotFound(err.Error())
	default:
		return InternalError(fmt.Sprintf("%s: %v", op, err))
	}
	data, marshalErr := json.Marshal(resp)
	if marshalErr != nil {
		return InternalError(fmt.Sprintf("marshal checkpoint disposition error: %v", marshalErr))
	}
	return mcplib.NewToolResultError(string(data))
}
