---
id: TASK-002.04.04
title: Implement CLI update command
status: done
assignee: []
created_date: '2026-03-30 07:00'
labels: []
dependencies:
  - TASK-002.01.05
  - TASK-002.03.03
parent_task_id: TASK-002.04
priority: high
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Implement `backlogit update <id> [--title <title>] [--status <status>] [--priority <priority>] [section flags]` in `internal/cli/update.go`. Updates frontmatter fields via `core.UpdateArtifact`. Section flags (defined in the template) update individual sections via the section writer. Enforce ID immutability: reject `--id` flag. Support stdin for multi-line section content (flag value `-`). Re-sync SQLite index after update.

Include slog instrumentation per review F4.

**Files:** `internal/cli/update.go` (new)
**Test files:** `internal/cli/update_test.go` (new)
**Patterns:** Follow `newSyncCommand` at `internal/cli/root.go`
**Verification:** `go test ./internal/cli/...` passes with update tests for both metadata and section content. ID update attempt produces descriptive error.
<!-- SECTION:DESCRIPTION:END -->

