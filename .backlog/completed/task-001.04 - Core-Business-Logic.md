---
id: TASK-001.04
title: Core Business Logic
status: Done
assignee: []
created_date: '2026-03-30 01:36'
updated_date: '2026-03-30 01:46'
labels:
  - epic
dependencies:
  - TASK-001.03
parent_task_id: TASK-001
priority: high
ordinal: 4000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Implement artifact creation with hierarchy enforcement, naming template resolution, custom field validation, status-based file routing, and workspace orchestration. This is the domain logic that both CLI and MCP layers invoke.

Per review P1-02: All I/O functions accept `context.Context` as first parameter.
Per review P1-06: `Workspace` struct coordinates cross-store writes (Markdown → SQLite → JSONL).
Per review P1-10: `SafeResolve` rejects path traversal outside `.backlogit/`.
Per review P2-06: Uses `filepath.WalkDir` instead of `filepath.Glob("**")`.
<!-- SECTION:DESCRIPTION:END -->
