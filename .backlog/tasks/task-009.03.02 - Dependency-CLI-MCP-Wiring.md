---
id: TASK-009.03.02
title: Dependency CLI & MCP Wiring
status: Done
assignee: []
created_date: '2026-03-31 06:06'
updated_date: '2026-04-01 05:19'
labels:
  - task
  - phase-2
dependencies:
  - TASK-009.03.01
parent_task_id: TASK-009.03
priority: high
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
**Unit 7 — Dependency CLI & MCP Wiring**

Expose dependency management through CLI commands and MCP tools.

Key deliverables:
- New CLI command: `backlogit dep add <from> --blocks|--relates-to <to>`
- New CLI command: `backlogit dep list <id>` — show dependencies for an artifact
- New CLI command: `backlogit dep remove <from> <to>`
- New MCP tools: `backlogit_add_dependency`, `backlogit_remove_dependency`, `backlogit_get_dependencies`
- `backlogit_get_dependencies` returns: direct dependencies, reverse dependencies (who depends on me), and transitive closure for graph traversal
- Circular dependency detection: reject cycles during add operations
- Cascade cleanup: when an artifact is deleted/archived, remove its dependency rows
- Update `backlogit_get_item` to include dependency information in response
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 backlogit dep add T001 --blocks T002 creates dependency in frontmatter and DB
- [x] #2 backlogit dep list T001 shows all dependencies with types and directions
- [x] #3 backlogit_add_dependency MCP tool creates dependency and returns updated graph
- [x] #4 backlogit_get_dependencies MCP tool returns dependency graph for an artifact
- [x] #5 Circular dependency detection prevents cycles (A blocks B blocks A rejected)
- [x] #6 Removing an artifact cascades dependency cleanup
<!-- AC:END -->
