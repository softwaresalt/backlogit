---
chunk_strategy: h1-h2-h3
closure_status: READY
compaction_status: done
description: "Post-merge closure for 169-S (baseline convergence RS-W12, U12 terminal boundary) and completion of feature 175-F."
doc_type: closure
docline:
  date: 2026-10-27T03:40:00Z
  status: final
  tags:
    - closure
    - 169-S
    - 175-F
schema_version: "1.0"
shipment_id: 169-S
source: docs/closure/169-S-175-F-post-merge-closure.md
title: "169-S closure"
---

# 169-S closure

Context: feature 175-F (baseline convergence), feature PR #589, merge `1832f1945efe30cb44769c290c78bab5e0d39f0d`. Member: 175.012-T (U12, terminal convergence verification, one wave). Change: the evidence artifact `docs/closure/175-baseline-convergence-convergence-evidence.md` (six gate markers and the 43-row NOLINT inventory), plus Stage's contract amendment `469ae8a7` and backlog state. No Go source changed.

## Releasability

`closure_status: READY`. There is no deployed service and no runtime surface changed. The release records that the supported-platform baseline (go test, go vet, gofmt, and golangci-lint on windows and linux) is green, which unblocks 149-S. Monitoring: CI `test` on the next PR to main. Rollback: revert PR #589. Owner: operator. Validation window: next PR to main.

## Runtime verification (validator evidence)

No runtime surface was touched; runtime verification is not applicable. The deliverable was verified by the task's own `exempt_verification_command` (exit 0, `EXEMPT_VERIFY_OK:175.012-T`) and `task_lint_cmd` in Schema, PreCommit and PostCommit phases (`TASK-LINT-OK:175.012-T:terminal-convergence`).

## Compaction status (P-020)

`compaction_status: done`. compact-context ran with `target: all`; top-level `docs/memory/` holds 40 files (about 116 KB), which does not exceed the 40-file / 500 KB thresholds, so there were no candidates.

## Feature 175-F

All 101 child tasks of 175-F are archived with `archived_status: done`. After the shipment closed, Ship moved 175-F queued to active to done and archived it (it was not a shipment member, so this was a separate explicit step). The archive record carries `status: done`, not `archived`; the archive tool did not rewrite it.

## Evidence

* Local review: adversarial review of `a0837abf` ran as a single reviewer without a shell (READY_WITH_FOLLOWUPS; P0 0, P1 0, P2 1, P3 1). Ship independently re-verified the P2 items (single-file commit, docs lint valid with 0 violations, clean worktree apart from governed claim state). The PR HEAD `89710ee033dea57f78edee46f53ffe20b20562bb` adds only `.backlogit/` state and a `docs/memory/` note on top of the reviewed commit.
* Copilot: P-018 gate SATISFIED when run after CI and again at the last mile; no open Copilot threads. CI all green (8 checks). Merge used the merge-commit strategy; no admin fallback.
* Tests: the exact `exempt_verification_command` exited 0 on the post-deliverable tree. An earlier pre-deliverable full run exited 2 on a single load-sensitive flake (`internal/evidencedag` heartbeat test), which passes in isolation; the pre-work red probe was completed with the go test step omitted and reached exit 7 as required.
* Transient: `backlogit_ship_shipment` timed out client-side and completed server-side; a CLI retry returned `shipment status conflict`, confirming completion. The archive record carries `archived_status: shipped` and the merge commit.
* Reconcile reports: `.backlogit/reconcile/169-S-pre-20261027T0335Z.md`, `169-S-safe-close-20261027T0338Z.md`, `169-S-post-20261027T0339Z.md`.
* Advisory: `backlogit doctor --check-shipped-event-completeness` reports pre-existing `missing_shipped_event` findings for older shipments; none concern 169-S.

## Source artifact provenance (untouched)

No source stash or deliberation was a shipment member; none mutated.

## Compound refresh

No entry in `docs/compound/` is superseded or invalidated by this shipment.

## Follow-ups

* Stash `03211AD7`: load-sensitive flake in `internal/evidencedag` `TestHardening_HolderOutlivingLockStaleKeepsItsLockViaHeartbeat`, plus the generic errcheck justification note.
* The full suite takes roughly 50 minutes serially; 149-S is now unblocked.
