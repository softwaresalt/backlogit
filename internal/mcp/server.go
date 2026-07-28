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

	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/core/templates"
	"github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/version"
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
	// CLICommandProvider, when set, supplies the CLI command catalog so the
	// metadata catalog returned over MCP reaches parity with the CLI metadata
	// path. It is injected by the cli package (which builds the cobra command
	// tree) to avoid an import cycle: the cli package imports internal/mcp, so
	// internal/mcp cannot import cli directly. When nil, the catalog omits CLI
	// command data (e.g. for a server constructed without CLI wiring).
	CLICommandProvider func() []core.CommandInfo
	// LatestVersionLookup, when set, overrides the latest-release lookup used by
	// backlogit_get_version. Tests use this seam to keep version checks hermetic.
	LatestVersionLookup func(context.Context) (string, error)
	// NoUpdateCheck disables outbound latest-release checks for version output.
	NoUpdateCheck bool
	mcp           *mcpserver.MCPServer
	toolNames     []string
	toolDefs      []mcplib.Tool
	workspaceMu   sync.Mutex
	// manifest is an in-memory snapshot of workspace file metadata used by
	// backlogit_merge_sync to compute incremental diffs without a full rehydrate.
	// Protected by manifestMu; lock ordering: workspaceMu must be held before
	// manifestMu if both are needed simultaneously.
	manifest        map[string]db.FileEntry
	manifestMu      sync.RWMutex
	manifestVersion uint64
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
		Events:     core.NewWorkspaceEventWriter(ws, logsDir),
		Telemetry:  events.NewTelemetryWriter(telemetryPath),
		HookEvents: events.NewHookEventWriter(backlogitDir),
	}
	s.mcp = mcpserver.NewMCPServer(
		"backlogit",
		version.Resolve(),
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

// ToolDefs returns a copy of the registered tool definitions. The returned
// slice is safe to read and serialize without locking.
func (s *Server) ToolDefs() []mcplib.Tool {
	result := make([]mcplib.Tool, len(s.toolDefs))
	copy(result, s.toolDefs)
	return result
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

	// Rebuild the shared event writer now that the workspace (and its
	// durable_writes config) is known. The server may have been constructed
	// with ws=nil (NewServerForRoot), which yields a durable-off writer; the
	// production append paths (move_item, append_comment) read s.Events only
	// after funneling through requireWorkspace→ensureWorkspace under
	// workspaceMu, so this single rebuild happens-before every reader.
	logsDir := filepath.Join(s.backlogitDir(), "logs")
	s.Events = core.NewWorkspaceEventWriter(ws, logsDir)

	// Populate the manifest baseline so the first backlogit_merge_sync call
	// can compute a real diff instead of treating every file as added.
	// manifestMu is acquired inside workspaceMu — the documented lock order.
	storageRoot := core.WorkspaceStorageRoot(ws.RootPath)
	s.manifestMu.Lock()
	if s.manifest == nil {
		if m, buildErr := db.BuildManifest(storageRoot); buildErr == nil {
			s.manifest = m
			s.manifestVersion++
		} else {
			logger.Warn("failed to build initial manifest", "error", buildErr)
		}
	}
	s.manifestMu.Unlock()

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
