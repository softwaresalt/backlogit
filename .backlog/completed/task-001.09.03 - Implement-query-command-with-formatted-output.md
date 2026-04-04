---
id: TASK-001.09.03
title: Implement query command with formatted output
status: Done
assignee: []
created_date: '2026-03-30 01:45'
labels: []
dependencies: []
parent_task_id: TASK-001.09
priority: high
ordinal: 9300
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Create `internal/cli/query.go` with:

1. `backlogit query "SQL"` command — accepts SQL string as positional argument
2. Resolves workspace, opens DB, calls `db.ExecuteGatedQuery`
3. Formats results as aligned table with column headers: `| id | title | status |`
4. Handles empty results: `No results found.`
5. Handles gate rejection: displays descriptive error message

Create `internal/cli/query_test.go` with tests for formatted output and error cases.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 backlogit query 'SELECT id, title FROM items' outputs formatted table
- [ ] #2 backlogit query rejects non-SELECT statements with error message
- [ ] #3 Output includes column headers and aligned rows
- [ ] #4 Tests verify formatted output and error handling
<!-- AC:END -->


## Implementation Notes

Completed in commit a49b9dd. Gates passed: `go test ./...`, `go vet ./...`, `golangci-lint run`.