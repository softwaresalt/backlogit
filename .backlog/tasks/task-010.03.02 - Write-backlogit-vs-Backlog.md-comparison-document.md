---
id: TASK-010.03.02
title: Write backlogit vs Backlog.md comparison document
status: To Do
assignee: []
created_date: '2026-04-01 22:34'
labels:
  - docs
dependencies:
  - TASK-010.03.01
parent_task_id: TASK-010.03
priority: medium
ordinal: 2000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Create `docs/backlogit-vs-backlog-md.md` differentiating backlogit from Backlog.md.

Content to include:
- Feature comparison matrix: storage model, query capabilities, agent integration, migration support, type system, dependency tracking
- Architectural differences: flat Markdown files vs. hybrid CQRS (Markdown + SQLite + JSONL)
- Token efficiency comparison: dumping full files vs. targeted SQL queries
- Use case guidance: when Backlog.md is sufficient vs. when backlogit adds value
- Compatibility: backlogit can read and migrate Backlog.md workspaces
- What backlogit solves that Backlog.md does not: structured queries, WIT type system, work queue, dependency graph, artifact templates, MCP protocol

Files to create:
- `docs/backlogit-vs-backlog-md.md` (new)

Verification: All feature claims are grounded in actual codebase capabilities.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Feature comparison matrix covers storage, querying, agent integration, and migration
- [ ] #2 Architectural differences section explains CQRS vs flat-file approaches
- [ ] #3 Use cases table shows when to use each tool
<!-- AC:END -->
