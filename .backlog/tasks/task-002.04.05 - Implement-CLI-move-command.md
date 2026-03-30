---
id: TASK-002.04.05
title: Implement CLI move command
status: To Do
assignee: []
created_date: '2026-03-30 07:00'
labels: []
dependencies:
  - TASK-002.01.05
parent_task_id: TASK-002.04
priority: high
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Implement `backlogit move <id> --status <new_status>` in `internal/cli/move.go`. Changes the artifact status and physically relocates the file according to `registry.yaml` routing rules. Uses `core.UpdateArtifact` for status change and `core.MoveArtifactFile` for relocation. Re-syncs the index after move.

Include slog instrumentation per review F4.

**Files:** `internal/cli/move.go` (new)
**Test files:** `internal/cli/move_test.go` (new)
**Patterns:** Follow `MoveArtifactFile` in `internal/core/routing.go`
**Verification:** `go test ./internal/cli/...` passes. Integration test: move changes both status in frontmatter and file location on disk.
<!-- SECTION:DESCRIPTION:END -->
