---
id: TASK-009.04.01
title: Archive Command & Lifecycle Management
status: Done
assignee: []
created_date: '2026-03-31 06:06'
updated_date: '2026-04-01 05:19'
labels:
  - task
  - phase-3
dependencies:
  - TASK-009.01.01
parent_task_id: TASK-009.04
priority: high
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
**Unit 8 — Archive Command & Lifecycle Management**

Add archive lifecycle management to move completed work items to an archive directory, keeping the active queue clean.

Key deliverables:
- New CLI command: `backlogit archive <id> [--all-done]`
- New MCP tool: `backlogit_archive_item`
- Archive directory: `.backlogit/archive/` mirrors queue structure
- Archive preserves full artifact content (frontmatter + body)
- Archived artifacts excluded from default `list` and `query` results (add `--include-archived` flag)
- Archive event logged to `events.jsonl`
- Update `internal/core/routing.go` with archive path resolution
- Update `internal/db/queries.go`: archived items filtered by default, queryable with flag
- Directory mapping in templates (queue.md requirement #8): templates include `archive_dir` path for type-specific archive locations
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 backlogit archive <id> moves artifact to .backlogit/archive/ directory
- [x] #2 Archive preserves full artifact content (frontmatter + body)
- [x] #3 Archived artifacts excluded from default list/query results
- [x] #4 backlogit archive --all-done archives all artifacts with status=done
- [x] #5 Archive event appended to events.jsonl
- [x] #6 SQLite items table updated: status set to 'archived' or row moved to archive table
<!-- AC:END -->
