---
title: "213-S U9 containment review halt"
doc_type: reference
source: docs/memory/2026-11-11-213-s-u9-review-containment-halt.md
schema_version: "1.0"
date: "2026-11-11"
status: resolved
chunk_strategy: h1-h2-h3
---

## State

Shipment `213-S` and task `212.002-T` remain active. Task `212.003-T`
remains claim-assigned and waits on `212.002-T`. The feature branch is
`feat/213-s-workspace-tool-environment-inventory-level-1-version-pinning-schema-and-threat-model-contract`.
The reviewed HEAD is `1a185d7a5984cf24e2aa11867b6496fd90e1af35`.
Wave 2 has not converged; no PR or merge was attempted.

## Evidence

The scoped POSIX test observed the named assertion fail before the production
edit, then passed all 12 U9 top-level tests. The Windows scoped race harness
passed all 12 tests. Repository lint and format checks passed. The scheduler
simulation and compile-only preflight passed. Engram was unavailable.

The standard report-only review and three-reviewer adversarial review returned
`BLOCKED`. The latter identified high-confidence static traces for a
`backlogit` remote or host declaration bypassing the local-only path check
and an extensionless interpreter script passing without an entrypoint.
It also identified possible unpinned entrypoint contents and other findings.
These are static review findings, not executed reproductions.

## Next step

Do not complete the task or advance the wave. The requested production-only
edit boundary excludes new regression tests, so a test-first fix for these
newly reported behaviors requires a governed scope and harness disposition.
Return the findings to the operator and Stage. Preserve the existing single
start record and committed work. Do not silently requeue or restart the task.

## Resolution and recovery

Stage resolved the halt under P-013.5:

* Recorded P-021 C2 deferred scope expansion stash `1406FAE2` for adding a
  dedicated `entrypoint_sha256` field to `schemas/workspace-toolchain.schema.json`
  and `ToolEntry` in `internal/toolchain/types.go` (owned by completed Wave 1
  task `212.001-T` / Unit U8).
* Returned `212.002-T` to `status: "queued"` with `harness-ready` retained on
  its labels for Step 4.0 to Step 4.1b Wave 2 admission, reusing the single
  existing `WORK_STARTED: 213-S` record in the `shipment claimed` epoch.
* Lifted the prior `Zero verify_test.go Edits` restriction in
  `.backlogit/queue/212.002-T.md` and
  `docs/plans/2026-11-04-multi-session-mcp-runtime-plan.md` (Unit U9),
  authorizing test-first subtest additions inside the existing 12 top-level
  `TestU9_*` functions in `internal/toolchain/verify_test.go` and corresponding
  production fixes in `internal/toolchain/verify.go` for (a) `backlogit`
  non-local containment class or non-`bin/backlogit.exe` executable path
  rejection, (b) interpreter/runner binaries requiring a non-empty
  `entrypoint_path`, and (c) lockfile content referencing `entrypoint_path`
  when both `entrypoint_path` and `lockfile_path` are present.
* Confirmed `checkpoint-20261112-052838.json` is resolved and cleaned up from
  the working tree so `_Ship` starts under `ZERO-CANDIDATE NORMAL STARTUP`.
