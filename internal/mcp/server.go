package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/backlogit/backlogit/internal/core"
	"github.com/backlogit/backlogit/internal/core/templates"
	"github.com/backlogit/backlogit/internal/events"
)

var logger = slog.With("package", "mcp")

// Server holds dependencies for MCP tool handlers.
type Server struct {
	Workspace   *core.Workspace
	Events      *events.EventWriter
	Telemetry   *events.TelemetryWriter
	templateSvc *templates.Service
	mcp         *mcpserver.MCPServer
	toolNames   []string
}

// NewServer creates an MCP server from a workspace.
func NewServer(ws *core.Workspace) *Server {
	eventsPath := filepath.Join(ws.RootPath, "events.jsonl")
	telemetryPath := filepath.Join(ws.RootPath, "telemetry.jsonl")
	s := &Server{
		Workspace: ws,
		Events:    events.NewEventWriter(eventsPath),
		Telemetry: events.NewTelemetryWriter(telemetryPath),
	}
	s.mcp = mcpserver.NewMCPServer(
		"backlogit",
		"1.0.0",
		mcpserver.WithToolCapabilities(true),
		mcpserver.WithResourceCapabilities(false, true),
		mcpserver.WithRecovery(),
	)
	s.RegisterTools()
	s.RegisterResources()

	// Construct a live template service for section-aware operations.
	templatesDir := filepath.Join(ws.RootPath, ".backlogit", "templates")
	svc, err := templates.NewService(context.Background(), templatesDir)
	if err != nil {
		logger.Warn("template service unavailable", "error", err)
	}
	RegisterSectionAwareTools(s, svc)
	return s
}

// addTool registers a tool with the mcp server and records its name for ListTools.
func (s *Server) addTool(tool mcplib.Tool, handler mcpserver.ToolHandlerFunc) {
	s.mcp.AddTool(tool, handler)
	s.toolNames = append(s.toolNames, tool.Name)
}

// RunStdio starts the MCP server on stdio transport.
func RunStdio(s *Server) error {
	logger.Info("starting backlogit MCP server on stdio")
	return mcpserver.ServeStdio(s.mcp)
}

// dirExists reports whether path is an existing directory.
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// toolResultJSON wraps a map as a JSON CallToolResult.
func toolResultJSON(v any) (*mcplib.CallToolResult, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal result: %w", err)
	}
	return mcplib.NewToolResultText(string(data)), nil
}
