---
title: "218-S session summary and pre-PR execution checkpoint"
description: "Ship execution summary for Shipment 218-S (Feature 217-F, Units U1-U4) across Waves 1-3."
doc_type: memory
source: docs/memory/2026-11-19/218-s-session-summary.md
ingested_at: "2026-11-19T13:15:00Z"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
docline:
  shipment_id: 218-S
  feature_id: 217-F
  branch: feat/218-s
  status: ready_for_review
---

## Items Completed

* **`217.001-T` (Unit U1, Wave 1, commit `e757d774`)**: Added optional `ToolsDir` (`tools_dir`) to `ToolchainManifest` (`internal/toolchain/types.go`) and updated `internal/toolchain/verify.go` to accept `.bin/backlogit[.exe]`, `bin/backlogit[.exe]`, and configured workspace-local `tools_dir` while preserving mixed inventories (`!isBacklogitTool` early return) and rejecting `.git`/`.backlog`/`.backlogit`, traversal, and symlinks/junctions.
* **`217.004-T` (Unit U4, Wave 1, commit `e4413c36`)**: Updated `scanLocalBranchArtifactIDsContext` (`internal/core/id_history.go`) to query all `refs/heads` root tree OIDs via `git for-each-ref`, deduplicate root/subtree OIDs, and batch tree inspection via `git cat-file --batch` alongside `branchIDScanCache`, preserving merged-then-reverted branch reservations.
* **`217.002-T` (Unit U2, Wave 2, commit `b4081144`)**: Added optional `tools_dir` to `schemas/workspace-toolchain.schema.json` and updated `internal/toolchain/launcher.go` provisioning validation and manifest-aware remediation paths (defaulting to `.bin` while preserving `./bin/...` for legacy `bin` manifests).
* **`217.003-T` (Unit U3, Wave 3, commits `5e894669`, `c8523d6e`)**: Added `WorkspaceConfig.ToolsDir` validation (`internal/config/schema.go`, `internal/config/loader.go`) and updated `pinnedMCPDaemonBinary` (`internal/cli/mcp_daemon.go`) to resolve `.mcp.json` -> `toolchain.yaml` -> `config.yaml` `tools_dir` -> `.bin` -> `bin` via step-by-step pinned child `*os.Root` directory handle traversal (`Lstat` -> `rejectLink` -> `OpenRoot` -> `Stat(".")` + `SameFile`, opening leaf on innermost pinned parent handle), using `core.WorkspaceRootCandidates()` for reserved-directory checks.

## Verification State

* `go test -run=^$ -count=1 ./...`: PASS
* `go vet ./...`: PASS
* `GOTOOLCHAIN=go1.26.5 golangci-lint run --timeout=5m`: PASS (0 issues)
* `gofmt -l cmd internal scripts tests`: PASS (0 unformatted files)
* `GOTOOLCHAIN=go1.26.5 go test -timeout=30m -p 1 ./...`: PASS (all packages green)
* `go build ./cmd/backlogit`: PASS (Note: `.bin/backlogit.exe` is currently locked by the running MCP server PIDs `7816` and `25480` and will be refreshed post-session when the daemon restarts).

## Next Steps

* Run local review gate (`mode:report-only`), verify docs/markdown lint, open PR on `feat/218-s`, satisfy CI + P-018 Copilot review + runtime verification, merge on approval, and execute Step 6 post-merge closure.
