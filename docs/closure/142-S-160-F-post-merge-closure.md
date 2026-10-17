---
chunk_strategy: h1-h2-h3
closure_status: READY_WITH_CONDITIONS
compaction_status: done
conditions:
  - id: feature-pr-515-merged
    description: "PR #515 (142-S implementation) is merged as a merge commit into main."
    satisfied: true
    evidence: "gh pr view 515: state MERGED, merge commit a61d409eac65eedbbbe22cf107cfd8b4c3bddab7; final PR head bf389af4."
  - id: governed-closure-archived
    description: "Shipment 142-S and its explicit members are archived."
    satisfied: true
    evidence: "ship_shipment recorded status archived/shipped with sha a61d409e; archive commit 6aaf85b5 on the closure branch."
  - id: soundness-gate-not-wired
    description: "The soundness linter is not wired into shipment assembly or CI."
    satisfied: false
    evidence: "Deferred as stash 09B597EE; run `backlogit soundness lint` in report mode before enforcing."
description: "142-S (160-F plan and work-item soundness linter) post-merge closure for PR #515, merge a61d409e. READY_WITH_CONDITIONS: the linter is not yet wired into assembly or CI."
doc_type: closure
docline:
  date: 2026-10-17T02:30:00Z
  status: draft
  tags:
    - operational-closure
    - post-merge
    - 142-S
    - 160-F
feature_id: 160-F
schema_version: "1.0"
shipment_id: 142-S
source: docs/closure/142-S-160-F-post-merge-closure.md
title: "142-S post-merge closure"
---

# 142-S post-merge closure

## Release record

| Field | Value |
|---|---|
| Shipment | `142-S` |
| Feature | `160-F` |
| Implementation PR | [#515](https://github.com/softwaresalt/backlogit/pull/515) |
| Merge commit | `a61d409eac65eedbbbe22cf107cfd8b4c3bddab7` (merge commit, P-009) |
| Reviewed HEADs | `ac6d5f04` -> `0021ac2e` -> `3c28cc81` -> `bf389af4` (final) |
| Closure branch | `post-merge/142-s8-plan-and-work-item-soundness-linter` |
| Closure status | `READY_WITH_CONDITIONS` |
| compaction status | `done` |

## Summary

A new package `internal/soundness` and the CLI `backlogit soundness lint` check plans and work items for soundness: budgets, references, contradictions, duplicated work, dependency consistency, missing artifacts, verifiability and canonical procedures. The gate supports report-only and enforce modes.

## Review record

* Copilot ran 6 fix cycles. The prior session stopped at the 3-cycle limit. The operator then explicitly authorized cycles 4-6 for this PR only, without widening scope (P-021 C1).
* Cycles 4-6 resolved 3 review threads and dispositioned 2 review-body findings in a PR comment. The three round-4 findings were: bounded contradictions, transitive related-work reachability, and deliverable deduplication. Round 5 fixed raw `..` in references. Round 6 added a per-run reference cache.
* A five-persona adversarial self-audit ran before cycle 4 and its real defects were fixed in one batch: P0 0, P1 0, P2 3, all fixed, P3 mostly fixed. Final local review: READY, with P0-P3 at 0.
* Copilot reviewed `bf389af4` with no new findings and the copilot-review gate returned SATISFIED. Merge was a normal merge commit; no admin fallback was used.
* The unfiltered `go test -timeout=30m -count=1 ./...` passed on the final code HEAD. One full-package run showed a load-induced flake of `TestU172_LockOrder_ArchiveItemDoesNotHoldArtifactLockWhileWaitingForItemLog`, which passed alone and on rerun; `internal/core` does not depend on the soundness package.

## Runtime verification and releasability

Runtime-verification is not applicable: the change is an offline CLI and library with no deployed runtime surface. Build and CLI behavior are covered by the full suite plus CI.

Releasability: `READY_WITH_CONDITIONS`.

* Condition: the linter is not wired into shipment assembly or CI (stash `09B597EE`).
* Validation window: run `backlogit soundness lint` in report mode against recent plans before enforcing.
* Rollback: leave the gate unwired, or revert merge `a61d409e` through a PR that preserves `.backlogit/` state.
* Owner: Ship / operator.

## Residual risk

* `internal/soundness` residuals are captured in stash `D399A888`: per-entry findings in the references rule, no panic recovery in `Run`, a unit that requires and forbids the same subject is not flagged, and grandparent relatedness.
* The `ship_shipment` MCP call timed out under load. The server completed the operation and it was verified by re-reading state, not by a retry.

## Governed closure

* Pre-closure baseline was clean apart from `.backlogit/hooks_queue.jsonl`, which is outside the predicted write set. The stale 153-S checkpoint was left alone.
* All 10 tasks were pre-archived. The explicit feature `160-F` was pending governed completion, and shipment `142-S` was `active`. `ship_shipment` with the merge sha archived `142-S` and `160-F`, and stamped the tasks with the merge commit and `archived` status.
* The staged set equalled the 14-path allowlist. Only the shipment, the feature and the ten tasks were touched, and no archive file was deleted (P-007).
* Provenance: no source stash or deliberation was archived by Ship.

## Follow-ups

Deferred stash entries, all requiring Stage triage: `09B597EE`, `F9C8E8C1`, `D399A888`.

Next shipment `143-S` is pending operator direction; it was not started.

## P-020 compaction

compact-context ran with `target: all`. The two 142-S checkpoints (the build checkpoint and the circuit-break notes) were compacted into `docs/memory/compacted/2026-10-16-142s-ship-compacted.md` and moved, not deleted, to `docs/archive/memory/2026-10-16/`. The circuit break is recorded as resolved there. Other referenced checkpoints stay. Outcome: `done`.

## Provenance

Compound refresh: `docs/closure/2026-10-16-142-s-compound-refresh.md`.
