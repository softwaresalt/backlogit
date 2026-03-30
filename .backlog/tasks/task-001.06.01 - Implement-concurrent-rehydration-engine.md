---
id: TASK-001.06.01
title: Implement concurrent rehydration engine
status: Done
assignee: []
created_date: '2026-03-30 01:42'
labels: []
dependencies: []
parent_task_id: TASK-001.06
priority: high
ordinal: 6100
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Create `internal/db/rehydration.go` with:

1. `Rehydrate(ctx context.Context, workspacePath string, db *sql.DB) (int, error)` — walks the `.backlogit/` directory tree using `filepath.WalkDir`, parses all `.md` files via `parser.ParseMarkdownFile`, rebuilds the SQLite index.
2. Fan-out/fan-in pattern: goroutines parse files concurrently via `errgroup.WithContext(ctx)`, collect results into a channel, then batch-insert within a single transaction (per P2-01: TRUNCATE existing data, then bulk INSERT).
3. Logs progress via `slog.Info` with file count and timing.
4. Returns count of indexed items. Wraps errors with `ErrRehydration`.

Create `internal/db/rehydration_test.go` with unit tests using temp workspace with sample .md files.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Rehydrate walks .backlogit/ tree and parses all .md files with frontmatter
- [ ] #2 Uses errgroup.WithContext for concurrent parsing with cancellation
- [ ] #3 Rebuilds index using TRUNCATE then bulk INSERT in single transaction (per P2-01)
- [ ] #4 Correctly indexes sprints as type=sprint items
- [ ] #5 Handles malformed files gracefully (logs warning, continues with remaining files)
<!-- AC:END -->
