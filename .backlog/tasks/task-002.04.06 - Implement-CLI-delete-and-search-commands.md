---
id: TASK-002.04.06
title: Implement CLI delete and search commands
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
Implement two commands in separate files:

**`backlogit delete <id> [--force]`** (`internal/cli/delete.go`): Remove the artifact's markdown file and delete from the SQLite index via `db.DeleteItem`. Without `--force`, prompt for confirmation on stderr.

**`backlogit search <query> [--limit N]`** (`internal/cli/search.go`): Full-text search via `db.SearchItems` with FTS5. Display results in table format with relevance ordering. Default limit 20.

Include slog instrumentation per review F4.

**Files:** `internal/cli/delete.go` (new), `internal/cli/search.go` (new)
**Test files:** `internal/cli/delete_test.go` (new), `internal/cli/search_test.go` (new)
**Patterns:** Follow `newSyncCommand` at `internal/cli/root.go`
**Verification:** Delete removes both markdown file and SQLite row. Search returns FTS5 results formatted as a table.
<!-- SECTION:DESCRIPTION:END -->
