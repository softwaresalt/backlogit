---
title: "217-S Wave 4 convergence checkpoint"
description: "Wave 4 completion and convergence gate checkpoint for Shipment 217-S (216.006-T)."
doc_type: memory
source: docs/memory/2026-11-16-217-s-wave4-convergence.md
ingested_at: "2026-11-17T06:00:00Z"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
docline:
  shipment_id: 217-S
  feature_id: 216-F
  branch: feat/217-s-zero-flake-concurrency-and-fast-ship-closure-engine
---

## Wave 4 Summary

Completed Wave 4 task `216.006-T` (Unit U5, commit `7147dc0ef215ee5e4295503651a6e8a41f665073`):

* Bounded `custodyGit` in `internal/core/custody_exec.go` with `defaultCustodyGitTimeout = 15s`, `cmd.WaitDelay = 2s`, `-c core.fsmonitor=false -c gc.auto=0 --no-replace-objects --no-optional-locks`, and `GIT_TERMINAL_PROMPT=0`, preserving `context.DeadlineExceeded` / `context.Canceled` across `custodyCurrentBranch` and `custodyRefuseDefaultBranch`.
* Deduplicated and memoized `custodyBlobDigest` (`4,096`-entry bounded cache keyed by `filepath.Clean(root)|lowercase(oid)` for valid object names, failing closed on `ctx.Err()`) and skipped duplicate `git cat-file blob` spawns in `custodyLiveDigests` when `indexOID == baseOID`.
* Configured `core.fsmonitor=false` and `gc.auto=0` in `buildCustExecSeed` (while retaining `PRAGMA synchronous = FULL` in `custExecNewRepo`) and added bounded contexts plus step-timing / `Verdicts` diagnostics in `TestU4_CustodyPreviewOfferAcceptVerifyAndRecovery/profile_b_stage_to_ship_handoff_and_stale_digest_invalidation` and `TestExternal_LiveAcceptAndVerifyUseExternalStore`.

## Wave 4 Convergence Gate

* `go test -run=^$ -count=1 ./...`: PASS
* `go vet ./...`: PASS
* `bin/golangci-lint run --timeout=5m`: PASS
* `gofmt -l internal cmd tests`: clean
* `go test -timeout=30m -p=1 ./...` (`GOTOOLCHAIN=go1.27.0`): PASS across all 50/50 packages (`open_red_deliverables_4 = []`).

## Next

Admit Wave 5 (`216.007-T` / Unit U6).
