---
id: TASK-009.06.02
title: CLI Enhancements (Tabular Listing)
status: Done
assignee: []
created_date: '2026-03-31 06:07'
updated_date: '2026-04-01 05:20'
labels:
  - task
  - phase-2
dependencies:
  - TASK-002
parent_task_id: TASK-009.06
priority: medium
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
**Unit 13 — CLI Enhancements (Tabular Listing)**

Improve CLI tabular listing with status filtering, type filtering, and full-view rendering for individual artifacts.

Key deliverables:
- Enhance `internal/cli/list.go`: add `--status`, `--type`, `--assignee`, `--priority` filter flags
- Improve tabwriter alignment with consistent column headers
- Add color coding for status values (if terminal supports it)
- Enhance `internal/cli/get.go`: show full artifact detail including all frontmatter fields, description body, dependencies, and associated commits
- Add `--format json` flag for machine-readable output
- Update help text for all enhanced commands
- Performance: stream results from SQLite rather than loading all into memory

This is an enhancement of existing CLI commands (queue.md requirement #9), not new commands.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 backlogit list shows aligned tabular output with status, priority, ID, title columns
- [x] #2 backlogit list --status active filters by status
- [x] #3 backlogit list --type bug filters by artifact type
- [x] #4 backlogit get <id> shows full artifact detail including all frontmatter fields and body
- [x] #5 Column widths auto-adjust to content using tabwriter
- [x] #6 Empty result sets show descriptive 'no items found' message
<!-- AC:END -->
