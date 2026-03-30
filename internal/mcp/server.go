package mcp

import (
	"github.com/backlogit/backlogit/internal/core"
	"github.com/backlogit/backlogit/internal/events"
)

// Server holds dependencies for MCP tool handlers.
type Server struct {
	Workspace *core.Workspace
	Events    *events.EventWriter
	Telemetry *events.TelemetryWriter
}

// NewServer creates an MCP server from a workspace.
//
// Worker: Implement server construction with dependency injection.
func NewServer(ws *core.Workspace) *Server {
	panic("not implemented: Worker: Implement MCP server construction")
}

// RunStdio starts the MCP server on stdio transport.
//
// Worker: Implement MCP server lifecycle with tool and resource registration.
func RunStdio(s *Server) error {
	panic("not implemented: Worker: Implement MCP stdio server")
}
