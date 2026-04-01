---
id: TASK-009.05.01
title: Work Queue Engine
status: To Do
assignee: []
created_date: '2026-03-31 06:06'
labels:
  - task
  - phase-3
dependencies:
  - TASK-009.03.01
  - TASK-009.03.02
parent_task_id: TASK-009.05
priority: high
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
**Unit 10 — Work Queue Engine**

Build the "what to work on next" engine that uses the dependency graph and artifact status to compute a prioritized streaming queue.

Key deliverables:
- New `internal/core/queue.go`: `ComputeQueue(db *sql.DB, opts ...QueueOption) ([]QueueItem, error)`
- `QueueItem` struct: ID, Title, Priority, Type, BlockedBy []string, ReadyAt time.Time
- Topological sort of dependency graph to determine execution order
- Parallel dependency awareness: surface all items whose dependencies are satisfied, not just one
- Priority weighting within each dependency tier
- Filter options: by type, by status, by assignee, limit
- Performance target: <100ms for 500-item workspace
- Uses dependency table (Unit 6) and hierarchy (Unit 1) for computation

The queue engine is the core algorithm; CLI and MCP exposure is in Unit 11.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 ComputeQueue returns ordered list of actionable items respecting dependency constraints
- [ ] #2 Blocked items (unresolved dependencies) excluded from queue output
- [ ] #3 Parallel-ready items (independent siblings) surfaced together
- [ ] #4 Priority weighting: high > medium > low within same dependency tier
- [ ] #5 Queue computation completes in <100ms for 500-item workspace
- [ ] #6 Queue considers both direct and transitive blocking dependencies
<!-- AC:END -->
