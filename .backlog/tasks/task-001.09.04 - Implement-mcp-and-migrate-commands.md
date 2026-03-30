---
id: TASK-001.09.04
title: Implement mcp and migrate commands
status: Done
assignee: []
created_date: '2026-03-30 01:45'
labels: []
dependencies: []
parent_task_id: TASK-001.09
priority: high
ordinal: 9400
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Create `internal/cli/mcp.go` with:

1. `backlogit mcp` command — resolves workspace, creates Workspace, creates Server, calls `RunStdio`. Logs startup info via slog.
2. Subcommand `backlogit mcp init [environment]` — delegates to MCP environment registration (Sub-epic 11)

Create `internal/cli/migrate.go` with:

1. `backlogit migrate [path]` command — accepts optional path to legacy backlog.md (defaults to `./backlog.md`). Calls `parser.Migrate` with the legacy path, workspace config, and registry.
2. Outputs: `Migrated {count} items from {path}. Original archived as {path}.bak`

Per review P1-09: this wires the CLI to the Unit 10 migration pipeline.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 backlogit mcp starts MCP stdio server that accepts JSON-RPC requests
- [ ] #2 backlogit migrate converts legacy backlog.md to atomic .backlogit/ files
- [ ] #3 backlogit migrate archives original file as .bak
- [ ] #4 Tests verify MCP server startup and migrate pipeline execution
<!-- AC:END -->


## Implementation Notes

Completed in commit a49b9dd. Gates passed: `go test ./...`, `go vet ./...`, `golangci-lint run`.