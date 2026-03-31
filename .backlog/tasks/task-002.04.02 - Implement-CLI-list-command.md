---
id: TASK-002.04.02
title: Implement CLI list command
status: To Do
assignee: []
created_date: '2026-03-30 06:59'
labels: []
dependencies:
  - TASK-002.01.02
parent_task_id: TASK-002.04
priority: high
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Implement `backlogit list [--type <type>] [--status <status>] [--assigned-to <user>] [--sprint <id>]` in `internal/cli/list.go`. Query SQLite index via `db.QueryItems` and format output as a table with columns: ID, Title, Status, Type, Priority. Support `--json` flag for JSON output. Default to table format.

Include slog instrumentation per review F4.

**Files:** `internal/cli/list.go` (new)
**Test files:** `internal/cli/list_test.go` (new)
**Patterns:** Follow `newSyncCommand` at `internal/cli/root.go`
**Verification:** `go test ./internal/cli/...` passes with list output formatting tests. JSON mode produces valid JSON array.
<!-- SECTION:DESCRIPTION:END -->

