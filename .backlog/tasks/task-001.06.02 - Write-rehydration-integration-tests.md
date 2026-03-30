---
id: TASK-001.06.02
title: Write rehydration integration tests
status: Done
assignee: []
created_date: '2026-03-30 01:42'
labels: []
dependencies: []
parent_task_id: TASK-001.06
priority: high
ordinal: 6200
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Create `tests/integration/rehydration_test.go` with end-to-end integration tests:

1. Create a temp `.backlogit/` workspace with 20+ sample Markdown files across multiple directories (sprint-board/todo/, sprint-board/active/, epics/, sprints/)
2. Run full rehydration cycle
3. Verify all items are indexed correctly via SQL queries
4. Verify FTS5 search returns relevant results
5. Verify custom_fields JSON column is queryable via `json_extract(custom_fields, '$.priority')`
6. Delete index.db and re-run rehydration to verify identical results (ephemeral cache property)
7. Test with malformed files mixed in (should log warnings but complete successfully)
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 End-to-end test creates 20+ sample .md files across multiple directories
- [ ] #2 Rehydration indexes all files and SQL queries return correct results
- [ ] #3 Deleting index.db and re-running rehydration produces identical results
- [ ] #4 FTS5 search finds items by keyword in title and description
- [ ] #5 Test verifies custom_fields JSON column is queryable via json_extract
<!-- AC:END -->


## Implementation Notes

Completed in commit 83cebfc. Gates passed: `go test ./...`, `go vet ./...`, `golangci-lint run`.