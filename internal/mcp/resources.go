package mcp

import (
	"context"
	"encoding/json"

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

func (s *Server) handleConfigResource(_ context.Context, req mcplib.ReadResourceRequest) ([]mcplib.ResourceContents, error) {
	data, err := json.MarshalIndent(s.Workspace.Config, "", "  ")
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
