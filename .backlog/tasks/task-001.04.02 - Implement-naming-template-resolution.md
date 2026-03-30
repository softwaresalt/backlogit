---
id: TASK-001.04.02
title: Implement naming template resolution
status: To Do
assignee: []
created_date: '2026-03-30 01:40'
labels: []
dependencies: []
parent_task_id: TASK-001.04
priority: high
ordinal: 4200
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Create `internal/core/naming.go` with:

1. `ResolveName(cfg *config.ArtifactTypeConfig, title string, nextID int, maxSlugLen int) string` — resolves name format template `{prefix}{NNN}-{title_slug}` with zero-padded ID and slugified title
2. `Slugify(title string, maxLen int) string` — converts title to lowercase kebab-case slug, truncates to maxLen, strips trailing hyphens
3. `NextID(ctx context.Context, db *sql.DB, artifactType string) (int, error)` — queries SQLite for `SELECT MAX(...)` to determine next sequential ID. Uses `filepath.WalkDir` as fallback when DB is unavailable (per P2-06).

Create `internal/core/naming_test.go` with table-driven tests.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 ResolveName with task type produces T001-implement-jwt format
- [ ] #2 Slug truncation respects MaxSlugLength from config
- [ ] #3 Sequential ID assignment increments correctly from existing max
- [ ] #4 Tests cover various name formats, long titles, special characters
<!-- AC:END -->
