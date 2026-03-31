---
id: TASK-001.08.01
title: Implement MCP Server struct and stdio lifecycle
status: Done
assignee: []
created_date: '2026-03-30 01:43'
labels: []
dependencies: []
parent_task_id: TASK-001.08
priority: high
ordinal: 8100
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Create `internal/mcp/server.go` with:

1. `Server` struct — holds `*sql.DB`, `*config.WorkspaceConfig`, workspace root path, `*events.EventWriter`, `*events.TelemetryWriter`. No global mutable state (per review P1-03).
2. `NewServer(ws *core.Workspace) *Server` — constructor accepting Workspace struct
3. `CreateMCPServer(s *Server) *server.MCPServer` — creates mcp-go server with `WithToolCapabilities(true)`, `WithResourceCapabilities(false, true)`, `WithRecovery()`. Registers all tools and resources via s.registerTools() and s.registerResources().
4. `RunStdio(s *Server) error` — creates MCP server, starts stdio transport, blocks.

Create `internal/mcp/errors.go` with helper functions using `json.Marshal` for safe escaping (per P2-07):
- `workspaceNotInitialized() *mcp.CallToolResult`
- `validationFailed(detail string) *mcp.CallToolResult`
- `internalError(detail string) *mcp.CallToolResult`
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Server struct holds *sql.DB, *WorkspaceConfig, and workspace path (no global state)
- [ ] #2 CreateServer() returns configured MCPServer with tool and resource capabilities
- [ ] #3 RunStdio() starts stdio transport and blocks until shutdown
- [ ] #4 Tests verify server creation and capability registration
<!-- AC:END -->


## Implementation Notes

Completed in commit a49b9dd. Gates passed: `go test ./...`, `go vet ./...`, `golangci-lint run`.