---
id: TASK-002.01.04
title: Update rehydration engine for new fields
status: Done
assignee: []
created_date: '2026-03-30 06:57'
labels: []
dependencies:
  - TASK-002.01.02
  - TASK-002.01.03
parent_task_id: TASK-002.01
priority: high
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Ensure the rehydration engine passes new fields through from parsed frontmatter to `UpsertItem`. Since rehydration uses `models.ParseFrontmatter` → `models.ArtifactFromFrontmatter` → `db.UpsertItem`, the changes from prior tasks should flow through, but integration testing is required.

**Files:** `internal/db/rehydration.go`
**Test files:** `internal/db/rehydration_test.go` (new or expand existing)
**Patterns:** Follow `Rehydrate` function in `internal/db/rehydration.go`
**Verification:** Integration test: write a Markdown file with all new fields → rehydrate → query SQLite → verify all fields round-trip correctly.
<!-- SECTION:DESCRIPTION:END -->

