---
chunk_strategy: h1-h2-h3
decision_status: decided
depth: deep
description: 'Deliberation and P-021 C5/C6 provenance adjudication for CAP-3: commit-writer lock-order inversion fix, ShipShipment per-member closure optimization, pooled SQLite schema visibility under -race, bounded custodyGit subprocess execution and blob digest deduplication, and deterministic lock-order test hardening.'
doc_type: decision
linked_artifacts:
  - .backlogit/archive/096-DL.md
  - .backlogit/archive/097-DL.md
  - .backlogit/archive/098-DL.md
  - .backlogit/archive/099-DL.md
  - .backlogit/archive/100-DL.md
  - .backlogit/archive/101-DL.md
  - .backlogit/queue/216-F.md
  - .backlogit/queue/217-S.md
  - docs/plans/2026-11-15-cap3-zero-flake-concurrency-ship-closure-plan.md
promoted_to: both
schema_version: "1.0"
source: docs/decisions/2026-11-15-cap3-zero-flake-concurrency-ship-closure-deliberation.md
tags:
  - cap-3
  - concurrency
  - lock-ordering
  - ship-performance
  - custody
  - sqlite-wal
title: 'CAP-3 Deliberation: Zero-Flake Concurrency and Fast Ship-Closure Engine'
topic: 'CAP-3: Zero-Flake Concurrency & Fast Ship-Closure Engine (703835DC, 76553D8D, F448AA52, 9CFC88E5, B25BF131, 63909DAE)'
---

## Problem Frame

Full-suite `-race` execution and multi-member `ShipShipment` closure face six coupled concurrency, lock-ordering, subprocess-timeout, and per-member lookup performance issues across `internal/core`, `internal/db`, and `internal/events`:

1. **`703835DC` (`096-DL`, `bug`, `high`, `DEFERRED SCOPE EXPANSION`) — Commit-writer lock-order inversion**: Commit writers (`AssociateCommit`, `LinkCommit`, `track_commit` in `internal/core/commits.go`) must acquire `lifecycle-global` before item-log lock `C` when the target item is a shipment or shipment member and preserve `global -> C -> B` order through persistence and compensation. While `AssociateCommit` was partially updated in `fcd5e4ef`, `LinkCommit` (`internal/core/commits.go:258-296`) still executes `INSERT OR REPLACE INTO commit_links` unlocked and acquires `C` (`LockItemLogCrossProcess`) without checking preheld `C` or acquiring `lifecycle-global`, and no forced-interleaving regression test using a real shipment member exists for the commit writers.
2. **`76553D8D` (`097-DL`, `task`, `high`) — `ShipShipment` per-member snapshot, lookup, ancestry, and `ReadAllEvents` buffer cost**: Closing a multi-member shipment (`ShipShipment` in `internal/core/shipment_lifecycle.go`, `internal/core/shipment_gate.go`, `internal/events/reader.go`) incurs high per-member overhead because:
   * `events.ReadAllEvents` (`internal/events/reader.go:40`) allocates and memclrs a `1 MiB` buffer (`make([]byte, 1<<20)`) on every call even for small event logs;
   * `snapshotShipArtifacts` (`internal/core/shipment_lifecycle.go:237-249`) calls `findArtifact(ctx, ws, id)` AND `FindArtifactPath(ctx, ws, id)` back-to-back for every member, and both `findArtifact` and `FindArtifactPath` (`internal/core/artifacts.go:778,1126`) perform O(N) `filepath.WalkDir` + full YAML frontmatter parses across `queue/` and `archive/` instead of checking the direct contained `{id}.md` path first;
   * `validateMemberGateEvidence` (`internal/core/shipment_gate.go`) spawns `git merge-base --is-ancestor` subprocesses (`isAncestor`) repeatedly for identical immutable `(memberHead, shipmentHead)` SHA pairs across members and across the two gate passes.
3. **`F448AA52` (`098-DL`, `bug`, `high`, `DEFERRED SCOPE EXPANSION`) — `internal/core` `-race` pooled-connection WAL/schema initialization and timeout issues**:
   * `(a)` `TestU2_ReconstituteShipmentsAtomicExecutionAndRollback/concurrent_associate_commit_and_post_hook_isolation` intermittently fails under `-race` with `no such table: commit_links` because (1) `db.Open` (`internal/db/connection.go:58-62`) registers `_pragma=journal_mode(WAL)` BEFORE `_pragma=busy_timeout(30000)` on pooled connections (`SetMaxOpenConns(4)`), and (2) when `db.Open` creates a brand-new 0-byte `backlogit.db`, `PRAGMA journal_mode(WAL)` sets WAL mode in memory on Connection #1 without writing Page 1 (bytes 18–19 = `0x02, 0x02`) to `backlogit.db` on disk; when a second pooled connection opens that 0-byte `backlogit.db` while Connection #1 holds an active lock, SQLite's `PRAGMA journal_mode(WAL)` returns `"delete"` with `SQLITE_OK` (rather than `SQLITE_BUSY`), leaving Connection #2 in `delete` mode (or with an empty cached `pSchema` from the 0-byte main file) so its query against `commit_links` fails with `no such table: commit_links`;
   * `(b)` `TestReconcileResumeForgedPreparedEvent` (`internal/core/shipment_reconcile_resume_auth_test.go`) rebuilds `newAuthSeamFixture` (artifacts, shipments, git seed copies, Ed25519 keys) from scratch on every subtest (50+ times across the reconcile auth tests), and `TestUR1B_FailClosedValidationAndAuthoritativeActiveSlot` (`internal/core/shipment_blocked_lifecycle_harness_test.go:579-612`) launches a child test binary via `exec.Command` without a context timeout or `WaitDelay` and uses a tight 10s readiness poll while the child runs full workspace recovery.
4. **`9CFC88E5` (`099-DL`, `bug`, `high`, `DEFERRED SCOPE EXPANSION`) — Custody Git-subprocess timeout in `TestExternal_LiveAcceptAndVerifyUseExternalStore`**: `custodyGit` (`internal/core/custody_exec.go:1066`) executes `git` with the caller's unbounded `ctx` (`context.Background()`), no `cmd.WaitDelay`, and no `core.fsmonitor=false` / `gc.auto=0` / `GIT_TERMINAL_PROMPT=0` guards while holding `global -> A -> C -> B` custody locks, and `custodyLiveDigests` / `custodyBlobDigest` spawns duplicate `git cat-file blob <oid>` subprocesses for identical immutable blob OIDs.
5. **`B25BF131` (`100-DL`, `task`, `high`) — Harden `TestU4_CustodyPreviewOfferAcceptVerifyAndRecovery/profile_b_stage_to_ship_handoff_and_stale_digest_invalidation` under concurrent `-race` load**: `buildCustExecSeed` (`internal/core/custody_exec_test.go:384`) does not disable `core.fsmonitor` or `gc.auto` in the seeded git repository config, and `profile_b_stage_to_ship_handoff_and_stale_digest_invalidation` lacks per-step diagnostic timing/verdict capture and bounded subtest context timeouts.
6. **`63909DAE` (`101-DL`, `bug`, `high`) — Full-suite concurrency contention in `TestU172_LockOrder_ArchiveItemDoesNotHoldArtifactLockWhileWaitingForItemLog` and `TestArchiveItemGovernance_WaitsForGlobalLifecycleLock` (plus unresolved unnamed `df4a2df9` panic risk)**: Both named tests rely on fixed sleeps/windows (`150ms`/`250ms`) and tight `defaultGateLockBoundedWait` (`3s`/`5s`) writer and post-unlock completion budgets without synchronizing on `ArchiveItem` reaching the exact contended lock boundary (`lockItemLogsSorted` in `TestU172`, `lockArchiveShipmentMemberships` in `TestArchiveItemGovernance`). Separately, the unnamed `internal/core` panic recorded at `df4a2df9` (`docs/closure/197-S-197-F-post-merge-closure.md:132-137`, where neither test name nor stack trace was captured) has an unproven origin and must remain explicitly unresolved until the test and stack trace are captured and a relevant remediation is demonstrated.

## P-021 C5 and C6 Deferred-Expansion Triage and Reconciliation Audit

All three `DEFERRED SCOPE EXPANSION` entries in CAP-3 were audited under Stage's P-021 C5/C6 intake obligations before entering grouping and planning:

| Stash ID | Deliberation | Originating Source Refs | (A) Unconditional Duplicate Scan Outcome | (B) Mandatory Late-Identifier Reconciliation Outcome |
| --- | --- | --- | --- | --- |
| `703835DC` | `096-DL` | `task_id=200.005-T/200.007-T` (late-reconciled), `feature_id=200-F`, `shipment_id=200-S`, `PR=540`, `review_thread_id=PRRT_kwDORzozKM6tyLh3` (`internal/core/archive.go:459`) | **CLEAN DUPLICATE SCAN** across all 349 records in `.backlogit/stash.jsonl` (`0` duplicates) | **Reconciled originating task IDs `200.005-T`/`200.007-T`** from `docs/closure/200-S-200-F-post-merge-closure.md` (`PR=540` and `review_thread_id=PRRT_kwDORzozKM6tyLh3` already present) and updated `703835DC` in place |
| `F448AA52` | `098-DL` | `task_id=210.003-T`, `feature_id=210-F`, `shipment_id=211-S`, `PR=N/A` (pre-PR; `211-S` shipped via PR `#641`), `review_thread_id=N/A` (threadless pre-PR full-suite finding) | **CLEAN DUPLICATE SCAN** across all 349 records in `.backlogit/stash.jsonl` (`0` duplicates) | **No late review-thread identifier found** (threadless pre-PR full-suite finding; shipment `211-S` PR `#641` recorded in place) |
| `9CFC88E5` | `099-DL` | `task_id=213.004-T` (`U11a`), `feature_id=213-F`, `shipment_id=214-S`, `PR=N/A` (pre-PR; `214-S` shipped via PR `#649`), `review_thread_id=N/A` (threadless Wave 1 convergence timeout; `docs/memory/2026-11-13-214-s-four-wave-resume.md`) | **CLEAN DUPLICATE SCAN** across all 349 records in `.backlogit/stash.jsonl`; candidate `5A81065E` (manual CLI/MCP runtime probe for Profile A accept) verified distinct | **No late review-thread identifier found** (threadless Wave 1 convergence timeout; shipment `214-S` PR `#649` recorded in place) |

## Research Findings

### Prior Institutional Learnings (`docs/compound/`)

* `docs/compound/2026-10-19-lock-held-side-effect-ordering.md`: Explicitly documents `703835DC` and requires canonical lock ordering `lifecycle-global -> A -> C -> B` across all writers touching shipment members.
* `docs/compound/2026-10-06-ship-shipment-mcp-timeout-result-unknown.md` and `docs/compound/2026-07-28-attach-commit-repersist-must-reload-from-markdown.md`: `ShipShipment` must keep both gate passes and must reload from Markdown (never SQLite `loadArtifact`) in `attachCommitToItems` so `item_links` and archive provenance are preserved.
* `docs/compound/best-practices/red-snapshot-observation-scoped-metadata-memo-2026-11-08.md`: Observation-scoped or content-addressed memoization eliminates repeated subprocess/filesystem probes without weakening freshness guarantees.
* `docs/compound/2026-07-06-bounded-helper-timeout-hard-cap.md` and `docs/compound/test-failures/separate-compile-budget-and-own-subprocess-tree-2026-11-08.md`: Local Git subprocesses invoked under workspace locks must enforce a self-derived timeout and `WaitDelay` so inherited Windows pipe handles cannot hang `os/exec.Cmd.Wait`.
* `docs/compound/2026-10-11-unfiltered-suite-evidence-under-load.md`: Lock-order and governance-wait tests (`63909DAE`) must synchronize on deterministic contention signals and use generous post-release observation timeouts (`30s`) rather than tight sleep/deadline races.
* `docs/memory/2026-11-10/stage-212-s-089-dl-ordinal17-u1k-custody-wal-fix.md`: `custExecNewRepo` (`internal/core/custody_exec_test.go`) MUST retain `PRAGMA synchronous = FULL` so Windows NTFS flushes `backlogit.db-wal` `ModTime` before `custExecSnapshot`.

## Options Evaluated

### Option A: Test-Only Timeout Bumps and Retries

* **Description**: Increase test timeouts and retry flaky tests without fixing `LinkCommit` lock order, `db.Open` pragma order, `custodyGit` unbounded waits, or `ShipShipment` O(M * N) walks.
* **Pros**: Minimal production code changes.
* **Cons**: Leaves `703835DC` lock-order inversion, `76553D8D` `ShipShipment` latency, `9CFC88E5` unbounded `custodyGit` hangs, and `F448AA52` pooled-connection `no such table: commit_links` defect unresolved.
* **Effort**: Low | **Fit**: Poor.

### Option B: Surgical Root-Cause Remediation Across DB, Events, Core, and Concurrency Harnesses (Recommended)

* **Description**:
  1. **`703835DC` (`internal/core/commits.go`, `internal/core/reconstitution_review_regression_test.go`)**: Enforce `lifecycle-global -> C -> B` in `LinkCommit` (matching `AssociateCommit`), including preheld-`C` lifecycle ownership validation (`validateShipmentLifecycleGlobalReentry`, returning `*ShipmentLifecycleLockOrderError`) and acquiring `lockShipmentLifecycleGlobal` before `commit_links` SQLite writes and `LockItemLogCrossProcess`; add forced-interleaving shipment-member regression tests in `package core` (`internal/core/reconstitution_review_regression_test.go`) for both `AssociateCommit` and `LinkCommit` across persistence and compensation.
  2. **`76553D8D` (`internal/events/reader.go`, `internal/core/artifacts.go`, `internal/core/shipment_lifecycle.go`, `internal/core/shipment_gate.go`, `internal/core/workspace.go`)**:
     * Change `ReadAllEvents` scanner buffer from eager `make([]byte, 1<<20)` to `make([]byte, 64*1024)` with `1<<20` max;
     * Add `findArtifactWithPath` with a two-pass search across `artifactSearchDirs(ws)` (Pass 1: probe direct `{id}.md` with `ensureArtifactLookupContained` + `parseFile` across all `searchDirs`; Pass 2: `filepath.WalkDir` fallback across all `searchDirs`) and use it in `snapshotShipArtifacts` to eliminate duplicate per-member `findArtifact` + `FindArtifactPath` scans and prevent `queue/` lookups from walking `archive/`;
     * Pass `--no-replace-objects` to `git merge-base --is-ancestor` and memoize verified 40/64-hex `(ancestorSHA, descendantSHA)` commit ancestry results in a bounded `Workspace`-scoped cache in `isAncestor` (checking `ctx.Err()` first and restricting caching strictly to complete, ungrafted history where `.git/shallow` and `.git/info/grafts` are both absent, clearing/bypassing on graph boundary changes) across members and across the two gate passes while preserving both gate passes, verdict timing, error classification, and lock order.
  3. **`F448AA52` (`internal/db/connection.go`, `internal/db/schema.go`, `internal/core/shipment_reconcile_auth_seam_test.go`, `internal/core/shipment_blocked_lifecycle_harness_test.go`)**:
     * Place `_pragma=busy_timeout(30000)` first in `db.Open` DSN pragmas AND separately materialize the persistent Page 1 WAL header (`hdr[18]==2, hdr[19]==2`) on newly created 0-byte `.db` files in `db.Open` and verify WAL mode and `commit_links` schema readiness in `EnsureSchema` / `EnsureSchemaWithExtensions` (`internal/db/schema.go`), with a concrete concurrent pooled-connection reproducer in `internal/db/connection_test.go`;
     * Seed `newAuthSeamFixture` once via `sync.Once` (`authSeamSeed`, checkpointing and closing the seed DB before cloning and setting `ws.Config.Reconcile = &config.ReconcileConfig{}` on the clone);
     * Bound `TestUR1B_FailClosedValidationAndAuthoritativeActiveSlot` child subprocess with `exec.CommandContext` (30s timeout + `cmd.WaitDelay`) and 30s readiness barrier.
  4. **`9CFC88E5` & `B25BF131` (`internal/core/custody_exec.go`, `internal/core/custody_exec_test.go`)**:
     * Add self-derived `custodyGitTimeout` (default `15s`, injectable for tests), `cmd.WaitDelay = 2 * time.Second`, `-c core.fsmonitor=false -c gc.auto=0`, and `GIT_TERMINAL_PROMPT=0` to `custodyGit`, and deduplicate `custodyBlobDigest` (`git cat-file blob <oid>`) across identical immutable blob OIDs (and when `indexOID == baseOID` in `custodyLiveDigests`);
     * Configure `core.fsmonitor=false` and `gc.auto=0` in `buildCustExecSeed` (while preserving `PRAGMA synchronous = FULL` in `custExecNewRepo`) and add diagnostic step-timing/verdict messages and bounded contexts in `TestU4_CustodyPreviewOfferAcceptVerifyAndRecovery/profile_b_stage_to_ship_handoff_and_stale_digest_invalidation`.
  5. **`63909DAE` (`internal/core/archive.go`, `internal/core/artifact_mutation_lock_order_test.go`, `internal/core/shipment_archive_governance_test.go`)**:
     * Synchronize `TestU172_LockOrder_ArchiveItemDoesNotHoldArtifactLockWhileWaitingForItemLog` on `ArchiveItem` reaching `lockItemLogsSorted` (after recovery and shipment discovery) before running the negative `B` probe and `runU172CBeforeBWriter` (with `u172ObservationWait = 30s`);
     * Synchronize `TestArchiveItemGovernance_WaitsForGlobalLifecycleLock` and `TestArchiveItemGovernance_WaitsForShipmentMembershipLock` on deterministic lock-attempt hooks with a short `50ms` post-contention hold and `30s` post-unlock completion budget.
* **Pros**: Eliminates every root cause across all six CAP-3 items with zero weakened assertions or skips.
* **Cons**: Spans `internal/db`, `internal/events`, and `internal/core` (decomposed into width-isolated tasks).
* **Effort**: Medium | **Fit**: Excellent.

## Trade-off Comparison

| Criterion | Option A | Option B (Selected) |
| --- | --- | --- |
| Fixes commit-writer lock-order inversion (`703835DC`) | No | Yes (`AssociateCommit` + `LinkCommit` + forced-interleaving test) |
| Optimizes `ShipShipment` without altering gate passes or lock order (`76553D8D`) | No | Yes (64 KiB->1 MiB buffer, `findArtifactWithPath`, ancestry memo) |
| Eliminates `no such table: commit_links` & 12m hangs under `-race` (`F448AA52`) | No | Yes (DSN pragma order + persistent WAL header & pooled schema init + seeded fixtures + bounded child) |
| Prevents `custodyGit` `Cmd.Wait` hang & `-race` flake (`9CFC88E5`, `B25BF131`) | No | Yes (bounded `custodyGit` + `WaitDelay` + OID digest memo + diagnostics) |
| Hardens `TestU172` and `TestArchiveItemGovernance` under load (`63909DAE`) | No | Yes (deterministic contention hooks + 30s post-unlock waits) |

## Decision

Adopt **Option B** and promote directly to the CAP-3 implementation plan (`docs/plans/2026-11-15-cap3-zero-flake-concurrency-ship-closure-plan.md`) and covering feature `216-F` / shipment `217-S` (sequenced with `217-S` blocked by `216-S`).

## Rejected Alternatives

* **Option A** is rejected because timeout bumps do not fix lock-order inversions, unbounded subprocess waits, or pooled-connection WAL schema visibility races.
* **Setting `PRAGMA synchronous = OFF` in `custExecNewRepo`** is rejected because it caused the Ordinal 16 Windows NTFS `backlogit.db-wal` `ModTime` regression (`stage-212-s-089-dl-ordinal17-u1k-custody-wal-fix.md`).
* **Replacing `findArtifact` with `loadArtifact` in `attachCommitToItems`** is rejected because SQLite projections omit `item_links` and `archived_from`/`archived_status` provenance (`docs/compound/2026-07-28-attach-commit-repersist-must-reload-from-markdown.md`).

## Unresolved Questions

* **Unnamed `df4a2df9` `internal/core` panic (`63909DAE` / `docs/closure/197-S-197-F-post-merge-closure.md:132-137`)**: Explicitly **UNRESOLVED**. Because neither the panicking test name nor the goroutine stack trace was captured at `df4a2df9` (and quiet unfiltered runs at `de1f8adf` and `b0f611ea` exited `0`), its origin is unproven and nothing connects it to `custodyGit` (`9CFC88E5`) or the `TestUR1B` subprocess (`F448AA52`). It remains an open, tracked diagnostic risk until a run captures the test name and stack trace and a relevant remediation is demonstrated. All other CAP-3 items have verified root causes and exact file/symbol boundaries.

## Risks and Mitigations

* **Risk**: `findArtifactWithPath` direct `{id}.md` probe bypassing symlink/reparse containment checks.
  * **Mitigation**: Always run `ensureArtifactLookupContained(ws, candidate)` on the direct `{id}.md` candidate BEFORE calling `parseFile(candidate)` and verify `a.ID == id`, preserving `TestSE7bLookupTimeContainmentHarness` byte-for-byte.
* **Risk**: `custodyBlobDigest` memoization returning stale digests if a working-tree file is edited.
  * **Mitigation**: Memoize ONLY `custodyBlobDigest(ctx, root, oid)` keyed by immutable content-addressed Git blob OID (`40`- or `64`-hex SHA), never by file path; working-tree files (`custodyWorktreeDigest`) and `git ls-tree` / `git ls-files` OID lookups still execute freshly on every call.
