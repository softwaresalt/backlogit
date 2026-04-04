# Memory Checkpoint: Feature 001 — Backlogit Core Implementation

## Session Metadata
- **Date**: 2026-03-29
- **Feature**: TASK-001
- **Branch**: 001-backlogit-core-implementation
- **Agent**: Build Orchestrator (batch mode)

## Tasks Completed

| Task ID | Description | Commit |
|---------|-------------|--------|
| TASK-001.02.01–03 | Config schema, loader, defaults | 83cebfc |
| TASK-001.03.01–04 | Artifact model, frontmatter, sprint, markdown parser | 83cebfc |
| TASK-001.04.01–06 | SafeResolve, naming, fields, routing, artifacts, workspace | 83cebfc |
| TASK-001.05.02–03 | Schema bootstrapping, query functions, ExecuteGatedQuery | 90b24e6 |
| TASK-001.06.01 | Rehydration engine (sequential walk) | 90b24e6 |
| TASK-001.07.01–04 | Event stream, telemetry, reader, memory | 90b24e6 |
| TASK-001.08.01–06 | MCP server, 9 tools, config resource | a49b9dd |
| TASK-001.09.01–04 | CLI root, init, sync, mcp commands | a49b9dd |
| TASK-001.10.01 | Legacy backlog.md parser | 90b24e6 |
| Review fixes | SafeResolve P1, SaveMemory race P2, MoveArtifactFile P2 | c392028 |

## Files Modified
- cmd/backlogit/main.go
- internal/cli/root.go
- internal/config/defaults.go, loader.go, schema.go
- internal/core/artifacts.go, fields.go, naming.go, routing.go, workspace.go
- internal/db/queries.go, rehydration.go, schema.go
- internal/events/memory.go, reader.go, stream.go, telemetry.go
- internal/mcp/resources.go, server.go, tools.go
- internal/models/artifact.go, frontmatter.go, sprint.go
- internal/parser/legacy.go, markdown.go

## Key Decisions

1. **Sequential rehydration** — db/rehydration.go walks workspace files sequentially (not errgroup concurrent) for simplicity.
2. **memoriesMu mutex** — added to SaveMemory after review gate P2 finding.
3. **SafeResolve absolute path fix** — workspaceRoot → filepath.Abs before comparison (P1 security fix).
4. **Import cycle prevention** — db/rehydration.go uses models.ParseFrontmatter directly (not parser.ParseMarkdownFile) to break db → parser → db cycle.
5. **MCP tools** — 9 tools registered with parameter validation + workspace nil guard.

## Errors Resolved During Session
1. gofmt formatting needed on all agent-generated files
2. Missing `strings` import in routing.go after MoveArtifactFile refactor
3. Legacy parser (parser/legacy.go) was missed in first delegation batch

## Final Gates
- `go build ./...` ✅
- `go vet ./...` ✅
- `go test ./internal/...` ✅ (all packages GREEN)
- gofmt clean ✅
- Review: P1 + P2 findings fixed ✅

## Next Context
- TASK-001.11.01 (MCP env registration) not yet implemented
- TASK-001.10.02 (migration pipeline Migrate()) — stub exists, not tested
- golangci-lint not available on this machine — install before next session
- DB query functions handle edge cases (ErrNotFound from internal/errors)
- mcp-go v0.27.0 API confirmed working: mcp.NewServer, server.NewStdioServer, .AddTool
