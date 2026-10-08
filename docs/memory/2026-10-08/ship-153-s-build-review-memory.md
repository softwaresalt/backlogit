---
title: "Ship 153-S build and review session memory"
description: "Ship session memory for 153-S (172-F) build waves, review cycles, and deferred scope captures"
ms.date: 2026-10-08
---

# Ship 153-S build and review memory

Written 2026-10-08T03:25Z (UTC). The session began on 2026-10-07 UTC, and earlier memory lives under `docs/memory/2026-10-07/`.

## Outcome

* Shipment 153-S (feature 172-F, tasks 172.001-T through 172.015-T): all 15 tasks are implemented across 7 dependency waves and tracked `done`.
* Harness discipline: the source-shape AST RED harness (172.014-T) landed first, then declarations (172.011-T, 172.013-T), then behavior harnesses, then GREEN implementation.
* Branch: `feat/153-s-concurrency-safety-hardening-for-artifact-mutation-writers`, base `0201e770`.

## Review cycles

* Cycle 1 (persona review): `BLOCKED`. I fixed findings A to J in `5feb4248`, `dafc93b2`, `1b17b672`, and `ce868001`.
* Cycle 2 (3-reviewer adversarial review): `READY_WITH_FOLLOWUPS`, findings F1 to F6.
  * In scope and fixed in `c5e3a1df` and `e6c963a2`:
    * F1: production-unreachability AST guard for the item-log barrier setter.
    * F2: lock-order test cleanup drains the archive goroutine.
    * F4: a CAS unarchive of a vanished item returns `ErrShipmentConflict`.
    * F6: corrected the hook-placement comment.
    * F3 and F5: documentation-scope corrections.
  * Out of scope (P-021 C1), captured as stash entries:
    * F3 code fix: `E4908F30`.
    * F5 ordering gap: `BACD94FC`.

## Deferred scope expansion stash entries

| ID | Kind | Priority | Summary |
|---|---|---|---|
| A0C92478 | bug | medium | `BulkUpdateConflict.Err` serializes to `{}` in JSON |
| E4908F30 | bug | high | ShipShipment holds global and B locks across item-log C during archival |
| BACD94FC | bug | medium | Deferred archive event causal ordering gap |
| 517BAC76 | feature | medium | Canonical C-before-B lock hierarchy redesign |
| DBF89C9C | bug | low | Post-archive hook and stash side effects are unlocked |
| 36D86223 | task | medium | Core test runtime is near the CI race timeout |
| 92AB6879 | bug | low | `TestClientLatestRespectsContextTimeout` flake |

A0C733C6 (findArtifact TOCTOU) predates this session and was reused, not duplicated.

## Decisions

* Archive events from the standalone and cascade `ArchiveItem` writers go to a deferred sink that is drained after B is released, so B is never held across C on those paths.
* Reconcile and unarchive writes use CAS on the expected status and surface `ErrShipmentConflict`.
* I did not change ShipShipment's governed path. It is recorded as a residual in `docs/design-docs/artifact-mutation-lock-order.md` and deferred as `E4908F30`.

## Next steps

Push the branch, then open the PR with the Local Review Readiness block. Then run the Copilot review loop, the copilot-review gate, a merge-commit merge, post-merge main sync, and the closure PR.

## Copilot round 2 (review 5451737450 on c89227e9)

* One body-only Medium finding (no review thread): Reconcile Step 12 re-archive returned nil when the item was deleted concurrently, because `lockArchiveGovernance` reload produced `ErrNotFound`, which Step 12 does not classify as a conflict.
* Fix: `ArchiveItem` translates not-found into `ErrShipmentConflict` (wrapping `ErrNotFound`) only when an expected-status CAS is set. Only Reconcile Step 12 and the archive CAS path set it, so plain archive callers keep `ErrNotFound`.
* RED-first test: `TestU172_ReconcileArchivedLifecycleReArchiveStepDetectsConcurrentDeletion`.

## Race watchdog flake

* Full suite failed once in `TestU172_Race_ReconcileArchivedLifecycleBatchAndSameItemWriterConverge`: the 10s watchdog fired, then the goroutines finished after cleanup (`sql: database is closed`), so the run was slow, not deadlocked. Neighbouring tests showed heavy load (7s workspace init).
* Isolated re-run passed 3/3 in 23s.
* Fix (P-021 C1, hardening this shipment's own tests): the three U172 race tests use `u172RaceDeadline = 45s`. A real lock-order deadlock never completes, so detection is preserved.

## Merge and post-merge closure (2026-10-08 UTC)

* Copilot round 3 raised 2 threads, fixed in `b7a06d53`, replied and
  resolved. The track-commit state landed as `1655bf2f`. Round 4 on
  `1655bf2f` returned 0 findings.
* `autoharness gate copilot-review 485` returned `SATISFIED`, and required CI
  was green. PR #485 merged by merge commit at 07:05:39Z as `271c916e`.
* Post-merge main sync: `POST_MERGE_SYNC_OK`. Closure branch:
  `post-merge/153-s-concurrency-safety-hardening`.
* Pre-reconcile returned `PROCEED`. The MCP `ship_shipment` call timed out on
  the client and was not retried; it completed server-side (shipped 07:14:45Z,
  archived 07:30:33Z). P-007 was clean, the post-reconcile report recommends
  `CLOSED`, and the archive state was committed as `c5d6bb06`.
* Closure artifact:
  `docs/closure/153-S-172-F-concurrency-safety-hardening-post-merge-closure.md`
  (closure `READY`, compaction `degraded`). compound-refresh classified all four
  candidates `keep`.
* Next: the closure PR loop, then merge, main re-sync, and STOP.