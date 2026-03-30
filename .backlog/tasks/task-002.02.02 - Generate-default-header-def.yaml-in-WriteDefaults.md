---
id: TASK-002.02.02
title: Generate default header-def.yaml in WriteDefaults
status: To Do
assignee: []
created_date: '2026-03-30 06:58'
labels: []
dependencies:
  - TASK-002.02.01
parent_task_id: TASK-002.02
priority: high
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Extend `WriteDefaults` in `internal/config/defaults.go` to generate a default `header-def.yaml` with:
- The 8 queue-specified types: Epic, Feature, Sub-Epic, User-Story, Task, Sub-Task, Bug, Decision
- OP prefix with 3-digit ID format (`OP{NNN}`)
- System-managed immutable defaults for id, created_date, updated_date
- Per-type field schemas appropriate for each type

Update `backlogit init` to create this file alongside config.yaml and registry.yaml.

**Files:** `internal/config/defaults.go`
**Test files:** `internal/config/defaults_test.go` (new or expand)
**Patterns:** Follow `WriteDefaults` pattern in `internal/config/defaults.go`
**Verification:** `go test ./internal/config/...` passes. Generated `header-def.yaml` can be loaded back by `LoadHeaderDef` without errors.
<!-- SECTION:DESCRIPTION:END -->
