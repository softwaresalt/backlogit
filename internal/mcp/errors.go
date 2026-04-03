package mcp

import (
	"encoding/json"
	"errors"
	"fmt"

	mcplib "github.com/mark3labs/mcp-go/mcp"

	corerrors "github.com/backlogit/backlogit/internal/errors"
)

type errorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func makeErrorResult(errType, message string) *mcplib.CallToolResult {
	resp := errorResponse{Error: errType, Message: message}
	data, _ := json.Marshal(resp)
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

// InternalError returns an MCP error for internal failures.
func InternalError(detail string) *mcplib.CallToolResult {
	return makeErrorResult("internal", detail)
}

// domainError routes not-found and validation errors to ValidationFailed and
// all other errors to InternalError. op is a short description prepended to
// InternalError messages to aid diagnosis.
func domainError(op string, err error) *mcplib.CallToolResult {
	if errors.Is(err, corerrors.ErrNotFound) || errors.Is(err, corerrors.ErrValidation) {
		return ValidationFailed(err.Error())
	}
	return InternalError(fmt.Sprintf("%s: %v", op, err))
}
