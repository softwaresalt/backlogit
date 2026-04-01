---
id: TASK-002.04.01
title: Implement CLI add command
status: done
assignee: []
created_date: '2026-03-30 06:59'
labels: []
dependencies:
  - TASK-002.01.05
  - TASK-002.02.01
  - TASK-002.03.03
parent_task_id: TASK-002.04
priority: high
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Implement `backlogit add --type <type> --title <title> [section flags]` in `internal/cli/add.go`. The command:
1. Opens the workspace via `core.NewWorkspace`
2. Resolves the type from `header-def.yaml`
3. Loads the appropriate template from `.backlogit/templates/`
4. Creates the artifact via `core.CreateArtifact` with functional options
5. Populates sections from flags or stdin (multi-line input buffer)
6. Writes the artifact file with template structure and section tags

For multi-line input: if a section flag value is `-`, read from stdin until EOF. Support pipe input (`echo "content" | backlogit add --type task --title "Foo" --description -`).

Include slog instrumentation: Info at command entry/exit, Debug for intermediate steps, Error for failures.

Register via `root.AddCommand(newAddCommand(&cwd))`.

**Files:** `internal/cli/add.go` (new)
**Test files:** `internal/cli/add_test.go` (new)
**Patterns:** Follow `newInitCommand` at `internal/cli/root.go`
**Verification:** `go test ./internal/cli/...` passes. Integration test: `add` creates a valid markdown file with frontmatter and section tags.
<!-- SECTION:DESCRIPTION:END -->

