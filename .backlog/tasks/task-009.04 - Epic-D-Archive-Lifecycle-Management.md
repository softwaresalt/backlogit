---
id: TASK-009.04
title: 'Epic D: Archive & Lifecycle Management'
status: In Progress
assignee: []
created_date: '2026-03-31 06:04'
labels:
  - sub-epic
  - phase-3
dependencies: []
references:
  - .backlog/exec-plans/2026-03-31-queue-features-v2-plan.md
parent_task_id: TASK-009
priority: high
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Add archive lifecycle management: a `backlogit archive` CLI command to move completed artifacts to an archive directory after branch merge, and commit tracking to associate git commits with work items for auto-archiving triggers. Include directory mapping in templates for tracking/archiving paths.

Phase 3 (Lifecycle). Units 8-9 from the implementation plan.

Review finding F4 (P2): Commit tracking shells out to git — implicit runtime dependency. Document the git requirement and handle gracefully when git is unavailable.
Review finding F5 (P2): Migration atomicity at scale — needs state file for crash recovery during bulk archive operations.
<!-- SECTION:DESCRIPTION:END -->
