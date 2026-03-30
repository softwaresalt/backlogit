---
id: TASK-002.04.03
title: Implement CLI get command
status: To Do
assignee: []
created_date: '2026-03-30 07:00'
labels: []
dependencies:
  - TASK-002.01.02
parent_task_id: TASK-002.04
priority: high
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Implement `backlogit get <id>` in `internal/cli/get.go`. Retrieve artifact via `db.GetItem`, then read the full Markdown file from disk. Display full artifact content (frontmatter + body). Support `--json` for frontmatter-only JSON output and `--section <name>` to extract a specific section using the section parser.

Include slog instrumentation per review F4.

**Files:** `internal/cli/get.go` (new)
**Test files:** `internal/cli/get_test.go` (new)
**Patterns:** Follow `newSyncCommand` at `internal/cli/root.go`
**Verification:** `go test ./internal/cli/...` passes with get output tests. `--section` flag returns only the specified section content.
<!-- SECTION:DESCRIPTION:END -->
