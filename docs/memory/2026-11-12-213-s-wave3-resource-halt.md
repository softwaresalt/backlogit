---
title: "213-S wave 3 resource halt"
description: "Wave convergence halted on Windows commit-memory exhaustion"
doc_type: reference
source: docs/memory/2026-11-12-213-s-wave3-resource-halt.md
schema_version: "1.0"
chunk_strategy: h1-h2-h3
date: "2026-11-12"
status: resolved
---

## Halt

* Shipment `213-S`, wave 3 convergence; task members `212.001-T`,
  `212.002-T`, and `212.003-T` are done
* Branch: `feat/213-s-workspace-tool-environment-inventory-level-1-version-pinning-schema-and-threat-model-contract`
* U10 documentation commit: `249e75e490de5381f8cd0b03be022b126ad130dc`
* Completion commit: `b3b4ce9cf4d5b399f438baed5aff6718a189a23d`
* U10 exemption baseline: `aad05d772fc4acaf51c968a43d169d7536a36cd5`

The wave compile command `go test -run=^$ -count=1 ./...` failed while
starting Go linker and vet processes. Windows reported a paging file too small,
insufficient system resources, and Go memory allocation failures. A preceding
task-level compile, vet, lint, format, and exemption verification passed.
Wave 3 vet, lint, format, scoped verification, and the full suite did not run.
This is the first wave 3 convergence failure.

The resource snapshot showed approximately 4.5 GiB free virtual commit and
29.7 GiB free physical memory. No unrelated process was terminated. No PR
was created and no merge was performed.

## Resume

Structured checkpoints `checkpoint-20261112-205339.json` and `checkpoint-20261112-220807.json` were resolved and cleaned up (`ZERO-CANDIDATE NORMAL STARTUP` — `0` active structured checkpoints; do not call `backlogit_get_checkpoint`). All three task members (`212.001-T`, `212.002-T`, `212.003-T`) are `done` in `.backlogit/archive/` (`0` active tasks in `M`).

Wave 3 convergence attempt 2 passed compile, vet, lint, format, and `212.003-T` exempt verification, and failed only on `TestU2_ReconstituteShipmentsAtomicExecutionAndRollback/concurrent_associate_commit_and_post_hook_isolation` because `GOMEMLIMIT=512MiB` + `GOGC=50` caused GC thrashing inside `core.test.exe` while holding `shipment-lifecycle-global`. Removing `GOMEMLIMIT` and `GOGC` while keeping `Remove-Item Env:GOMAXPROCS, Env:GOGC, Env:GOMEMLIMIT -ErrorAction SilentlyContinue; $env:GOFLAGS = "-count=1 -p=1 -parallel=4"` eliminates GC thrashing while serializing package compilation (`TestU2_ReconstituteShipmentsAtomicExecutionAndRollback` verified passing in 48.1s).

Resume via `ZERO-CANDIDATE NORMAL STARTUP` and run the complete Wave 3 Step 4.6 convergence gate (attempt 3 of 3) with `Remove-Item Env:GOMAXPROCS, Env:GOGC, Env:GOMEMLIMIT -ErrorAction SilentlyContinue; $env:GOFLAGS = "-count=1 -p=1 -parallel=4"`, then proceed to Step 5 (PR lifecycle & merge) and Step 6 (post-merge closure).
