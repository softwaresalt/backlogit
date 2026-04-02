---
id: TASK-009.01.02
title: Migration Pipeline (Flat → Hierarchical)
status: Done
assignee: []
created_date: '2026-03-31 06:05'
updated_date: '2026-04-01 05:18'
labels:
  - task
  - phase-1
  - foundation
dependencies:
  - TASK-009.01.01
parent_task_id: TASK-009.01
priority: high
ordinal: 3000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
**Unit 2 — Migration Pipeline (flat → hierarchical)**

Build a migration pipeline that converts existing flat per-type directory layouts to the new hierarchical `.backlogit/queue/` structure.

Key deliverables:
- New CLI command: `backlogit migrate [--dry-run] [--rollback]`
- New `internal/core/migration.go`: `MigrateFlatToHierarchical(workspace, dryRun bool) (*MigrationReport, error)`
- State file `.backlogit/.migration-state` for crash recovery (review finding F5)
- Mapping logic: read existing artifacts, compute hierarchy based on parent_id relationships, assign new hierarchical IDs, move files atomically
- Dry-run mode: report planned moves without executing
- Rollback: reverse using state file
- Post-migration rehydration: rebuild SQLite index with new hierarchy columns
- Atomic file moves via temp-file-then-rename pattern

Review finding F5 (P2): Migration atomicity at scale — state file tracks each file move for crash recovery. On restart, resume from last successful move.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 backlogit migrate --dry-run reports all files that would move without modifying anything
- [x] #2 backlogit migrate converts flat layout to hierarchical queue/ layout
- [x] #3 State file (.backlogit/.migration-state) tracks progress for crash recovery
- [x] #4 Rollback via backlogit migrate --rollback restores pre-migration layout
- [x] #5 SQLite index.db rehydrated after migration with new hierarchy columns populated
- [x] #6 Migration preserves all frontmatter content and description body
<!-- AC:END -->
