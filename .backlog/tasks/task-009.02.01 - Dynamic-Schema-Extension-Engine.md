---
id: TASK-009.02.01
title: Dynamic Schema Extension Engine
status: Done
assignee: []
created_date: '2026-03-31 06:05'
updated_date: '2026-04-01 05:18'
labels:
  - task
  - phase-1
  - foundation
  - security
dependencies:
  - TASK-002
parent_task_id: TASK-009.02
priority: high
ordinal: 4000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
**Unit 3 — Dynamic Schema Extension Engine**

Auto-generate SQLite schema columns from YAML field definitions in header-def.yaml, enabling agents to query custom fields without manual schema maintenance.

Key deliverables:
- Extend `EnsureSchema` in `internal/db/schema.go` to read custom field definitions from `HeaderDefConfig`
- Generate `ALTER TABLE items ADD COLUMN` for each custom field, mapping YAML types to SQLite column types
- **CRITICAL (P1 review finding F1)**: Validate column names against `^[a-z][a-z0-9_]{0,62}$` regex before DDL generation to prevent SQL injection through crafted field names in config.yaml
- Rebuild FTS5 index to include searchable custom field columns
- Update `internal/db/queries.go`: extend `UpsertItem` and `GetItem` to include custom field columns
- Update rehydration engine to populate custom field columns from frontmatter values
- Idempotent: re-running with same config produces no schema changes

Security constraint: Column names MUST be validated before use in DDL. This is the highest-priority review finding.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 ALTER TABLE items ADD COLUMN executes for each custom field defined in header-def.yaml
- [x] #2 Column names validated against ^[a-z][a-z0-9_]{0,62}$ regex before DDL generation
- [x] #3 Invalid column names produce descriptive validation error, not silent failure
- [x] #4 EnsureSchema idempotent: re-running with same config produces no schema changes
- [x] #5 FTS5 index rebuilt to include new custom field columns
- [x] #6 Rehydration populates custom field columns from frontmatter values
<!-- AC:END -->
