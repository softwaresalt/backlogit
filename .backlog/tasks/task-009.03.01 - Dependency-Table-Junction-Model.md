---
id: TASK-009.03.01
title: Dependency Table & Junction Model
status: To Do
assignee: []
created_date: '2026-03-31 06:06'
labels:
  - task
  - phase-1
  - foundation
dependencies:
  - TASK-009.01.01
parent_task_id: TASK-009.03
priority: high
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
**Unit 6 — Dependency Table & Junction Model**

Add a SQLite junction table for cross-level dependency tracking between artifacts at any hierarchy level.

Key deliverables:
- New table in `internal/db/schema.go`: `CREATE TABLE dependencies (from_id TEXT, to_id TEXT, dep_type TEXT, PRIMARY KEY(from_id, to_id))`
- Dependency types: `blocks`, `relates_to`, `parent_of`
- Extend `Artifact` model with `Dependencies []Dependency` struct (ID + Type)
- Extend YAML frontmatter format: `dependencies: [{id: 'T001', type: 'blocks'}]`
- Update `internal/models/frontmatter.go` to parse dependency entries
- Update rehydration engine to populate dependencies table from frontmatter
- Update `UpsertItem` to manage dependency rows
- Index: `CREATE INDEX idx_deps_to ON dependencies(to_id)` for reverse lookups

Review finding F3 (P2): Extend frontmatter format to include dependency type. Default to "blocks" when type is omitted for backward compatibility.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 CREATE TABLE dependencies with from_id, to_id, dep_type columns exists in schema
- [ ] #2 Dependency frontmatter format: dependencies: [{id: 'TASK-001', type: 'blocks'}]
- [ ] #3 Rehydration engine populates dependencies table from frontmatter
- [ ] #4 backlogit_query_sql can query dependency graph via JOIN on dependencies table
- [ ] #5 Default dep_type is 'blocks' when type omitted from frontmatter
<!-- AC:END -->
