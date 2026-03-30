---
id: TASK-001.11.01
title: Implement environment detection and config injection
status: Done
assignee: []
created_date: '2026-03-30 01:45'
labels: []
dependencies: []
parent_task_id: TASK-001.11
priority: low
ordinal: 11100
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Create `internal/cli/mcp_init.go` with:

1. `mcpInitCmd` — subcommand of `backlogit mcp init [environment]`
2. Supported environments: `ghcp` (GitHub Copilot), `vscode` (VS Code), `cursor` (Cursor), `claude` (Claude Code)
3. For JSON-based environments (ghcp, vscode, cursor):
   - Locate config file at known path
   - Parse existing JSON (handle missing/empty/malformed files)
   - Inject `{"backlogit": {"command": "backlogit", "args": ["mcp"]}}` into the servers section
   - Write back safely without overwriting existing entries
4. For Claude Code: execute `claude mcp add backlogit -- backlogit mcp`
5. Handles missing config directories by creating them

Create `internal/cli/mcp_init_test.go` with tests for each environment, existing config preservation, and duplicate prevention.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 backlogit mcp init ghcp injects into .copilot/mcp-config.json
- [ ] #2 backlogit mcp init vscode injects into .vscode/mcp.json
- [ ] #3 backlogit mcp init cursor injects into .cursor/mcp.json
- [ ] #4 Injection preserves existing config entries (no data loss)
- [ ] #5 Repeated runs do not duplicate the backlogit entry
- [ ] #6 Creates config directories if they do not exist
<!-- AC:END -->
