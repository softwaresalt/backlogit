---
id: TASK-001.05.04
title: Implement read-only SQL query gate
status: Done
assignee: []
created_date: '2026-03-30 01:41'
labels: []
dependencies: []
parent_task_id: TASK-001.05
priority: high
ordinal: 5400
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Create `internal/db/gate.go` with:

1. `ValidateQuery(sqlStr string) GateResult` — checks SQL statement against compiled regex patterns for forbidden operations (INSERT, UPDATE, DELETE, DROP, ALTER, CREATE, REPLACE, ATTACH, non-whitelisted PRAGMA, SQL comments `--`, multi-statement). Returns `GateResult{Allowed, Reason}`.
2. `ExecuteGatedQuery(db *sql.DB, query string, params ...any) ([]map[string]interface{}, error)` — validates then executes, caps results at `MaxRows` (default 500). Returns rows as maps keyed by column name.
3. Constants: `MaxRows = 500`, `forbiddenPatterns` compiled regex slice

Create `internal/db/gate_test.go` with comprehensive table-driven tests for allowed and forbidden patterns.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 ValidateQuery allows SELECT statements
- [ ] #2 ValidateQuery rejects INSERT, UPDATE, DELETE, DROP, ALTER, ATTACH
- [ ] #3 ValidateQuery rejects SQL injection patterns (comments, multi-statement)
- [ ] #4 ExecuteGatedQuery caps results at MaxRows (default 500)
- [ ] #5 Tests cover allowed queries, each forbidden pattern, and row limit enforcement
<!-- AC:END -->
