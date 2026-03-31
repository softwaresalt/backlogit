---
id: TASK-001.06
title: Rehydration Engine
status: Done
assignee: []
created_date: '2026-03-30 01:36'
updated_date: '2026-03-30 01:46'
labels:
  - epic
dependencies:
  - TASK-001.05
  - TASK-001.03
parent_task_id: TASK-001
priority: high
ordinal: 6000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Implement the auto-rehydration engine that walks the `.backlogit/` directory tree, parses all Markdown frontmatter, and rebuilds the SQLite index using concurrent goroutines.

Per review P2-01: Uses TRUNCATE + bulk INSERT in a single transaction for consistency.
Uses `errgroup.WithContext(ctx)` for fan-out parsing and fan-in batch insertion.

Split from original Unit 5 per review P2-11.
<!-- SECTION:DESCRIPTION:END -->
