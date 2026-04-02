---
id: TASK-009.05.02
title: Queue CLI & MCP Tools
status: Done
assignee: []
created_date: '2026-03-31 06:07'
updated_date: '2026-04-01 05:19'
labels:
  - task
  - phase-3
dependencies:
  - TASK-009.05.01
parent_task_id: TASK-009.05
priority: high
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
**Unit 11 — Queue CLI & MCP Tools**

Expose the work queue engine through CLI commands and MCP tools.

Key deliverables:
- New CLI command: `backlogit queue [--type TYPE] [--limit N] [--assignee NAME]`
- Tabular output showing: priority, ID, title, type, blocked-by (if any)
- New MCP tool: `backlogit_get_queue` — returns JSON array of queue items
- MCP tool parameters: `type` (filter), `status` (filter), `assignee` (filter), `limit` (max results)
- Include blocking reason in output for items that are not yet actionable
- Support `--verbose` flag for CLI to show full dependency chain
- Register in `internal/mcp/tools.go` and `internal/cli/root.go`
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 backlogit queue shows next actionable items in priority order
- [x] #2 backlogit queue --type task --limit 5 filters and limits output
- [x] #3 backlogit_get_queue MCP tool returns JSON array of queue items
- [x] #4 backlogit_get_queue accepts filter parameters: type, status, assignee, limit
- [x] #5 Queue output includes blocking reason for items not yet actionable
- [x] #6 CLI output uses tabwriter for aligned tabular display
<!-- AC:END -->
