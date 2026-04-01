---
id: TASK-009.03
title: 'Epic C: Dependency Graph'
status: Done
assignee: []
created_date: '2026-03-31 06:04'
updated_date: '2026-04-01 05:20'
labels:
  - sub-epic
  - phase-1
  - phase-2
dependencies: []
references:
  - .backlog/exec-plans/2026-03-31-queue-features-v2-plan.md
parent_task_id: TASK-009
priority: high
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Implement cross-level dependency tracking between artifacts at any hierarchy level (feature, epic, task, sub-task, bugs, decisions). Add a junction table with dependency types (blocks, relates_to, parent_of), CLI commands for wiring dependencies, and MCP tools for querying the dependency graph.

Phase 1-2 (Foundation + Dependencies). Units 6-7 from the implementation plan.

Review finding F3 (P2): Junction table `dep_type` column has no source in current frontmatter format. Extend YAML frontmatter to include dependency type or default to "blocks".
<!-- SECTION:DESCRIPTION:END -->
