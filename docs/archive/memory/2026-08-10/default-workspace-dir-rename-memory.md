---
title: Default workspace directory rename memory
date: 2026-08-10
feature: 135-F
shipment: 121-S
---

# Summary

Implemented dual-root workspace support with `.backlog/` as the default for
new workspaces and `.backlogit/` as the supported legacy root.

## Completed tasks

* 135.001-T - Added dual-root resolver test matrix
* 135.002-T - Implemented resolver, override validation, and ambiguous-root errors
* 135.003-T - Updated core archive and scan paths and added literal guard coverage
* 135.004-T - Changed `init` to create `.backlog/` and refuse legacy-root collisions
* 135.005-T - Added `migrate --workspace-dir` core + CLI support with tests
* 135.006-T - Added doctor workspace-root conflict detection for core, CLI, and MCP
* 135.007-T - Updated MCP path resolution, startup errors, and storage-root usage
* 135.008-T - Updated README, migration guide, and AGENTS.md
* 135.009-T - Updated agent instruction files and backlog registry defaults

## Files modified

* `internal/core/workspace.go`
* `internal/core/canonical_scan.go`
* `internal/core/archive.go`
* `internal/core/shipment_verify.go`
* `internal/core/doctor.go`
* `internal/core/migrate_workspace_dir.go`
* `internal/cli/root.go`
* `internal/cli/migrate.go`
* `internal/cli/doctor.go`
* `internal/mcp/server.go`
* `internal/mcp/errors.go`
* `internal/mcp/tools.go`
* `internal/telemetry/*.go`
* `internal/db/telemetry_schema.go`
* `README.md`
* `docs/migration-guide.md`
* `AGENTS.md`
* `.github/instructions/backlog*.md`
* `.autoharness/backlog-registry.yaml`

## Decisions

* `Workspace` now stores `StorageRoot` separately from `RootPath`
* `BACKLOGIT_WORKSPACE_DIR` is a closed-set override: `.backlog` or `.backlogit`
* Ambiguous dual-root workspaces fail closed with a typed error
* Safety-critical path joins now resolve from the effective storage root
* Telemetry and DB telemetry rehydration now probe both supported roots

## Failed approaches

* Initial shipment-verify helper placement broke parsing and was moved out of
  the function body
* A broad literal-guard scan caught telemetry and DB paths outside the intended
  safety-critical scope, so the guard was narrowed to core, CLI, MCP, and config

## Open questions

* The generated AGENTS.md remains larger than the architecture-doc guidance; this
  session only applied the requested storage-root wording updates
* We could later normalize more user-facing CLI help text from legacy-root
  references to default-root wording

## Verification

* `go test ./...`
* `go vet ./...`
* `golangci-lint run`
* `gofmt -l .`

## Next steps

* If the harness exposes `compact-context`, run it against the updated memory set
* Consider follow-up cleanup for remaining legacy-root phrasing in non-critical
  comments and help text
