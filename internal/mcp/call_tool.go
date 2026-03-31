package mcp

import (
	mcpserver "github.com/mark3labs/mcp-go/server"
)

// MCPServer returns the underlying mcp-go server instance.
// Intended for use by test helpers that need direct MCP client access.
func (s *Server) MCPServer() *mcpserver.MCPServer {
	return s.mcp
}
