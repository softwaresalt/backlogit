---
chunk_strategy: h1-h2-h3
closure_status: READY
compaction_status: done
conditions:
  - id: feature-pr-521-merged
    description: "PR #521 (144-S implementation) is merged as a merge commit into main."
    satisfied: true
    evidence: "gh pr view 521: state MERGED, merge commit 4fa4c6e7f9a62edc60b9c9af2b5f2f9e1aff5ff4; final PR head fa35c7ff."
  - id: governed-closure-archived
    description: "Shipment 144-S and its explicit members are archived."
    satisfied: true
    evidence: "ship_shipment with sha 4fa4c6e7f9a62edc60b9c9af2b5f2f9e1aff5ff4 archived 144-S and 162-F; tasks 162.001-T..162.003-T were pre-archived and stamped."
description: "144-S (162-F shipment-level evidence DAG framework) post-merge closure for PR #521, merge 4fa4c6e7f9a62edc60b9c9af2b5f2f9e1aff5ff4. READY: the framework is an offline library with no deployed runtime surface."
doc_type: closure
docline:
  date: 2026-10-17T21:00:00Z
  status: draft
  tags:
    - operational-closure
    - post-merge
    - 144-S
    - 162-F
feature_id: 162-F
schema_version: "1.0"
shipment_id: 144-S
source: docs/closure/144-S-162-F-post-merge-closure.md
title: "144-S post-merge closure"
---

# 144-S post-merge closure

## Release record

| Field | Value |
|---|---|
| Shipment | `144-S` |
| Feature | `162-F` |
| Implementation PR | [#521](https://github.com/softwaresalt/backlogit/pull/521) |
| Merge commit | `4fa4c6e7f9a62edc60b9c9af2b5f2f9e1aff5ff4` (merge commit, P-009) |
| Final reviewed HEAD | `fa35c7ffa6af8b9d613092dc03b44b94e8736799` |
| Closure branch | `post-merge/144-s10-evidence-dag-framework` |
| Closure status | `READY` |
| compaction status | `done` |

## Summary

A new package `internal/evidencedag` provides a shipment-level fault-line evidence DAG: a strict declaration loader and validator, a shared graph-evaluation core with incremental evaluation (a library for S11 to consume), detector integration as node executors with evidence binding and anti-replay checks, and the authoritative review-ready gate with ed25519 signed out-of-band waivers and an anchored hash-chain waiver ledger. Documented in `docs/design-docs/2026-10-17-s10-evidence-dag-framework.md`.

## Review record

* Two pre-PR adversarial multi-persona review rounds (correctness, security, go-reviewer, concurrency, maintainability) were remediated before the PR opened.
* Copilot ran 5 fix cycles covering 12 threads; all 12 were fixed and none were deferred. The cap of 8 was not reached. A final read-only delta review of the Copilot-fix commits found no P0/P1/P2.
* The copilot-review gate returned SATISFIED on `fa35c7ff` and every required CI check passed. Merge was a normal merge commit; no admin fallback was used.
* Local verification: `go build ./cmd/backlogit`, `go vet ./...` and gofmt were clean, and the `internal/evidencedag` package passed with and without `-race`. The unfiltered `go test -timeout=30m ./...` passed for every package except `internal/core`, which is untouched by this change and hit the 30-minute timeout under heavy parallel load; its flagged tests (`TestU172_LockOrder_*`, `TestIsAncestor`, `TestShipmentLifecycleBarrier_*`) passed rerun alone and CI `test` passed.

## Runtime verification and releasability

Runtime-verification is not applicable: the change is an offline library with no deployed runtime surface. Releasability: `READY`.

* Owner: Ship / operator.
* Rollback: revert merge `4fa4c6e7f9a62edc60b9c9af2b5f2f9e1aff5ff4` through a PR that preserves `.backlogit/` state.
* Validation window: the gate is library-only; `ModeReportOnly` is the rollout switch for S11 consumers.
* Consumer wiring (the S11 policy engine composing into this DAG) is planned work and does not gate this release.

## Residual risk

* Evidence "authenticity" is binding and digest self-consistency, not cryptographic provenance; evidence signing is deferred (stash `5BDD615F`).
* Fault-line families `mutation-state`, `compat-error`, `docs-audit` and `plan-task-soundness` evaluate as unverifiable (fail closed) until S4 adds their evidence families.
* Waiver ledger tamper evidence depends on a caller-supplied external anchor; see the design doc's Documented limitations.
* One race-mode timing flake was seen once locally in `TestHardening_HolderOutlivingLockStaleKeepsItsLockViaHeartbeat`; it passed on rerun and in CI.
* The `claim_shipment` and `ship_shipment` MCP calls timed out client-side and completed server-side; state was verified by re-reading, never by retry.

## Governed closure

* Pre-closure baseline was clean apart from `.backlogit/hooks_queue.jsonl`, which is outside the predicted write set. The stale 153-S checkpoint was left alone.
* All three tasks were pre-archived; `162-F` was pending governed completion and `144-S` was `active`. The ship archived `144-S` and `162-F` and stamped the tasks with the merge commit. No archive file was deleted (P-007).
* Provenance: no source stash or deliberation was archived by Ship.

## Follow-ups

Deferred stash entry requiring Stage triage: `5BDD615F`.

The next shipment `145-S` (deterministic workflow-policy enforcement engine) is the final shipment in the sequence. It depends on `144-S`, which is now shipped and archived, so nothing in this artifact blocks it. It was not started.

## P-020 compaction

compact-context ran with `target: all`. The 144-S checkpoint was compacted into `docs/memory/compacted/2026-10-17-144s-ship-compacted.md` and moved, not deleted, to `docs/archive/memory/2026-10-17/`. Outcome: `done`.

## Provenance

Compound refresh: `docs/closure/2026-10-17-144-s-compound-refresh.md`.
