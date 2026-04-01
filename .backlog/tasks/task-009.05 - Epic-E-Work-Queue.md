---
id: TASK-009.05
title: 'Epic E: Work Queue'
status: In Progress
assignee: []
created_date: '2026-03-31 06:04'
labels:
  - sub-epic
  - phase-3
dependencies: []
references:
  - .backlog/plans/2026-03-31-queue-features-v2-plan.md
parent_task_id: TASK-009
priority: high
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Build the "what to work on next" work queue engine. Uses the dependency graph and artifact status to compute a prioritized streaming queue that respects parallel dependency constraints. Includes CLI commands (`backlogit queue`) and MCP tools (`backlogit_get_queue`) for agents to query the queue.

Phase 3 (Queue). Units 10-11 from the implementation plan.
<!-- SECTION:DESCRIPTION:END -->
