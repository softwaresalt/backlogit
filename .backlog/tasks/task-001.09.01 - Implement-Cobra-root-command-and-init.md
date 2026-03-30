---
id: TASK-001.09.01
title: Implement Cobra root command and init
status: Done
assignee: []
created_date: '2026-03-30 01:44'
labels: []
dependencies: []
parent_task_id: TASK-001.09
priority: high
ordinal: 9100
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Create `internal/cli/root.go` with:

1. Cobra root command `backlogit` with `--cwd` persistent flag, version info
2. `log/slog` structured logging setup based on `BACKLOGIT_LOG_LEVEL` and `BACKLOGIT_LOG_FORMAT`
3. Workspace resolution: `--cwd` flag → `BACKLOGIT_WORKSPACE` env → `os.Getwd()`

Create `internal/cli/init.go` with:

1. `backlogit init` command — creates `.backlogit/` directory, writes default `config.yaml` and `registry.yaml` via `config.WriteDefaults`, creates subdirectories per registry rules, initializes empty `events.jsonl` and `telemetry.jsonl`, creates empty `index.db`
2. `--legacy` flag — when set, uses read-only adapter mode: parses existing `backlog.md` files without modifying them, generates initial `index.db` from heuristic parsing

Create `internal/cli/root_test.go` and `internal/cli/init_test.go` with tests.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Root command supports --cwd flag and BACKLOGIT_WORKSPACE env var
- [ ] #2 backlogit init creates .backlogit/ with default config.yaml, registry.yaml, and directory structure
- [ ] #3 backlogit init --legacy reads existing backlog.md without modifying it
- [ ] #4 Repeated init on existing workspace is safe (no data loss)
- [ ] #5 Tests verify init output, directory creation, and --legacy flag behavior
<!-- AC:END -->


## Implementation Notes

Completed in commit a49b9dd. Gates passed: `go test ./...`, `go vet ./...`, `golangci-lint run`.