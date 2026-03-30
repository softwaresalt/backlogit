---
id: TASK-001.08.03
title: Implement item read and query tools
status: To Do
assignee: []
created_date: '2026-03-30 01:43'
labels: []
dependencies: []
parent_task_id: TASK-001.08
priority: high
ordinal: 8300
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Add to `internal/mcp/tools.go`:

1. `backlogit_get_item` tool — parameter: item_id (required). Handler: reads Markdown file via `parser.ParseMarkdownFile`, returns JSON with frontmatter fields and body.
2. `backlogit_get_item_history` tool — parameters: item_id (required), limit (default 5). Handler: calls `events.TailEvents` to read recent events from events.jsonl.
3. `backlogit_query_sql` tool — parameter: sql (required). Handler: calls `db.ExecuteGatedQuery` with the read-only gate. Returns JSON array of result rows capped at MaxRows.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 backlogit_get_item returns Markdown content and frontmatter for a valid item_id
- [ ] #2 backlogit_get_item_history returns last N events from events.jsonl for an item_id
- [ ] #3 backlogit_query_sql enforces read-only gate and returns query results as JSON
- [ ] #4 query_sql rejects INSERT/UPDATE/DELETE with descriptive error
- [ ] #5 Tests verify successful reads and error responses
<!-- AC:END -->
