package mcp

import (
	"encoding/json"

	mcplib "github.com/mark3labs/mcp-go/mcp"
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
