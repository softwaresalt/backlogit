package mcp

import (
	"context"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/client"
)

// CallToolForTest invokes a registered MCP tool by name, returning the tool result.
// This method exposes the server's tool dispatch for testing purposes, using the
// mcp-go InProcessClient to call tools through the standard JSON-RPC protocol.
//
// Worker: Create a client.InProcessClient from s.mcp, initialize it, then call
// the named tool with the given arguments. Return the CallToolResult. Cache the
// client for reuse across multiple calls in the same test.
func (s *Server) CallToolForTest(ctx context.Context, toolName string, args map[string]any) (*mcplib.CallToolResult, error) {
	panic("not implemented: Worker: Create InProcessClient from s.mcp, initialize, dispatch tool call by name, return result")
}

// ensure client import is used
var _ = client.NewInProcessClient
