---
id: TASK-001.05.01
title: Implement SQLite connection management
status: To Do
assignee: []
created_date: '2026-03-30 01:41'
labels: []
dependencies: []
parent_task_id: TASK-001.05
priority: high
ordinal: 5100
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Create `internal/db/connection.go` with:

1. `Open(dbPath string) (*sql.DB, error)` — opens SQLite connection via `modernc.org/sqlite` (blank import `_ "modernc.org/sqlite"` per P2-03), sets pragmas: `journal_mode=WAL`, `foreign_keys=ON`, `busy_timeout=5000`
2. Returns wrapped `ErrQuery` on connection or pragma failures

Create `internal/db/connection_test.go` verifying WAL mode and pragma settings.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Open() returns *sql.DB in WAL mode with foreign keys enabled
- [ ] #2 Busy timeout set to 5000ms
- [ ] #3 Connection closes cleanly with defer db.Close()
- [ ] #4 Tests verify WAL mode and foreign keys via PRAGMA queries
<!-- AC:END -->
