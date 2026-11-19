---
title: "218-S Wave 1 convergence checkpoint"
description: "Wave 1 completion and full-suite convergence for tasks 217.001-T (U1) and 217.004-T (U4)."
doc_type: memory
source: docs/memory/2026-11-19/218-s-wave1-convergence.md
ingested_at: "2026-11-19T09:40:00Z"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
docline:
  shipment_id: 218-S
  feature_id: 217-F
  branch: feat/218-s
  wave: 1
---

## Wave 1 Summary

* **Completed**:
  * `217.001-T` (Unit U1, commit `e757d77489b4c92a6eefac7ff317b6476a41a873`): Added `ToolchainManifest.ToolsDir` (`internal/toolchain/types.go`) and updated `internal/toolchain/verify.go` to support `.bin` default, `bin` fallback, and configurable `tools_dir` while preserving mixed inventories and rejecting `.git`/`.backlog`/`.backlogit`, traversal, and symlinks/junctions.
  * `217.004-T` (Unit U4, commit `e4413c36cdead7371ca4a68bd7cdb5bfe18269a7`): Updated `scanLocalBranchArtifactIDsContext` (`internal/core/id_history.go`) to batch and deduplicate branch-tree inspection across all `refs/heads` via `git cat-file --batch` alongside `branchIDScanCache`, preserving merged-then-reverted branch reservations.
* **Wave 1 Convergence Gate (P-002.6)**:
  * `go test -run=^$ -count=1 ./...`: PASS
  * `go vet ./...`: PASS
  * `GOTOOLCHAIN=go1.26.5 golangci-lint run --timeout=5m`: PASS (0 issues)
  * `gofmt -l cmd internal scripts tests`: PASS (clean)
  * `GOTOOLCHAIN=go1.26.5 go test -timeout=30m -p 1 ./...`: PASS (all packages green, 0 open red deliverables)

## Next Step

* Admit Wave 2 (`217.002-T`), scaffold RED harness, implement, verify, and converge.
