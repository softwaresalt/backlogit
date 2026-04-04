---
id: TASK-001.04.06
title: Implement Workspace orchestration struct
status: Done
assignee: []
created_date: '2026-03-30 01:41'
labels: []
dependencies: []
parent_task_id: TASK-001.04
priority: high
ordinal: 4600
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Expand `internal/core/workspace.go` with:

1. `Workspace` struct — holds `*config.WorkspaceConfig`, `*sql.DB`, event stream path, and workspace root path. Coordinates cross-store write operations (per review P1-06).
2. `NewWorkspace(ctx context.Context, rootPath string) (*Workspace, error)` — loads config, opens DB connection (single `*sql.DB` per review P1-05), ensures schema, returns ready workspace.
3. `Close() error` — closes DB connection with error logging.
4. Write coordination: `Workspace` methods on CreateArtifact/UpdateArtifact perform file write → DB upsert → event append as ordered steps. If DB upsert fails, log warning and mark index as stale (rehydration self-heals). If file write fails, return error immediately.

Create `internal/core/workspace_test.go` with integration tests using `t.TempDir()`.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Workspace struct holds config, DB connection, event stream path, and workspace root
- [ ] #2 Write operations coordinate Markdown file write then DB upsert then event append
- [ ] #3 If DB upsert fails after file write, index is marked stale and warning is logged
- [ ] #4 Workspace.Close() closes DB connection cleanly
- [ ] #5 Tests verify cross-store coordination and graceful degradation
<!-- AC:END -->


## Implementation Notes

Completed in commit 83cebfc. Gates passed: `go test ./...`, `go vet ./...`, `golangci-lint run`.