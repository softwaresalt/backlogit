package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/backlogit/backlogit/internal/core"
	"github.com/backlogit/backlogit/internal/core/templates"
	"github.com/backlogit/backlogit/internal/events"
	"github.com/backlogit/backlogit/internal/version"
)

var logger = slog.With("package", "mcp")

// Server holds dependencies for MCP tool handlers.
type Server struct {
	RootPath    string
	Workspace   *core.Workspace
	Events      *events.EventWriter
	Telemetry   *events.TelemetryWriter
	HookEvents  *events.HookEventWriter
	templateSvc *templates.Service
	mcp         *mcpserver.MCPServer
	toolNames   []string
	toolDefs    []mcplib.Tool
	workspaceMu sync.Mutex
}

// NewServer creates an MCP server from a workspace.
func NewServer(ws *core.Workspace) *Server {
	return newServer(ws.RootPath, ws)
}

// NewServerForRoot creates an MCP server bound to a repository root even when
// the workspace has not been initialized yet.
func NewServerForRoot(rootPath string) *Server {
	return newServer(rootPath, nil)
}

func newServer(rootPath string, ws *core.Workspace) *Server {
	backlogitDir := filepath.Join(rootPath, ".backlogit")
	logsDir := filepath.Join(backlogitDir, "logs")
	telemetryPath := filepath.Join(backlogitDir, "telemetry.jsonl")
	s := &Server{
		RootPath:   rootPath,
		Workspace:  ws,
		Events:     events.NewEventWriter(logsDir),
		Telemetry:  events.NewTelemetryWriter(telemetryPath),
		HookEvents: events.NewHookEventWriter(backlogitDir),
	}
	s.mcp = mcpserver.NewMCPServer(
		"backlogit",
		version.Version,
		mcpserver.WithToolCapabilities(true),
		mcpserver.WithResourceCapabilities(false, true),
		mcpserver.WithRecovery(),
	)
	s.RegisterTools()
	s.RegisterResources()

	s.refreshTemplateService(context.Background())
	return s
}

// addTool registers a tool with the mcp server and records its name for ListTools.
func (s *Server) addTool(tool mcplib.Tool, handler mcpserver.ToolHandlerFunc) {
	s.mcp.AddTool(tool, handler)
	s.toolNames = append(s.toolNames, tool.Name)
	s.toolDefs = append(s.toolDefs, tool)
}

// RunStdio starts the MCP server on stdio transport.
func RunStdio(s *Server) error {
	logger.Info("starting backlogit MCP server on stdio")
	return mcpserver.ServeStdio(s.mcp)
}

func (s *Server) backlogitDir() string {
	return filepath.Join(s.RootPath, ".backlogit")
}

func (s *Server) refreshTemplateService(ctx context.Context) {
	if s.Workspace == nil {
		RegisterSectionAwareTools(s, nil)
		return
	}

	templatesDir := filepath.Join(s.backlogitDir(), "templates")
	svc, err := templates.NewService(ctx, templatesDir)
	if err != nil {
		logger.Warn("template service unavailable", "error", err)
		svc = nil
	}
	RegisterSectionAwareTools(s, svc)
}

func (s *Server) ensureWorkspace(ctx context.Context) (*core.Workspace, error) {
	s.workspaceMu.Lock()
	defer s.workspaceMu.Unlock()

	if s.Workspace != nil {
		return s.Workspace, nil
	}

	// dirExists is checked inside the lock so a concurrent creation of the
	// .backlogit directory is visible and a failed init can be retried.
	if !dirExists(s.backlogitDir()) {
		return nil, os.ErrNotExist
	}

	ws, err := core.NewWorkspace(ctx, s.RootPath)
	if err != nil {
		// Do NOT cache the failure — allow the next caller to retry.
		return nil, err
	}
	s.Workspace = ws
	s.refreshTemplateService(ctx)
	return ws, nil
}

func (s *Server) requireWorkspace(ctx context.Context) (*core.Workspace, *mcplib.CallToolResult) {
	ws, err := s.ensureWorkspace(ctx)
	if err == nil {
		return ws, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return nil, WorkspaceNotInitialized()
	}
	return nil, InternalError(fmt.Sprintf("open workspace: %v", err))
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
		return InternalError(fmt.Sprintf("marshal result: %v", err)), nil
	}
	return mcplib.NewToolResultText(string(data)), nil
}
