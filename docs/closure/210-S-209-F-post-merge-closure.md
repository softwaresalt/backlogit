---
title: "210-S / 209-F Post-Merge Closure"
doc_type: closure
schema_version: "1.0"
source: docs/closure/210-S-209-F-post-merge-closure.md
description: "Post-merge closure for shipment 210-S and covering feature 209-F: governed safe-close, member provenance repair, and closure follow-ups after PR 638."
chunk_strategy: h1-h2-h3
shipment_id: 210-S
feature_id: 209-F
mode: post-merge
closure_status: BLOCKED
compaction_status: degraded
close_path: safe_close
---

# 210-S / 209-F Post-Merge Closure

## Release record

| Field | Value |
|---|---|
| Shipment | `210-S` |
| Covering feature | `209-F` |
| Tasks | `209.001-T`, `209.002-T`, `209.003-T` |
| Pull request | `#638` |
| Merge commit | `ac8975d86ed9eb12e2237a141a1437dea362003d` |
| Reviewed HEAD | `b8c2a3be6ca5eda14f346041d3d1fc240da01973` |
| Closure branch | `post-merge/210-s-closure` |

## Merge outcome

* PR `#638` merged as a merge commit at the pinned reviewed HEAD. No squash,
  rebase, or admin merge was used.
* The Copilot-review gate returned `SATISFIED: PASS` before the merge.
* `HEAD == origin/main` after the post-merge sync.

## Pre-close reconciliation

The pre-close reconciliation first returned `HALT - operator reconcile
required`. The four member archive records (`209-F`, `209.001-T`,
`209.002-T`, `209.003-T`) lacked `archived_from` and `archived_status` after
commit `a2eae8f7`.

The operator directed governed provenance repair. The governed
`backlogit_reconcile_archived_lifecycle` tool refused these items because it
only accepts items already at `status: archived`. The members were then
stamped with `backlogit_archive_item`, which for a pre-archived file records
`archived_from` (the canonical queue path) and `archived_status: done`.
`209-F` required one retry after a client-side timeout. The file was verified
untouched before the retry.

Post-repair pre-mode reconciliation returned `PROCEED`. The report is
`.backlogit/reconcile/210-S-pre-20261102-052600.md`.

## Governed closure

* `backlogit_ship_shipment` was called with merge SHA
  `ac8975d86ed9eb12e2237a141a1437dea362003d`. The client call timed out, but
  the item log shows the governed transaction completed: `shipment_status_changed`
  to `shipped`, `commit_tracked`, and `archived`.
* The retry returned `shipment status conflict`, which confirms the first call
  had already applied. Nothing was double-applied.
* `.backlogit/archive/210-S.md` carries `status: archived` and
  `archived_status: shipped`. The queue record is removed.
* Post-close doctor check: no findings for `210-S`. The historical advisory
  `missing_shipped_event` findings for older shipments predate this closure.
* Archive integrity (P-007): no archive deletions in the working tree.

## Validation evidence

* Pre-merge releasability and runtime evidence:
  `docs/closure/2026-11-01-210-s-209-f-pre-merge-closure.md` and
  `docs/closure/2026-11-01-210-s-209-f-runtime-verification.md`.
* Lifecycle `pipeline-topology` gate passed for `210-S` before any closure
  write (exit 0, `BRANCH_POST_MERGE_CLOSURE_ELIGIBLE`, `WORKTREE_TOPOLOGY_OK`).

## Deviations and residual risk

* **Compaction status is `degraded`, not `done`.** `compact-context` was
  triggered: `docs/memory/` holds 107 files and about 610 KB, against the 40-file
  and 500 KB thresholds. This pass assessed the corpus but did not execute the
  consolidation, so no originals were moved. Per P-020, the compaction run is
  recorded as degraded and a follow-up is stashed.
* **Closure status is `BLOCKED`.** The closure-evidence gate fails with
  `failed_check: close_evidence`. This closure used governed `safe_close`
  through MCP, so no cascade-close evidence exists. Per the closure-evidence
  contract, closure is not declared complete until the operator decides how to
  satisfy this requirement. The status is recorded as `BLOCKED` so that a
  successor `pre_claim` predicate does not accept an incomplete closure as
  `READY`. The record was not edited to pass the gate.

## Follow-ups

| ID | Summary |
|---|---|
| `7E14915F` | Residual-risk decision for 078-DL and 080-DL crash-window candidates |
| `DF533572` | Repo-wide gofmt follow-up (079-DL) |
| `D3C8D13B` | Deferred crash-window candidate (080-DL); summary not re-verified in this pass |
| `C2CFE06E` | Unfiltered local test gate |
| `F2BC9C37` | Carried follow-up; summary not re-verified in this pass |
