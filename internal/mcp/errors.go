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
	return makeErrorResult("workspace_not_initialized", "No .backlogit directory found. Run backlogit init first.")
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
