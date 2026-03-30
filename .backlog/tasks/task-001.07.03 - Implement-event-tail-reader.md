---
id: TASK-001.07.03
title: Implement event tail reader
status: Done
assignee: []
created_date: '2026-03-30 01:42'
labels: []
dependencies: []
parent_task_id: TASK-001.07
priority: high
ordinal: 7300
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Create `internal/events/reader.go` with:

1. `TailEvents(ctx context.Context, path string, itemID string, limit int) ([]Event, error)` — reads `events.jsonl`, filters by item_id, returns the most recent `limit` events. Scans from the end of file for efficiency on large files.
2. `ReadAllEvents(ctx context.Context, path string) ([]Event, error)` — reads all events (used by tests and diagnostics)

Create `internal/events/reader_test.go` with tests for filtering, limit, empty results, and large file handling.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 TailEvents returns the last N events for a specific item_id
- [ ] #2 Returns empty slice when no events match the item_id
- [ ] #3 Correctly handles large files by scanning from the end
- [ ] #4 Tests verify filtering by item_id and limit enforcement
<!-- AC:END -->
