---
id: TASK-001.10
title: Legacy Migration Pipeline
status: Done
assignee: []
created_date: '2026-03-30 01:37'
updated_date: '2026-03-30 01:46'
labels:
  - epic
dependencies:
  - TASK-001.04
parent_task_id: TASK-001
priority: medium
ordinal: 10000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Implement the legacy backlog.md AST parser and transformation pipeline for migrating from monolithic backlog files to atomic `.backlogit/` artifacts.

Per review P1-01: `internal/parser/markdown.go` was moved to Sub-epic 3 (Models). This sub-epic retains only `legacy.go` (AST parser) and `migration.go` (transformation pipeline).

Parsing heuristics: section headings → status mapping, checklist markers → todo/done states, heading depth → parent-child hierarchy inference. Migration pipeline: extract → decompose → attribute → scaffold → archive (rename to `.bak`).
<!-- SECTION:DESCRIPTION:END -->
