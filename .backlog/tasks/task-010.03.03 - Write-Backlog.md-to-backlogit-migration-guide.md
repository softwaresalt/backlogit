---
id: TASK-010.03.03
title: Write Backlog.md to backlogit migration guide
status: done
assignee: []
created_date: '2026-04-01 22:34'
labels:
  - docs
dependencies:
  - TASK-010.01
parent_task_id: TASK-010.03
priority: medium
ordinal: 3000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Create `docs/migration-guide.md` providing a user-facing guide for migrating from Backlog.md to backlogit.

Content to include:
- Prerequisites: backlogit installed, existing Backlog.md workspace
- Step 1: Initialize backlogit workspace alongside existing files
- Step 2: Run `backlogit migrate` pointing at the legacy backlog.md
- Step 3: Review migrated artifacts (status mapping, hierarchy preservation)
- Step 4: Configure artifact types and templates for the new workspace
- Before/after examples: how a Backlog.md checklist becomes a backlogit artifact with YAML frontmatter
- Status mapping table: Backlog.md checked/unchecked → backlogit status values
- Troubleshooting: common migration issues and resolutions
- Rollback: how to revert if migration produces unexpected results

Files to create:
- `docs/migration-guide.md` (new)

Depends on migration tooling sub-epic (TASK-010.01) for accurate CLI command documentation.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Step-by-step guide covers full migration from Backlog.md workspace to backlogit
- [ ] #2 Before/after examples show format transformation for common artifact types
- [ ] #3 Troubleshooting section addresses common migration failures
<!-- AC:END -->
