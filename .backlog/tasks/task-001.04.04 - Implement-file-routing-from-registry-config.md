---
id: TASK-001.04.04
title: Implement file routing from registry config
status: Done
assignee: []
created_date: '2026-03-30 01:40'
labels: []
dependencies: []
parent_task_id: TASK-001.04
priority: high
ordinal: 4400
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Create `internal/core/routing.go` with:

1. `ResolveTargetDir(registry *config.RegistryConfig, artifactType string, status string) string` — matches artifact type and status against registry directory conditions to determine the target directory path. Falls back to a default directory when no condition matches.
2. `MoveArtifactFile(ctx context.Context, workspaceRoot string, currentPath string, newDir string) (string, error)` — relocates an artifact Markdown file from its current directory to the new target directory. Uses SafeResolve for path validation and atomic rename.

Create `internal/core/routing_test.go` with table-driven tests for various status/type combinations.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 ResolveTargetDir returns sprint-board/todo/ for status=todo
- [ ] #2 ResolveTargetDir returns sprint-board/active/ for status=in_progress
- [ ] #3 ResolveTargetDir returns correct directory based on registry.yaml conditions
- [ ] #4 Returns fallback directory when no condition matches
- [ ] #5 Tests cover status-based routing and type-based routing
<!-- AC:END -->


## Implementation Notes

Completed in commit 83cebfc. Gates passed: `go test ./...`, `go vet ./...`, `golangci-lint run`.