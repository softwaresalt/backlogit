---
id: TASK-002.01.02
title: Update DB schema and queries for new artifact fields
status: To Do
assignee: []
created_date: '2026-03-30 06:56'
labels: []
dependencies:
  - TASK-002.01.01
parent_task_id: TASK-002.01
priority: high
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Add columns to the `items` table: `assigned_to TEXT`, `owner TEXT`, `labels TEXT` (JSON array), `dependencies TEXT` (JSON array), `references TEXT` (JSON array), `commit TEXT`. Update `UpsertItem` to JSON-serialize slice fields. Update `scanArtifactRow` to deserialize them. Add `labels` and `dependencies` to FTS5 content. Update `QueryFilters` with `AssignedTo` and `Owner` filter options.

Since `index.db` is ephemeral, the schema change requires dropping and recreating via `EnsureSchema`. Document that `backlogit sync` rebuilds from scratch.

**Files:** `internal/db/schema.go`, `internal/db/queries.go`
**Test files:** `internal/db/queries_test.go` (new)
**Patterns:** Follow `UpsertItem`/`scanArtifactRow` at `internal/db/queries.go`
**Verification:** `go test ./internal/db/...` passes with upsert/scan round-trip tests for new fields. FTS5 search returns results matching label content.
<!-- SECTION:DESCRIPTION:END -->
