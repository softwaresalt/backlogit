---
id: TASK-002.04.07
title: Implement CLI query and status commands
status: done
assignee: []
created_date: '2026-03-30 07:01'
labels: []
dependencies:
  - TASK-002.01.02
parent_task_id: TASK-002.04
priority: high
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Implement two commands in separate files:

**`backlogit query "<sql>"`** (`internal/cli/query.go`): Execute a read-only SQL query via `db.ExecuteGatedQuery` and display results as a formatted table. Reject non-SELECT statements with descriptive error.

**`backlogit status`** (`internal/cli/status.go`): Show workspace summary including artifact counts by type and status, last sync time, workspace path, and template count.

Include slog instrumentation per review F4.

**Files:** `internal/cli/query.go` (new), `internal/cli/status.go` (new)
**Test files:** `internal/cli/query_test.go` (new), `internal/cli/status_test.go` (new)
**Patterns:** Follow `db.ExecuteGatedQuery` in `internal/db/gate.go`
**Verification:** `query` rejects non-SELECT statements with descriptive error. `status` displays correct counts matching SQLite index.
<!-- SECTION:DESCRIPTION:END -->

