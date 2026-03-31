package mcp

import (
	"context"
	"fmt"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/client"
)

// CallToolForTest invokes a registered MCP tool by name using an in-process client.
// It initializes a fresh client connection on each call to avoid state leakage between tests.
func (s *Server) CallToolForTest(ctx context.Context, toolName string, args map[string]any) (*mcplib.CallToolResult, error) {
	c, err := client.NewInProcessClient(s.mcp)
	if err != nil {
		return nil, fmt.Errorf("create in-process client: %w", err)
	}
	if err := c.Start(ctx); err != nil {
		return nil, fmt.Errorf("start in-process client: %w", err)
	}

	initReq := mcplib.InitializeRequest{}
	initReq.Params.ClientInfo = mcplib.Implementation{Name: "test", Version: "0.0.1"}
	initReq.Params.ProtocolVersion = mcplib.LATEST_PROTOCOL_VERSION
	if _, err := c.Initialize(ctx, initReq); err != nil {
		return nil, fmt.Errorf("initialize in-process client: %w", err)
	}

	req := mcplib.CallToolRequest{}
	req.Params.Name = toolName
	req.Params.Arguments = args
	return c.CallTool(ctx, req)
}
