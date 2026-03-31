---
id: TASK-009.06.01
title: Harness Status Attribute
status: To Do
assignee: []
created_date: '2026-03-31 06:07'
labels:
  - task
  - phase-2
dependencies:
  - TASK-009.02.02
parent_task_id: TASK-009.06
priority: medium
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
**Unit 12 — Harness Status Attribute**

Add a new "harness" status attribute to feature-level WIT templates that gates progression through the build-feature workflow.

Key deliverables:
- Add `harness_status` field to feature-level WIT template in `defaults.go` or `header-def.yaml`
- Enum values: `pending`, `scaffolded`, `passing`, `failing`
- Field added to `HeaderDefConfig` system defaults for feature types
- Update dynamic schema (Unit 3) to include `harness_status` column
- Queryable via `backlogit_query_sql`: `SELECT id, title, harness_status FROM items WHERE type='feature'`
- CLI: `backlogit update <id> --harness-status scaffolded`
- MCP: `backlogit_update_item` accepts `harness_status` parameter
- Used by build-feature skill to check readiness before implementation loops

Review finding F8 (P3): Could merge into Unit 4 (template self-description) since both modify WIT definitions. Kept separate for cleaner scope and independent testing.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Feature-level WIT template includes harness_status field with enum: pending, scaffolded, passing, failing
- [ ] #2 harness_status queryable via backlogit_query_sql
- [ ] #3 build-feature workflow can check harness_status to gate progression
- [ ] #4 Default harness_status is 'pending' for new feature artifacts
- [ ] #5 backlogit update <id> --harness-status scaffolded updates the field
<!-- AC:END -->
