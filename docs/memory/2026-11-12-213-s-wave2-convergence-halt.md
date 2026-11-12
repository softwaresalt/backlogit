---
title: "213-S Wave 2 convergence halt"
doc_type: reference
source: docs/memory/2026-11-12-213-s-wave2-convergence-halt.md
schema_version: "1.0"
chunk_strategy: h1-h2-h3
date: "2026-11-12"
status: resolved
---

## Cursor

* Shipment: `213-S`; Wave 2 convergence remains blocked
* Completed: `212.001-T`, `212.002-T`; next: `212.003-T`
* Branch: `feat/213-s-workspace-tool-environment-inventory-level-1-version-pinning-schema-and-threat-model-contract`
* U9 completion commit: `df2cc1b6`; implementation: `a435bbb6`
* Last U9 review: `READY_WITH_FOLLOWUPS` at `b8eeac9c`, P0=0, P1=0

## Gate evidence

Wave 2 compile, vet, lint, formatting, and the 12-test U9 race selector passed.
The unfiltered `go test -timeout=30m ./...` failed in `internal/core`:

* `TestRound3ReconstituteSnapshotOwners/structured=false`: item upsert
  transaction returned `context deadline exceeded`
* `TestU1f_ExecuteReconstitutionPreliminaryPlanReadsUnderCanonicalLocks`:
  dependency insertion transaction returned `context deadline exceeded`

The two named tests passed on a later targeted diagnostic run. That result does
not replace the failed unfiltered full-suite gate or establish its root cause.
No code fix or scope expansion was made. Wave 3 and PR work were not started.
Wave convergence failure count: 1.

## Next step

Diagnose and remediate the convergence failure within the authorized scope,
then repeat the required full Wave 2 convergence gate. Do not treat the
targeted diagnostic pass as full-suite success.

## Wave 2 Convergence PASS (Resolved)

Under bounded Go test concurrency (`Remove-Item Env:GOMAXPROCS -ErrorAction SilentlyContinue; $env:GOFLAGS = "-count=1 -p=2 -parallel=4"`), Wave 2 Step 4.6 convergence passed in full during `ship-213-s-run51` (recorded in `checkpoint-20261112-193115.json` and `checkpoint-20261112-194026.json`):

* `go test -run=^$ -count=1 ./...` exited `0`
* `go vet ./...` exited `0`
* `golangci-lint run` exited `0` (`0 issues`)
* `gofmt -l .` exited `0` (empty output)
* 12-test U9 race selector (`go test -timeout=30m -count=1 -race -run '^TestU9_' ./internal/toolchain`) exited `0`
* Unfiltered 46-package full suite (`go test -timeout=30m ./...`) exited `0`

Wave 2 is complete and converged; shipment `213-S` advances to Wave 3 (`212.003-T` / Unit U10, `harness-exempt: docs-only`).

