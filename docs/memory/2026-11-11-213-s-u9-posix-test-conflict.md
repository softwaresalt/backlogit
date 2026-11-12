---
title: "213-S U9 POSIX test conflict"
doc_type: reference
source: docs/memory/2026-11-11-213-s-u9-posix-test-conflict.md
schema_version: "1.0"
chunk_strategy: h1-h2-h3
date: "2026-11-11"
status: resolved
---

## State

* Shipment `213-S`; task `212.002-T` is placed in `status: "queued"` with `harness-ready` retained in `labels` (`["code", "track-a", "toolchain", "pinning", "u9", "harness-ready"]`).
* Branch `feat/213-s-workspace-tool-environment-inventory-level-1-version-pinning-schema-and-threat-model-contract`, HEAD `462458af`.
* `internal/toolchain/verify.go` contains uncommitted remediation; `verify_test.go` was not changed. No task completion or PR was performed.
* Windows U9 and package tests passed. POSIX execution was not performed.

## Blocker

The existing `casefold-env-binding` test expects a lowercase `engram_data_dir` binding to pass on every OS. The authorized remediation requires that spelling to be rejected on POSIX while prohibiting edits to `verify_test.go`. These requirements conflict. Route to Stage for a contract and test-scope resolution; do not silently weaken either requirement.

## Resolution

Stage aligned Item 4 of Unit U9 in `.backlogit/queue/212.002-T.md` and `docs/plans/2026-11-04-multi-session-mcp-runtime-plan.md` with the locked cross-platform `TestU9_ReviewFixMCPArgumentsAndBindings/casefold-env-binding` contract (`key = strings.ToUpper(strings.TrimSpace(key))` across all platforms with no `runtime.GOOS` branch) and deferred OS-specific POSIX case-sensitive matching under P-021 C2 stash `8841FD73`.

## Recovery

Structured checkpoint `checkpoint-20261112-041410.json` was resolved and cleaned up as part of the Stage `462458af` contract alignment. Stage placed `212.002-T` in `status: "queued"` with `harness-ready` retained so Ship enters via Step 4.0 Wave Admission (`ready_2 = {212.002-T}`, zero active residuals), skips Step 2 (`harness-architect`) because `harness-ready` is present, moves `212.002-T` to `active` at Step 4.1b item 1 while reusing the single existing `WORK_STARTED: 213-S` record in the `shipment claimed` epoch (`count == 1`, no duplicate append per Step 4.1b item 3), and dispatches Step 4.2 (`build-feature`) to finalize and commit `internal/toolchain/verify.go`.
