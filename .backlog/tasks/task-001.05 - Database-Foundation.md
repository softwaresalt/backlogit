---
id: TASK-001.05
title: Database Foundation
status: To Do
assignee: []
created_date: '2026-03-30 01:36'
updated_date: '2026-03-30 01:46'
labels:
  - epic
dependencies:
  - TASK-001.04
parent_task_id: TASK-001
priority: high
ordinal: 5000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Implement the SQLite ephemeral cache with WAL mode, schema bootstrapping (items table with FTS5 and custom_fields JSON column), parameterized queries, and the read-only SQL query gate.

Per review P1-08: `items` table includes a `custom_fields` JSON column for queryable custom field storage via `json_extract()`.
Per review P2-03: Uses blank import `_ "modernc.org/sqlite"` for driver registration.

Split from original Unit 5 per review P2-11. Rehydration is a separate sub-epic.
<!-- SECTION:DESCRIPTION:END -->
