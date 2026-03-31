---
id: TASK-001.02.02
title: Implement YAML config loader with env var overrides
status: Done
assignee: []
created_date: '2026-03-30 01:38'
labels: []
dependencies: []
parent_task_id: TASK-001.02
priority: high
ordinal: 2200
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Create `internal/config/loader.go` with:

1. `Load(workspacePath string) (*WorkspaceConfig, error)` — reads `config.yaml`, `registry.yaml`, `hooks.yaml` from workspace; unmarshals into typed structs; applies env var overrides; validates with `go-playground/validator`
2. `applyEnvOverrides(cfg *WorkspaceConfig)` — checks `BACKLOGIT_WORKSPACE`, `BACKLOGIT_LOG_LEVEL`, `BACKLOGIT_LOG_FORMAT` via `os.LookupEnv`
3. Custom validation: `allowed_children` references must exist as keys in `artifact_types` map
4. All errors wrapped with `ErrConfig` sentinel

Create `internal/config/loader_test.go` with table-driven tests using `t.TempDir()` for temp config files.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Load() reads and validates config.yaml from workspace path
- [ ] #2 Load() returns wrapped ErrConfig on missing file or invalid YAML
- [ ] #3 BACKLOGIT_WORKSPACE, BACKLOGIT_LOG_LEVEL, BACKLOGIT_LOG_FORMAT env vars override config
- [ ] #4 Validates allowed_children references exist in artifact_types map
- [ ] #5 Table-driven tests cover valid config, missing files, invalid YAML, validation failures, env overrides
<!-- AC:END -->


## Implementation Notes

Completed in commit 83cebfc. Gates passed: `go test ./...`, `go vet ./...`, `golangci-lint run`.