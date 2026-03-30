---
id: TASK-001.05.02
title: Implement schema bootstrapping with FTS5
status: Done
assignee: []
created_date: '2026-03-30 01:41'
labels: []
dependencies: []
parent_task_id: TASK-001.05
priority: high
ordinal: 5200
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Create `internal/db/schema.go` with:

1. `EnsureSchema(db *sql.DB) error` — creates tables idempotently using `CREATE TABLE IF NOT EXISTS`:
   - `items` table: id TEXT PRIMARY KEY, title TEXT, status TEXT, type TEXT, parent_id TEXT, sprint TEXT, priority TEXT, description TEXT, custom_fields TEXT (JSON column per P1-08), created_at TEXT, updated_at TEXT, FOREIGN KEY(parent_id) REFERENCES items(id)
   - FTS5 virtual table: `items_fts` on title and description columns
   - Indexes on status, type, parent_id, sprint

Create `internal/db/schema_test.go` verifying schema creation, idempotency, and FTS5.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 EnsureSchema creates items table with all required columns including custom_fields JSON
- [ ] #2 FTS5 virtual table created for full-text search on title and description
- [ ] #3 Schema creation is idempotent (safe to call multiple times)
- [ ] #4 Tests verify table existence, column types, and FTS5 functionality
<!-- AC:END -->


## Implementation Notes

Completed in commit 83cebfc. Gates passed: `go test ./...`, `go vet ./...`, `golangci-lint run`.