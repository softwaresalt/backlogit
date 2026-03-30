---
id: TASK-001.05.03
title: Implement parameterized query functions
status: To Do
assignee: []
created_date: '2026-03-30 01:41'
labels: []
dependencies: []
parent_task_id: TASK-001.05
priority: high
ordinal: 5300
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Create `internal/db/queries.go` with parameterized query functions:

1. `UpsertItem(ctx context.Context, db *sql.DB, artifact *models.Artifact) error` — INSERT OR REPLACE with all artifact fields including JSON-serialized custom_fields
2. `GetItem(ctx context.Context, db *sql.DB, id string) (*models.Artifact, error)` — SELECT by primary key, deserializes custom_fields JSON
3. `DeleteItem(ctx context.Context, db *sql.DB, id string) error` — DELETE by primary key
4. `QueryItems(ctx context.Context, db *sql.DB, filters QueryFilters) ([]*models.Artifact, error)` — parameterized SELECT with optional status, type, parent_id, sprint filters
5. `SearchItems(ctx context.Context, db *sql.DB, query string, limit int) ([]*models.Artifact, error)` — FTS5 MATCH query

All functions accept `context.Context` as first parameter (per P1-02). All errors wrapped with `ErrQuery`.

Create `internal/db/queries_test.go` with table-driven tests.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 UpsertItem inserts new items and updates existing ones
- [ ] #2 GetItem retrieves a single item by ID with all fields
- [ ] #3 QueryItems supports filtering by status, type, parent_id, sprint
- [ ] #4 SearchItems uses FTS5 for full-text search across title and description
- [ ] #5 All queries use parameterized statements (no string concatenation)
<!-- AC:END -->
