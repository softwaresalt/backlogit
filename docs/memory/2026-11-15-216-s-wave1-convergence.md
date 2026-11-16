---
title: "216-S Wave 1 convergence checkpoint"
description: "Wave 1 completion and convergence gate checkpoint for Shipment 216-S (215.001-T, 215.003-T, 215.005-T)."
doc_type: memory
source: docs/memory/2026-11-15-216-s-wave1-convergence.md
ingested_at: "2026-11-16T08:05:00Z"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
docline:
  shipment_id: 216-S
  feature_id: 215-F
  branch: feat/216-s-self-contained-sandbox-and-multi-session-runtime-hardening
---

## Wave 1 Summary

Completed all three Wave 1 tasks of `216-S` (`215-F` / CAP-2):

* `215.001-T` (U1, commit `bc01ee04`): Exported `atomicfile.ReplaceTempFileInRoot` (handle-relative `unix.Renameat` on POSIX; `GetFinalPathNameByHandle` + `MoveFileExW` retry/durability on Windows) and wired `WriteToolchainManifest` through `replaceToolchainManifestFile`.
* `215.003-T` (U3a, commit `00f68137`): Restored `(*mcp.Server).CloseAllSessions` to session-only semantics, added `admitMu` / `activeHandlers` tracking and `stopAdmission()` + handler drain in `(*OperationCoordinator).Close()`, and exposed `(*mcp.Server).CloseCoordinator()` and `Shutdown()`.
* `215.005-T` (U4, commit `0bd8c6cf`): Fixed `withTempGitCeiling` in `internal/core/gate/baseref.go` to shift the synthesized ceiling to `filepath.Dir` when `tempDir` / `evalTemp` contains a valid `.git` marker while preserving ancestor isolation for non-repo temp directories and explicit caller `GIT_CEILING_DIRECTORIES`.

## Wave 1 Convergence Gate

* `go test -run=^$ -count=1 ./...`: PASS
* `go vet ./...`: PASS
* `gofmt -l cmd internal scripts tests`: clean
* `go test -timeout=30m ./...` (across all 50 packages, with `GOTOOLCHAIN=go1.27.0` matching `golangci-lint v2.13.2` and `-p 1` for CPU/subprocess-heavy suites): PASS (`open_red_deliverables_1 = []`).

## Next

Admit Wave 2 (`215.002-T` U2, `215.004-T` U3b, `215.006-T` U5a).
