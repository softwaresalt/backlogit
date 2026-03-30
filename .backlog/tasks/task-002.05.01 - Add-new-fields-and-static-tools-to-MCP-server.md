---
id: TASK-002.05.01
title: Add new fields and static tools to MCP server
status: To Do
assignee: []
created_date: '2026-03-30 07:01'
labels: []
dependencies:
  - TASK-002.01.05
parent_task_id: TASK-002.05
priority: high
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Add the new fields (`assigned_to`, `owner`, `labels`, `dependencies`, `references`, `commit`) to `backlogit_create_item` and `backlogit_update_item` tool schemas in `internal/mcp/tools.go`. Create new static MCP tools:
- `backlogit_list_items` — filter by type, status, assigned_to, sprint
- `backlogit_search_items` — FTS5 search with limit parameter
- `backlogit_move_item` — status change with file routing
- `backlogit_delete_item` — remove artifact by ID

Each tool follows the five-step handler pattern. Add contract tests validating input/output schemas.

**Files:** `internal/mcp/tools.go`
**Test files:** `tests/contract/tools_contract_test.go` (new or expand)
**Patterns:** Follow existing tool registration at `internal/mcp/tools.go`
**Verification:** Contract tests validate tool input/output schemas. Each new tool follows the five-step handler pattern.
<!-- SECTION:DESCRIPTION:END -->
