package mcp

import (
	"context"
	"encoding/json"
	"runtime"

	mcplib "github.com/mark3labs/mcp-go/mcp"

	"github.com/softwaresalt/backlogit/internal/version"
)

// handleGetVersion handles the backlogit_get_version MCP tool.
// Returns version, commit, build_date, and go_version fields.
// The tool must function without a workspace (safe for pre-init calls).
func (s *Server) handleGetVersion(_ context.Context, _ mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	data := map[string]string{
		"version":    version.Resolve(),
		"commit":     version.Commit,
		"build_date": version.BuildDate,
		"go_version": runtime.Version(),
	}
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return makeErrorResult("internal", err.Error()), nil
	}
	return mcplib.NewToolResultText(string(b)), nil
}
