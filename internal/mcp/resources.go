package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	mcplib "github.com/mark3labs/mcp-go/mcp"
)

// RegisterResources adds workspace metadata resources to the MCP server.
func (s *Server) RegisterResources() {
	s.mcp.AddResource(
		mcplib.NewResource(
			"backlogit://config",
			"Workspace Configuration",
			mcplib.WithResourceDescription("Current workspace configuration"),
			mcplib.WithMIMEType("application/json"),
		),
		s.handleConfigResource,
	)
}

func (s *Server) handleConfigResource(ctx context.Context, req mcplib.ReadResourceRequest) ([]mcplib.ResourceContents, error) {
	ws, err := s.ensureWorkspace(ctx)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("workspace not initialized: no supported workspace directory found. Run backlogit init first: %w", err)
		}
		return nil, fmt.Errorf("open workspace: %w", err)
	}
	data, err := json.MarshalIndent(ws.Config, "", "  ")
	if err != nil {
		return nil, err
	}
	return []mcplib.ResourceContents{
		mcplib.TextResourceContents{
			URI:      req.Params.URI,
			MIMEType: "application/json",
			Text:     string(data),
		},
	}, nil
}
