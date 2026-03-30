---
id: TASK-002.03.02
title: Create default templates for 8 artifact types
status: To Do
assignee: []
created_date: '2026-03-30 06:58'
labels: []
dependencies:
  - TASK-002.03.01
parent_task_id: TASK-002.03
priority: high
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Generate default template files for the 8 artifact types: Task, Bug, Epic, Feature, Sub-Epic, User-Story, Sub-Task, Decision. Each template uses section tags appropriate for that type (e.g., Task has description, acceptance-criteria, implementation-notes; Bug has steps-to-reproduce, expected-behavior, actual-behavior).

Update `WriteDefaults` and `backlogit init` to create `.backlogit/templates/` with these files and register them in `registry.yaml`.

**Files:** `internal/config/defaults.go`
**Test files:** `internal/config/defaults_test.go`
**Patterns:** Follow `WriteDefaults` in `internal/config/defaults.go`
**Verification:** `backlogit init` on a fresh directory produces `.backlogit/templates/` with 8 template files. Each loads successfully via `LoadTemplates`.
<!-- SECTION:DESCRIPTION:END -->
