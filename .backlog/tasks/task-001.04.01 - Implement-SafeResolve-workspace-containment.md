---
id: TASK-001.04.01
title: Implement SafeResolve workspace containment
status: To Do
assignee: []
created_date: '2026-03-30 01:40'
labels: []
dependencies: []
parent_task_id: TASK-001.04
priority: high
ordinal: 4100
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Create `internal/core/workspace.go` with:

1. `SafeResolve(workspaceRoot, target string) (string, error)` — resolves target path relative to workspace root, validates the resolved path stays within the workspace boundary. Returns `ErrValidation` if the target contains `..` traversal that escapes the workspace.

Per review P1-10: All file operations throughout the codebase must use SafeResolve before filesystem access. This is the containment boundary mandated by Constitution Principle IV.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 SafeResolve returns absolute path for valid targets within workspace
- [ ] #2 SafeResolve returns error for paths containing .. that escape workspace
- [ ] #3 SafeResolve returns error for absolute paths outside workspace
- [ ] #4 Tests cover normal paths, relative paths, traversal attempts, symlink edge cases
<!-- AC:END -->
