---
chunk_strategy: h1-h2-h3
description: 'Implementation plan for CAP-3 (216-F / 217-S): commit-writer lock-order inversion fix, ShipShipment per-member closure optimization, pooled SQLite schema visibility under -race, bounded custodyGit subprocess execution and blob digest deduplication, and deterministic lock-order test hardening.'
doc_type: plan
schema_version: "1.0"
source: docs/plans/2026-11-15-cap3-zero-flake-concurrency-ship-closure-plan.md
title: 'CAP-3 Plan: Zero-Flake Concurrency and Fast Ship-Closure Engine'
---

## Problem Frame

Full-suite `-race` convergence and multi-member `ShipShipment` closure require six targeted concurrency, lock-ordering, subprocess-timeout, and lookup-performance fixes resolved in [the CAP-3 deliberation](../decisions/2026-11-15-cap3-zero-flake-concurrency-ship-closure-deliberation.md) (`096-DL` through `101-DL`):

1. **`703835DC` (`096-DL`)**: Commit writers (`AssociateCommit`, `LinkCommit`, `track_commit` in `internal/core/commits.go`) must enforce canonical lock order `lifecycle-global -> C (item-log) -> B (artifact mutation)` when the target item is a shipment or shipment member and preserve that order through persistence and compensation. Currently, `LinkCommit` (`internal/core/commits.go:258-296`) writes `commit_links` unlocked and acquires `C` (`LockItemLogCrossProcess`) without checking preheld `C` or acquiring `lifecycle-global`, and no forced-interleaving regression test using a real shipment member exists for the commit writers.
2. **`76553D8D` (`097-DL`)**: `ShipShipment` (`internal/core/shipment_lifecycle.go`, `internal/core/shipment_gate.go`, `internal/events/reader.go`) incurs excessive per-member latency because: (a) `events.ReadAllEvents` (`internal/events/reader.go:40`) eagerly allocates and zeroes a `1 MiB` buffer (`make([]byte, 1<<20)`) on every call; (b) `snapshotShipArtifacts` (`internal/core/shipment_lifecycle.go:237-249`) runs two separate O(N) `filepath.WalkDir` + YAML-parse scans per member (`findArtifact` then `FindArtifactPath`), and `findArtifact`/`FindArtifactPath` (`internal/core/artifacts.go:778,1126`) walk the entire directory instead of probing the contained `{id}.md` candidate path first; and (c) `validateMemberGateEvidence` (`internal/core/shipment_gate.go`) spawns `git merge-base --is-ancestor` subprocesses (`isAncestor`) repeatedly for identical immutable `(memberHead, shipmentHead)` SHA pairs across members and across the two gate passes.
3. **`F448AA52` (`098-DL`)**: Under concurrent `-race` package runs, (a) `TestU2_ReconstituteShipmentsAtomicExecutionAndRollback/concurrent_associate_commit_and_post_hook_isolation` intermittently fails with `no such table: commit_links` because `db.Open` (`internal/db/connection.go:58-62`) sets `_pragma=journal_mode(WAL)` before `_pragma=busy_timeout(30000)` on newly opened pooled connections (`SetMaxOpenConns(4)`) AND leaves a newly created `backlogit.db` at 0 bytes on disk without a persistent Page 1 WAL header (`hdr[18]==2, hdr[19]==2`), so a contended `PRAGMA journal_mode(WAL)` on a second pooled connection returns `"delete"` with `SQLITE_OK` and leaves the connection in `delete` mode reading the 0-byte main file; and (b) `TestReconcileResumeForgedPreparedEvent` (`internal/core/shipment_reconcile_resume_auth_test.go`) rebuilds `newAuthSeamFixture` from scratch on every subtest (50+ times) while `TestUR1B_FailClosedValidationAndAuthoritativeActiveSlot` (`internal/core/shipment_blocked_lifecycle_harness_test.go:579-612`) runs an unbounded `exec.Command` child subprocess with a tight 10s readiness poll.
4. **`9CFC88E5` (`099-DL`)**: `TestExternal_LiveAcceptAndVerifyUseExternalStore` timed out at `30m0s` waiting in `os/exec.Cmd.Wait` via `custodyGit` / `custodyBlobDigest` (`internal/core/custody_exec.go:1066,1115`) because `custodyGit` uses the caller's unbounded `ctx` (`context.Background()`) without a self-derived timeout, `cmd.WaitDelay`, or `core.fsmonitor=false`/`gc.auto=0`/`GIT_TERMINAL_PROMPT=0` guards while holding `global -> A -> C -> B` custody locks, and `custodyLiveDigests` / `custodyBlobDigest` spawns duplicate `git cat-file blob <oid>` subprocesses for identical immutable blob OIDs.
5. **`B25BF131` (`100-DL`)**: `TestU4_CustodyPreviewOfferAcceptVerifyAndRecovery/profile_b_stage_to_ship_handoff_and_stale_digest_invalidation` (`internal/core/custody_exec_test.go:110-169`) lacks `core.fsmonitor=false` / `gc.auto=0` in `buildCustExecSeed`, bounded per-subtest contexts, and diagnostic step-timing/verdict failure messages under concurrent `-race` load.
6. **`63909DAE` (`101-DL`)**: `TestU172_LockOrder_ArchiveItemDoesNotHoldArtifactLockWhileWaitingForItemLog` (`internal/core/artifact_mutation_lock_order_test.go`) and `TestArchiveItemGovernance_WaitsForGlobalLifecycleLock` (`internal/core/shipment_archive_governance_test.go`) fail intermittently under heavy concurrent load because they rely on fixed sleeps and tight `3s`/`5s` (`defaultGateLockBoundedWait`) writer and post-unlock completion budgets without synchronizing on `ArchiveItem` reaching the exact contended lock boundary.

## Requirements Trace

| Requirement | Stash / Deliberation | Implementation Unit | Verification Target |
| --- | --- | --- | --- |
| R1: Enforce `lifecycle-global -> C -> B` in commit writers (`AssociateCommit`, `LinkCommit`, `track_commit`) across persistence and compensation with shipment-member forced-interleaving test | `703835DC` / `096-DL` | Unit U1 | `TestU1_CommitWritersShipmentMemberLockOrderGlobalBeforeItemLog` |
| R2: Reduce `events.ReadAllEvents` initial scanner buffer allocation from 1 MiB to 64 KiB (growing up to 1 MiB max) | `76553D8D` / `097-DL` | Unit U2 | `TestU2_ReadAllEventsSmallInitialBufferGrowsToOneMiBMax` |
| R3: Eliminate redundant per-member directory walks in `findArtifact`, `FindArtifactPath`, and `snapshotShipArtifacts` via contained direct `{id}.md` probe and `findArtifactWithPath` | `76553D8D` / `097-DL` | Unit U3a | `TestU3a_SnapshotShipArtifactsSinglePassContainedDirectLookup` |
| R4: Memoize replacement-free `--no-replace-objects` `(ancestorSHA, descendantSHA)` commit ancestry checks in `isAncestor` restricted to complete, ungrafted history across both `ShipShipment` gate passes | `76553D8D` / `097-DL` | Unit U3b | `TestU3b_ValidateMemberGateEvidenceMemoizesImmutableCommitAncestry` |
| R5: Order `_pragma=busy_timeout(30000)` first in `db.Open` and enforce persistent Page 1 WAL header & pooled schema initialization across `db.Open` and `EnsureSchema` | `F448AA52` / `098-DL` | Unit U4 | `TestU4_OpenPragmaOrderAndPooledSchemaInitialization` |
| R6: Bound `custodyGit` with self-derived timeout, `WaitDelay`, and `fsmonitor=false`/`gc.auto=0`, deduplicate `custodyBlobDigest` by immutable OID, and harden `TestU4` `profile_b` diagnostics | `9CFC88E5`, `B25BF131` / `099-DL`, `100-DL` | Unit U5 | `TestU5_CustodyGitBoundedTimeoutAndBlobDigestDeduplication` |
| R7: Seed `newAuthSeamFixture` via `sync.Once` and bound `TestUR1B` child subprocess execution and readiness barrier | `F448AA52` / `098-DL` | Unit U6 | `TestReconcileResumeForgedPreparedEvent`, `TestUR1B_FailClosedValidationAndAuthoritativeActiveSlot` |
| R8: Synchronize `TestU172_LockOrder` and `TestArchiveItemGovernance` on deterministic lock-contention hooks with 30s post-unlock completion budgets | `63909DAE` / `101-DL` | Unit U7 | `TestU172_LockOrder_ArchiveItemDoesNotHoldArtifactLockWhileWaitingForItemLog`, `TestArchiveItemGovernance_WaitsForGlobalLifecycleLock` |

## Implementation Units

### Unit U1: Fix commit-writer lock-order inversion across persistence and compensation (`703835DC`)

* **Domain**: `code` (width-isolated to `internal/core`)
* **Size / Complexity**: `M` / `medium` (2 files, <= 3 functions, <= 3 test scenarios)
* **Execution posture**: `test-first` (P-002.1 compile PASS + runtime assertion RED before production edit)
* **Affected files (2)**:
  * `internal/core/commits.go`
  * `internal/core/reconstitution_review_regression_test.go`
* **Changes needed**:
  1. In `internal/core/commits.go` (`LinkCommit`, lines 258-296): when `ws != nil` and `ws.RootPath != ""`, apply the same canonical `lifecycle-global -> C (item-log)` lock-order guard and acquisition as `AssociateCommit` BEFORE executing `db.ExecContext(ctx, "INSERT OR REPLACE INTO commit_links ...")` and BEFORE acquiring `events.LockItemLogCrossProcess`:
     * If `events.ItemLogCrossProcessLockHeld(ctx, logsDir, itemID)` is already true, verify preheld lifecycle ownership via `validateShipmentLifecycleGlobalReentry(ctx, ws, "LinkCommit")` and the workspace-bound `shipmentLifecycleHeldLocksContextKey{}` token; if not held and `itemID` is a shipment or shipment member (`artifactIsShipmentLifecycleMember(ws, itemID)`), fail closed with `shipmentLifecycleLockOrderError(ctx, "LinkCommit", ...)` (returning `*ShipmentLifecycleLockOrderError`).
     * Otherwise, acquire `lockShipmentLifecycleGlobal(ctx, ws)` before touching `commit_links` or acquiring `C` (`LockItemLogCrossProcess`), and hold `global` through `AppendEvent` and `IndexEvent`.
  2. In `internal/core/commits.go` (`AssociateCommit`, lines 56-250): preserve `lockShipmentLifecycleGlobal` acquisition before `findArtifact`, `commit_links` pre-snapshot, and `LockItemLogCrossProcess` (`C`), ensuring `global -> C -> B` ordering holds across both `Apply` and `Compensate` steps of `MutationEnvelope`.
* **RED-first acceptance criteria**:
  * **RED harness (`internal/core/reconstitution_review_regression_test.go`, `package core`)**: Add `TestU1_CommitWritersShipmentMemberLockOrderGlobalBeforeItemLog` using a real shipment member task to verify: (a) calling `LinkCommit` or `AssociateCommit` when `C` (`LockItemLogCrossProcess`) is preheld without `lifecycle-global` fails closed with `*ShipmentLifecycleLockOrderError` (`var orderErr *ShipmentLifecycleLockOrderError; require.ErrorAs(t, err, &orderErr)`) without mutating `commit_links` (failing deterministically today because `LinkCommit` succeeds and writes `commit_links` under preheld `C` without `global`); (b) in a forced interleaving where a competing goroutine holds `lifecycle-global`, both `AssociateCommit` and `LinkCommit` wait on `lifecycle-global` BEFORE acquiring `C` (leaving `C` free for the `lifecycle-global` holder to acquire without a `C`-then-`global` inversion); and (c) when `AssociateCommit` fails at `commit-links-upsert` or `jsonl-append` on a shipment member, compensation restores frontmatter and `commit_links` while still holding `global -> C`.
  * **Scoped verification command**: `go test -timeout=30m -count=1 -race -v -run '^(TestU1_CommitWriters|TestReviewAssociateCommit|TestAssociateCommit_|TestLinkCommit_)' ./internal/core`

### Unit U2: Optimize `events.ReadAllEvents` scanner buffer allocation (`76553D8D` Part 1)

* **Domain**: `code` (width-isolated to `internal/events`)
* **Size / Complexity**: `S` / `low` (2 files, <= 2 functions, <= 3 test scenarios)
* **Execution posture**: `test-first`
* **Affected files (2)**:
  * `internal/events/reader.go`
  * `internal/events/reader_test.go`
* **Changes needed**:
  1. In `internal/events/reader.go` (`ReadAllEvents`, line 40), change `scanner.Buffer(make([]byte, 1<<20), 1<<20)` to `scanner.Buffer(make([]byte, 64*1024), 1<<20)` (64 KiB initial buffer, growing on demand up to the 1 MiB maximum token size, preserving `TestU175_028_LintFix` close-error handling).
* **RED-first acceptance criteria**:
  * **RED harness (`internal/events/reader_test.go`)**: Add `TestU2_ReadAllEventsSmallInitialBufferGrowsToOneMiBMax` verifying: (a) after a warmup call and `runtime.GC()`, the minimum deterministic heap allocation (`runtime.ReadMemStats` `TotalAlloc` delta across 3 runs) for `ReadAllEvents` on a small 2-line event log is `< 256 * 1024` bytes (failing deterministically against the current `1,048,576`-byte eager allocation); (b) an event line larger than 64 KiB (e.g., 256 KiB payload) grows the scanner buffer on demand and parses cleanly; and (c) an oversized line (`1<<20 + 1` bytes) still fails closed with `bufio.ErrTooLong`.
  * **Scoped verification command**: `go test -timeout=30m -count=1 -race -v -run '^(TestU2_ReadAllEvents|TestReadAllEvents|TestTailEvents|TestU175_028_LintFix)' ./internal/events`

### Unit U3a: Optimize `findArtifact`, `FindArtifactPath`, and `snapshotShipArtifacts` with two-pass contained direct `{id}.md` lookup (`76553D8D` Part 2a)

* **Domain**: `code` (width-isolated to `internal/core`)
* **Size / Complexity**: `M` / `medium` (3 files, <= 4 functions, <= 3 test scenarios)
* **Execution posture**: `test-first`
* **Dependencies**: `blocks` on Unit U1 (serializes `internal/core` edits)
* **Affected files (3)**:
  * `internal/core/artifacts.go`
  * `internal/core/shipment_lifecycle.go`
  * `internal/core/shipment_gate_bench_test.go`
* **Changes needed**:
  1. In `internal/core/artifacts.go`, add `findArtifactWithPath(ctx context.Context, ws *Workspace, id string) (*models.Artifact, string, error)` and update `findArtifact`, `FindArtifactPath`, and `findArtifactInSearchDir`:
     * Use a **two-pass search across `artifactSearchDirs(ws)`** (since `DefaultRegistry` orders `archive/` before `queue/` in `searchDirs`):
       * **Pass 1 (Direct Contained Probe across all `searchDirs`)**: When `id != "" && id != "." && id != ".." && filepath.Base(id) == id && filepath.VolumeName(id) == "" && !strings.ContainsAny(id, `/\`)`, iterate over each `dirPath` in `searchDirs` and probe `candidate := filepath.Join(dirPath, id+".md")` via `os.Lstat(candidate)`. When present and not a directory, run `ensureArtifactLookupContained(ws, candidate)` FIRST (returning any containment violation error immediately so `TestSE7bLookupTimeContainmentHarness` is preserved), then call `parseFile(candidate)` and return `(a, candidate, nil)` immediately when `parseErr == nil && a.ID == id`.
       * **Pass 2 (`filepath.WalkDir` fallback across `searchDirs`)**: Only if Pass 1 did not find `id` in any `dirPath`, iterate over `searchDirs` and run `filepath.WalkDir(dirPath, ...)` to locate slug-named or nested files. Retain the single-directory probe-then-walk behavior inside `findArtifactInSearchDir(ws, dirPath, id)` for `ensureShipmentReconcileArchiveOnlyLocation`.
  2. In `internal/core/shipment_lifecycle.go` (`snapshotShipArtifacts`, lines 235-268), replace the separate `findArtifact(ctx, ws, id)` and `FindArtifactPath(ctx, ws, id)` calls with a single `artifact, path, err := findArtifactWithPath(ctx, ws, id)` call.
* **RED-first acceptance criteria**:
  * **RED harness (`internal/core/shipment_gate_bench_test.go`, `package core`)**: Add `TestU3a_SnapshotShipArtifactsSinglePassContainedDirectLookup` (using only existing symbols so the RED harness compiles cleanly before `artifacts.go` or `shipment_lifecycle.go` is edited) which: (a) plants an unrelated escaping symlink `archive/000-unrelated.md` in `archive/` while placing 5 valid shipment member artifacts at `queue/{id}.md`, and invokes `snapshotShipArtifacts(ctx, ws, memberIDs)` — today `findArtifact` immediately runs `WalkDir("archive")` before checking `queue/{id}.md` and fails on `archive/000-unrelated.md` (`assertion RED`), whereas with the two-pass direct `{id}.md` lookup Pass 1 resolves `queue/{id}.md` directly without walking `archive/` (`GREEN`); (b) verifies that looking up `"000-unrelated"` directly still fails closed on `ensureArtifactLookupContained` and that a slug-named file (`queue/901-F-custom-slug.md`) is still found via Pass 2; and (c) parses `shipment_lifecycle.go` via `go/parser` to assert `snapshotShipArtifacts` calls `findArtifactWithPath` rather than calling both `findArtifact` and `FindArtifactPath`.
  * **Scoped verification command**: `go test -timeout=30m -count=1 -race -v -run '^(TestU3a_SnapshotShipArtifacts|TestSE7bLookupTimeContainmentHarness|TestRestoreShipArtifacts)' ./internal/core`

### Unit U3b: Memoize verified immutable commit ancestry checks in `isAncestor` across `ShipShipment` gate passes (`76553D8D` Part 2b)

* **Domain**: `code` (width-isolated to `internal/core`)
* **Size / Complexity**: `S` / `low` (3 files, <= 3 functions, <= 3 test scenarios)
* **Execution posture**: `test-first`
* **Dependencies**: `blocks` on Unit U3a (shares `internal/core/shipment_gate_bench_test.go`)
* **Affected files (3)**:
  * `internal/core/workspace.go`
  * `internal/core/shipment_gate.go`
  * `internal/core/shipment_gate_bench_test.go`
* **Changes needed**:
  1. In `internal/core/workspace.go` and `internal/core/shipment_gate.go` (`isAncestor`, lines 77-115):
     * Pass `--no-replace-objects` (`exec.CommandContext(runCtx, "git", "--no-replace-objects", "merge-base", "--is-ancestor", ancestor, descendant)`) so `refs/replace/*` refs cannot alter ancestry evaluation or poison cached verdicts.
     * Check `if err := ctx.Err(); err != nil { return false, fmt.Errorf("ancestor check aborted: %w", err) }` FIRST before consulting the cache.
     * Add a thread-safe bounded cache on `Workspace` (capped at 4,096 entries under a mutex, keyed by `strings.ToLower(ancestor) + "|" + strings.ToLower(descendant)` when both `isGitObjectName(ancestor)` and `isGitObjectName(descendant)` are full 40/64-hex commit SHAs) **restricted strictly to complete, ungrafted repository history**: inspect `.git/shallow` and `.git/info/grafts` under the resolved `.git` directory on every lookup and store; when either `.git/shallow` or `.git/info/grafts` is present (or cannot be confirmed absent via `os.IsNotExist`), clear/invalidate any cached entries and bypass the cache completely. When both `.git/shallow` and `.git/info/grafts` are absent, memoize only definitive `exit 0` (`true, nil`) and `exit 1` (`false, nil`) outcomes when `runCtx.Err() == nil` (never caching `exit 128` or exec/timeout errors).
  2. Preserve both `gateShipmentCompletion` passes in `ShipShipment` (`shipment_lifecycle.go:605,629`), member status/event-log reloads, `headSHABounded` drift checks, error classification, and lock order unchanged.
* **RED-first acceptance criteria**:
  * **RED harness (`internal/core/shipment_gate_bench_test.go`, `package core`)**: Add `TestU3b_ValidateMemberGateEvidenceMemoizesImmutableCommitAncestry` (referencing only existing functions so the RED harness compiles cleanly before `workspace.go` or `shipment_gate.go` is edited) verifying: (a) in a complete, ungrafted Git repository with ancestor commit `memberHead` and descendant `shipmentHead`, after `validateMemberGateEvidence(ctx, ws, ids, shipmentHead)` Pass 1 populates the cache and `.git/objects` is temporarily renamed to `.git/objects.bak`, Pass 2 succeeds from cache while an uncached SHA and a canceled `ctx` still fail closed; (b) creating `refs/replace/<shipmentHead>` pointing to an unrelated root commit does not change `isAncestor(ctx, memberHead, shipmentHead)` or poison the cache because `--no-replace-objects` is enforced; and (c) when `.git/shallow` marks `shipmentHead` as shallow so `isAncestor(ctx, memberHead, shipmentHead)` returns `false, nil`, that shallow result is not cached — removing `.git/shallow` (simulating `git fetch --unshallow`) causes `isAncestor` to re-evaluate and return `true, nil` (and introducing `.git/shallow` or `.git/info/grafts` invalidates/bypasses the cache).
  * **Scoped verification command**: `go test -timeout=30m -count=1 -race -v -run '^(TestU3b_ValidateMemberGateEvidence|TestValidateMemberGateEvidence|TestGateShipmentCompletion|TestIsAncestor)' ./internal/core`

### Unit U4: Order `busy_timeout` first in `db.Open` and enforce WAL-mode and schema initialization across pooled connections (`F448AA52` Part 1)

* **Domain**: `code` (width-isolated to `internal/db`)
* **Size / Complexity**: `S` / `low` (3 files, <= 3 functions, <= 3 test scenarios)
* **Execution posture**: `test-first`
* **Affected files (3)**:
  * `internal/db/connection.go`
  * `internal/db/schema.go`
  * `internal/db/connection_test.go`
* **Changes needed**:
  1. **Busy-timeout ordering (`internal/db/connection.go`, lines 58-68)**: Place `_pragma=busy_timeout(30000)` FIRST in `q["_pragma"]` before `_pragma=journal_mode(WAL)` and `_pragma=foreign_keys(1)` so every newly opened physical connection in the `database/sql` pool (`SetMaxOpenConns(4)`) sets its 30-second busy timeout before executing `PRAGMA journal_mode(WAL)`.
  2. **Database identity, persistent WAL header, and pooled-connection schema initialization (`internal/db/connection.go` & `internal/db/schema.go`)**:
     * In `internal/db/connection.go` (`Open`), verify on initial open that `PRAGMA journal_mode` equals `"wal"` (returning an error if WAL mode was not established), and when the newly opened `dbPath` on disk is 0 bytes (empty), materialize Page 1 (with the persistent 100-byte WAL header bytes `18..19 = 2, 2` via `PRAGMA user_version` / `PRAGMA wal_checkpoint(TRUNCATE)`) before returning `db` so no subsequent pooled connection (`Conn #2`..`Conn #4`) can ever open a 0-byte main `.db` file in `delete` mode (where a contended `PRAGMA journal_mode(WAL)` returns `"delete"` with `SQLITE_OK` and causes `no such table: commit_links`).
     * In `internal/db/schema.go` (`EnsureSchema` and `EnsureSchemaWithExtensions`), execute schema DDL and `migrations` deterministically and verify schema readiness (`commit_links` present in `sqlite_schema` and `PRAGMA journal_mode == "wal"`) before returning (do not rely on a best-effort `PRAGMA wal_checkpoint(PASSIVE)` or a main-file byte-size assertion).
* **RED-first acceptance criteria**:
  * **RED harness (`internal/db/connection_test.go`)**: Add `TestU4_OpenPragmaOrderAndPooledSchemaInitialization` (checking all `database.Close()` errors per `TestU175_075_LintFix`) verifying: (a) AST/source inspection of `connection.go` confirms `_pragma=busy_timeout(30000)` is added to `q` before `_pragma=journal_mode(WAL)` and `_pragma=foreign_keys(1)`; (b) right after `db.Open(dbPath)` on a brand-new path, Page 1 of `dbPath` on disk carries the persistent SQLite WAL header (bytes 18 and 19 equal `2`, failing deterministically today because `db.Open` leaves `dbPath` at 0 bytes on disk so a second connection whose `PRAGMA journal_mode(WAL)` is contended stays in `delete` mode and fails with `no such table: commit_links`); and (c) after `db.Open(dbPath)` + `db.EnsureSchema(database)`, while Connection #1 holds an active `BEGIN IMMEDIATE` write transaction inserting into `commit_links`, 4 concurrent goroutines opening/acquiring pooled connections on `database` verify `PRAGMA journal_mode == "wal"` and query/insert `commit_links` with zero `no such table: commit_links` or `SQLITE_BUSY` errors.
  * **Scoped verification command**: `go test -timeout=30m -count=1 -race -v -run '^(TestU4_OpenPragmaOrder|TestOpen|TestEnsureSchema|TestU175_025_LintFix|TestU175_075_LintFix)' ./internal/db`

### Unit U5: Bound `custodyGit` subprocess execution, deduplicate `custodyBlobDigest` calls, and harden `TestU4` `profile_b` under `-race` (`9CFC88E5`, `B25BF131`)

* **Domain**: `code` (width-isolated to `internal/core`)
* **Size / Complexity**: `M` / `medium` (3 files, <= 4 functions, <= 4 test scenarios)
* **Execution posture**: `test-first`
* **Dependencies**: `blocks` on Unit U3b (serializes `internal/core` edits)
* **Affected files (3)**:
  * `internal/core/custody_exec.go`
  * `internal/core/custody_exec_test.go`
  * `internal/core/custody_external_config_test.go`
* **Changes needed**:
  1. In `internal/core/custody_exec.go`:
     * Add `const defaultCustodyGitTimeout = 15 * time.Second` and update `custodyGit(ctx, root, args...)` (line 1066) to derive `runCtx, cancel := context.WithTimeout(ctx, defaultCustodyGitTimeout)`, prepend `"-C", root, "-c", "core.fsmonitor=false", "-c", "gc.auto=0", "--no-replace-objects", "--no-optional-locks"`, append `"GIT_TERMINAL_PROMPT=0"` to `cmd.Env`, set `cmd.WaitDelay = 2 * time.Second`, check `if ctxErr := runCtx.Err(); ctxErr != nil` before returning (wrapping both `ctxErr` and `err` when `err != nil` so `errors.Is(err, context.DeadlineExceeded)` and `errors.Is(err, context.Canceled)` are preserved), and in `custodyCurrentBranch` (`line 1051`) / `custodyRefuseDefaultBranch` (`line 1030`) avoid masking `context.DeadlineExceeded` / `context.Canceled` as `blerrors.ErrValidation` or silently falling back to `"main"`.
     * In `custodyLiveDigests` / `custodyBlobDigest` (`custody_exec.go:1100-1155`), check `if err := ctx.Err(); err != nil { return "", err }` first, avoid spawning duplicate `git cat-file blob <oid>` subprocesses when `indexOID == baseOID`, and memoize `oid -> sha256Digest` in a bounded thread-safe cache (capped at 4,096 entries, keyed by `filepath.Clean(root) + "|" + strings.ToLower(oid)` only when `isGitObjectName(oid)` is true).
  2. In `internal/core/custody_exec_test.go` and `internal/core/custody_external_config_test.go`:
     * In `buildCustExecSeed` (`custody_exec_test.go:384`), configure `seed.git("config", "core.fsmonitor", "false")` and `seed.git("config", "gc.auto", "0")` (while retaining `PRAGMA synchronous = FULL` in `custExecNewRepo` per the Ordinal 17 WAL `ModTime` invariant).
     * In `TestU4_CustodyPreviewOfferAcceptVerifyAndRecovery/profile_b_stage_to_ship_handoff_and_stale_digest_invalidation` and `TestExternal_LiveAcceptAndVerifyUseExternalStore`, use bounded contexts (`context.WithTimeout`) and include step-timing and full `Verdicts` diagnostic messages on every assertion.
* **RED-first acceptance criteria**:
  * **RED harness (`internal/core/custody_exec_test.go`, `package core`)**: Add `TestU5_CustodyGitBoundedTimeoutAndBlobDigestDeduplication` (referencing only existing symbols so the RED harness compiles cleanly before `custody_exec.go` is edited) verifying: (a) `strings.TrimSpace(runGit(t, repo.root, "config", "--get", "core.fsmonitor")) == "false"` and `strings.TrimSpace(runGit(t, repo.root, "config", "--get", "gc.auto")) == "0"` in `custExecNewRepo(t)` (failing deterministically today because `buildCustExecSeed` sets neither); (b) after calling `PreviewCustodyCandidates` on an `unstaged_only` candidate and temporarily moving `.git/objects` aside, `custodyBlobDigest(ctx, repo.root, baseOID)` returns the cached SHA-256 digest without spawning `git cat-file blob` while a canceled `ctx` still fails closed; and (c) AST inspection of `custody_exec.go` via `go/parser` confirms `custodyGit` sets `WaitDelay` and `WithTimeout`.
  * **Scoped verification command**: `go test -timeout=30m -count=1 -race -v -run '^(TestU5_CustodyGitBounded|TestU4_CustodyPreviewOfferAcceptVerifyAndRecovery|TestExternal_LiveAcceptAndVerifyUseExternalStore)' ./internal/core`

### Unit U6: Seed `newAuthSeamFixture` via `sync.Once` and bound `TestUR1B` active-slot subprocess (`F448AA52` Part 2)

* **Domain**: `tests` (width-isolated to `internal/core` test fixtures)
* **Size / Complexity**: `S` / `low` (2 files, <= 3 functions, <= 3 test scenarios)
* **Execution posture**: `test-first`
* **Dependencies**: `blocks` on Unit U5 (serializes `internal/core` test runs)
* **Affected files (2)**:
  * `internal/core/shipment_reconcile_auth_seam_test.go`
  * `internal/core/shipment_blocked_lifecycle_harness_test.go`
* **Changes needed**:
  1. In `internal/core/shipment_reconcile_auth_seam_test.go` (`newAuthSeamFixture`, lines 58-100), seed the base workspace (`.git` + `.backlogit` with `member`, archived `shipment`, `closureRel`, `evidenceRefRel`, `digest`, and Ed25519 keypair `priv`/`pub`/`key`, running `PRAGMA wal_checkpoint(TRUNCATE)` and `ws.Close()` before publishing the seed) once in a package-level `sync.Once` (`authSeamSeed`), and have each `newAuthSeamFixture(t)` call copy the seeded directory into `t.TempDir()`, open `NewDiagnosticWorkspace`, set `ws.Config.Reconcile = &config.ReconcileConfig{}`, and create a fresh `NonceConsumer` (eliminating 50+ redundant `CreateArtifact`/`CreateShipment`/`ArchiveItem`/`ed25519.GenerateKey` runs across `TestReconcile*`).
  2. In `internal/core/shipment_blocked_lifecycle_harness_test.go` (`TestShipmentBlockedActiveSlotSubprocessHelper` and `TestUR1B_FailClosedValidationAndAuthoritativeActiveSlot/claim_and_unblock_serialize_across_boundaries/processes`, lines 37-65 and 575-630):
     * Open the child workspace with `NewDiagnosticWorkspace` (avoiding redundant recovery scans during child startup) and bound `ClaimShipment` in the child with a 30-second context timeout;
     * Launch the child subprocess with `exec.CommandContext(subCtx, os.Args[0], ...)` (`30 * time.Second` timeout + `cmd.WaitDelay = 2 * time.Second`) and increase the readiness/start barrier deadline from `10 * time.Second` to `30 * time.Second` so `finishChild` can never hang `cmd.Wait()` past 12 minutes under `-race`.
* **RED-first acceptance criteria**:
  * **RED harness (`internal/core/shipment_reconcile_auth_seam_test.go`, `package core`)**: Add `TestU6_AuthSeamFixtureUsesSharedSeedAndUR1BSubprocessIsBounded` asserting across two `newAuthSeamFixture(t)` calls that `first.ws.RootPath != second.ws.RootPath` AND `require.Equal(t, first.priv, second.priv)` / `require.Equal(t, first.pub.Public, second.pub.Public)` (which compiles cleanly against existing `authSeamFixture` fields, fails 100% deterministically today because `ed25519.GenerateKey(nil)` generates a random keypair per call, and turns GREEN once `sync.Once` seeding is wired), plus AST verification of `shipment_blocked_lifecycle_harness_test.go` confirming `CommandContext` and `WaitDelay`, and verifying `TestReconcileResumeForgedPreparedEvent`, `TestUR1B_FailClosedValidationAndAuthoritativeActiveSlot`, and `TestU2_ReconstituteShipmentsAtomicExecutionAndRollback/concurrent_associate_commit_and_post_hook_isolation` pass under `-race`.
  * **Scoped verification command**: `go test -timeout=30m -count=1 -race -v -run '^(TestU6_AuthSeamFixture|TestReconcileResumeForgedPreparedEvent|TestUR1B_FailClosedValidationAndAuthoritativeActiveSlot)$' ./internal/core` and `go test -timeout=30m -count=1 -race -v -run '^TestU2_ReconstituteShipmentsAtomicExecutionAndRollback$/^concurrent_associate_commit_and_post_hook_isolation$' ./internal/core`

### Unit U7: Harden `TestU172_LockOrder` and `TestArchiveItemGovernance` contention synchronization under full-suite load (`63909DAE`)

* **Domain**: `code` (width-isolated to `internal/core`)
* **Size / Complexity**: `S` / `low` (3 files, <= 3 functions, <= 3 test scenarios)
* **Execution posture**: `test-first`
* **Dependencies**: `blocks` on Unit U6 (serializes `internal/core` edits)
* **Affected files (3)**:
  * `internal/core/archive.go`
  * `internal/core/artifact_mutation_lock_order_test.go`
  * `internal/core/shipment_archive_governance_test.go`
* **Changes needed**:
  1. In `internal/core/archive.go` (`lockArchiveGovernance`, lines 421-485), emit deterministic lifecycle hook stages on `shipmentLifecycleGlobalLockHookContextKey` when `lockArchiveGovernance` reaches the shipment-membership lock step (`"attempt_membership"`) and the item-log lock step (`"attempt_item_log"`, right before `lockItemLogsSorted(lockedCtx, ws, scopeIDs)` after `recoverPendingShipmentOperations` and `archiveRelatedShipmentIDs` have completed).
  2. In `internal/core/artifact_mutation_lock_order_test.go` (`TestU172_LockOrder_ArchiveItemDoesNotHoldArtifactLockWhileWaitingForItemLog`, lines 68-125), wait for the `"attempt_item_log"` stage signal (confirming `ArchiveItem` has finished recovery/discovery and is actively contending on `C`) before running the negative probe on `B` (`50ms`) and `runU172CBeforeBWriter(cLockedCtx, ws, item.ID, u172ObservationWait)` (using the `30s` `u172ObservationWait` budget instead of `3s` `defaultGateLockBoundedWait`).
  3. In `internal/core/shipment_archive_governance_test.go` (`TestArchiveItemGovernance_WaitsForGlobalLifecycleLock` and `TestArchiveItemGovernance_WaitsForShipmentMembershipLock`, lines 15-96), synchronize on `"contended"` (emitted by `lockTaskFileWithHeartbeat` in `internal/core/task_lock.go:126-128` the instant `lockTaskFile` observes `ErrTaskBusy` after `"attempt"` / `"attempt_membership"`) before unlocking, and increase the post-unlock completion wait from `defaultGateLockBoundedWait + 2*time.Second` (`5s`) to `u172ObservationWait` (`30s`).
* **RED-first acceptance criteria**:
  * **RED harness (`internal/core/artifact_mutation_lock_order_test.go`)**: Add `TestU7_ArchiveGovernanceEmitsDeterministicMembershipAndItemLogAttemptStages` verifying that `ArchiveItem` with a `shipmentLifecycleGlobalLockHookContextKey` observer emits `"acquired"`, `"attempt_membership"`, and `"attempt_item_log"` in strict causal order before acquiring `C` and `B`, and that `TestU172_LockOrder_ArchiveItemDoesNotHoldArtifactLockWhileWaitingForItemLog`, `TestArchiveItemGovernance_WaitsForGlobalLifecycleLock`, and `TestArchiveItemGovernance_WaitsForShipmentMembershipLock` pass deterministically under concurrent load.
  * **Scoped verification command**: `go test -timeout=30m -count=1 -race -v -run '^(TestU7_ArchiveGovernance|TestU172_LockOrder_|TestArchiveItemGovernance_)' ./internal/core`

## Dependency Graph

```text
Unit U2 (76553D8D Part 1: internal/events/reader.go 64KiB->1MiB scanner buffer) [independent]

Unit U4 (F448AA52 Part 1: internal/db/connection.go + schema.go pragma order & pooled WAL/schema initialization) [independent]

Unit U1 (703835DC: internal/core/commits.go lock-order fix & shipment-member test)
  └──> Unit U3a (76553D8D Part 2a: internal/core/artifacts.go + shipment_lifecycle.go findArtifactWithPath)
         └──> Unit U3b (76553D8D Part 2b: internal/core/shipment_gate.go isAncestor memoization)
                └──> Unit U5 (9CFC88E5 + B25BF131: internal/core/custody_exec.go bounded custodyGit & OID memo)
                       └──> Unit U6 (F448AA52 Part 2: internal/core reconcile seed & UR1B bounded child)
                              └──> Unit U7 (63909DAE: internal/core archive.go + TestU172 & TestArchiveItemGovernance)
```

* **Wave 1 (Parallel DAG roots across 3 disjoint packages)**: `Unit U1` (`internal/core`), `Unit U2` (`internal/events`), `Unit U4` (`internal/db`).
* **Wave 2**: `Unit U3a` (`internal/core`).
* **Wave 3**: `Unit U3b` (`internal/core`).
* **Wave 4**: `Unit U5` (`internal/core`).
* **Wave 5**: `Unit U6` (`internal/core`).
* **Wave 6**: `Unit U7` (`internal/core`).

## Decisions and Rationale

1. **Preserve both gate passes and Markdown reloads in `ShipShipment` (`76553D8D`)**: Per `shipment_lifecycle.go:624-626` and `docs/compound/2026-07-28-attach-commit-repersist-must-reload-from-markdown.md`, Pass 2 re-validates after `C` and `B` are held and `attachCommitToItems` must load from Markdown so `item_links` and archive provenance are never dropped. Optimizing `ReadAllEvents` (`64 KiB -> 1 MiB`), `findArtifactWithPath` (contained direct `{id}.md` lookup), and `isAncestor` (replacement-free `--no-replace-objects` `(ancestorSHA, descendantSHA)` memoization restricted to complete, ungrafted history) eliminates the O(M * N) filesystem walks, 93% of scanner buffer zeroing, and duplicate `git merge-base` subprocesses without altering any safety invariant.
2. **Place `_pragma=busy_timeout(30000)` first in `db.Open` and materialize the persistent Page 1 WAL header / verify schema initialization (`F448AA52`)**: Separates `busy_timeout` pragma ordering from the 0-byte main `.db` non-WAL connection failure mode (where SQLite's `PRAGMA journal_mode(WAL)` on a locked 0-byte file returns `"delete"` with `SQLITE_OK` rather than `SQLITE_BUSY`, leaving a newly opened pooled connection in `delete` mode and causing `no such table: commit_links`), ensuring every pooled connection opens in WAL mode and sees `commit_links`.
3. **Memoize `custodyBlobDigest` by immutable Git blob OID, never by file path (`9CFC88E5`, `B25BF131`)**: Git blob OIDs are cryptographic hashes over blob content, so `(root, oid) -> custodyDigest` is mathematically invariant while working-tree edits and index changes produce new OIDs via `git ls-files` / `custodyWorktreeDigest`.

## Risks and Caveats

* **Risk**: `PRAGMA synchronous = OFF` in `custExecNewRepo` causing delayed Windows NTFS `backlogit.db-wal` `ModTime` updates in `TestU4_CustodyPreviewOfferAcceptVerifyAndRecovery/preview_and_verify_zero_write_and_default_branch_refusal`.
  * **Mitigation**: `custExecNewRepo` (`internal/core/custody_exec_test.go`) explicitly retains `PRAGMA synchronous = FULL` per `stage-212-s-089-dl-ordinal17-u1k-custody-wal-fix.md`.
* **Risk**: Direct `{id}.md` candidate lookup in `findArtifactWithPath` bypassing lookup-time containment (`TestSE7bLookupTimeContainmentHarness`).
  * **Mitigation**: Always run `ensureArtifactLookupContained(ws, candidate)` on the direct candidate path before reading/parsing it, returning any containment error immediately.

## Constitution Check

* **Safety-First Go (`NON-NEGOTIABLE`)**: `pass` — All changes are idiomatic Go 1.24, wrap errors with `%w`, and use explicit synchronization (`sync.Mutex` / `sync.Map` / `atomic`) with zero `unsafe` usage.
* **Test-First Development (`NON-NEGOTIABLE`)**: `pass` — Every unit (`U1` through `U7`) defines a deterministic, non-timing compile-PASS + assertion-RED harness before production edits.
* **Workspace Isolation and Security Boundaries (`NON-NEGOTIABLE`)**: `pass` — `findArtifactWithPath` enforces `ensureArtifactLookupContained` before reading any file, and `custodyGit` preserves `config.ChildProcessEnv()` with `--no-replace-objects` and `--no-optional-locks`.
* **CLI Workspace Containment (`NON-NEGOTIABLE`)**: `pass` — All operations and test fixtures remain strictly within workspace/temp roots.
* **Destructive Command Approval (`NON-NEGOTIABLE`)**: `pass` — No destructive commands are introduced.
* **Merge Commit History Preservation (`NON-NEGOTIABLE`)**: `pass` — Shipment `217-S` will merge via a merge commit (`--merge`) under P-009/P-014 after predecessor `216-S` ships.

Constitution Check: pass

## Plan Hardening Signals

* **public API, schema, or contract change**: `present` — Tightens `LinkCommit` lock-ordering contract (`703835DC`) and `custodyGit` bounded execution contract (`9CFC88E5`).
* **security, auth, permission, or compliance-sensitive behavior**: `present` — Touches `isAncestor` shipment-gate lineage verification (`76553D8D`), `findArtifactWithPath` lookup containment (`76553D8D`), and `custodyGit` / `custodyBlobDigest` workflow-custody verification (`9CFC88E5`).
* **migration, backfill, destructive data/config action, or irreversible step**: `absent` — No schema migration or destructive data change (`db.Open` and `EnsureSchema` enforce persistent WAL header and pooled schema initialization).
* **external integration, operator checkpoint, or external dependency**: `absent` — No new external dependencies.
* **high runtime, rollout, or rollback risk**: `present` — Modifies core lock ordering (`commits.go`, `archive.go`), `ShipShipment` closure hot path, and `db.Open` connection-pool pragma ordering.

Requires plan hardening: yes

## Runtime Verification and Closure

* **Changed runtime surfaces**:
  * Commit association (`backlogit update --commit`, `backlogit_track_commit`, `ShipShipment` commit attachment in `internal/core/commits.go`)
  * Shipment closure (`backlogit shipment ship`, `backlogit_ship_shipment` in `internal/core/shipment_lifecycle.go`, `internal/core/shipment_gate.go`, `internal/events/reader.go`)
  * Workflow custody Git inspection (`backlogit custody preview/offer/accept/verify` in `internal/core/custody_exec.go`)
  * SQLite index connection pool initialization (`internal/db/connection.go`, `internal/db/schema.go`)
* **Runtime verification probes (for `_Ship` Step 6)**:
  1. Run the full `internal/core`, `internal/db`, and `internal/events` test suites under `-race` (`go test -timeout=30m -count=1 -race ./internal/db ./internal/events ./internal/core`) and verify zero failures, zero `no such table: commit_links` errors, and zero timeouts.
  2. Run `go test -timeout=30m -count=1 -race -v -run '^(TestU1_CommitWriters|TestU3a_SnapshotShipArtifacts|TestU3b_ValidateMemberGateEvidence|TestU5_CustodyGitBounded|TestU6_AuthSeamFixture|TestU7_ArchiveGovernance|TestU172_LockOrder_|TestArchiveItemGovernance_|TestExternal_LiveAcceptAndVerifyUseExternalStore|TestU4_CustodyPreviewOfferAcceptVerifyAndRecovery|TestReconcileResumeForgedPreparedEvent|TestUR1B_FailClosedValidationAndAuthoritativeActiveSlot)$' ./internal/core` and `go test -timeout=30m -count=1 -race -v -run '^TestU2_ReconstituteShipmentsAtomicExecutionAndRollback$/^concurrent_associate_commit_and_post_hook_isolation$' ./internal/core`.
* **Operational closure expectations**:
  * **Owner**: `_Ship` agent (supervised by `_Orchestrator` and operator).
  * **Validation window**: Immediate post-merge verification on `main` plus next shipment closure observation window.
  * **Healthy signals**: Fast `backlogit_ship_shipment` completion without MCP `-32001` timeouts, zero `ErrShipmentLockOrderInverted` or `no such table: commit_links` errors, and green unfiltered 50-package convergence runs.
  * **Rollback trigger & procedure**: Any lock-order regression, custody verification regression, or shipment gate discrepancy triggers a reviewed `git revert -m 1 <merge_sha>` PR; no database schema migration is involved, so rollback is cleanly reversible.

## Plan Hardening

* **Hardening required**: Yes — P-006 triggered by concurrency lock-ordering (`703835DC`), `ShipShipment` gate/closure optimization (`76553D8D`), SQLite connection-pool initialization (`F448AA52`), and custody Git subprocess execution (`9CFC88E5`).
* **Protected invariants**:
  * **I1 (Canonical lock order `lifecycle-global -> A -> C -> B`)**: Neither `AssociateCommit` nor `LinkCommit` may acquire `C` (`LockItemLogCrossProcess`) or mutate `commit_links` before `lifecycle-global` is held when the target is a shipment or shipment member, and a preheld `C` without verified `lifecycle-global` ownership must fail closed with `ErrShipmentLockOrderInverted`.
  * **I2 (`ShipShipment` dual-gate and Markdown reload integrity)**: Both `gateShipmentCompletion` passes (`shipment_lifecycle.go:605,629`) must remain intact, `attachCommitToItems` must continue to reload from Markdown via `findArtifact` (never SQLite `loadArtifact`) so `item_links` and archive provenance survive, and `findArtifactWithPath` must run `ensureArtifactLookupContained` before reading any direct `{id}.md` candidate.
  * **I3 (Content-addressed memoization safety)**: `isAncestor` enforces `--no-replace-objects` and memoizes only full 40/64-hex `(ancestorSHA, descendantSHA)` commit pairs when the repository has complete, ungrafted history (`.git/shallow` and `.git/info/grafts` both absent, clearing/bypassing the cache whenever `.git/shallow` or `.git/info/grafts` is present), and `custodyBlobDigest` memoizes only immutable 40/64-hex Git blob OIDs (never file paths or working-tree reads).
  * **I4 (Ordinal 17 `custExecNewRepo` WAL `ModTime` invariant)**: `custExecNewRepo` (`internal/core/custody_exec_test.go`) must NOT set `PRAGMA synchronous = OFF` on `repo.ws.DB` so `OfferCustodyHandoff` flushes `backlogit.db-wal` before `custExecSnapshot`.
* **Reinforced instructions and compound learnings consulted**:
  * `.github/instructions/constitution.instructions.md`, `.github/instructions/concurrency.instructions.md`, `.github/instructions/go.instructions.md`
  * `docs/compound/2026-10-19-lock-held-side-effect-ordering.md`
  * `docs/compound/2026-10-06-ship-shipment-mcp-timeout-result-unknown.md`
  * `docs/compound/2026-07-28-attach-commit-repersist-must-reload-from-markdown.md`
  * `docs/compound/best-practices/red-snapshot-observation-scoped-metadata-memo-2026-11-08.md`
  * `docs/compound/2026-07-06-bounded-helper-timeout-hard-cap.md`
  * `docs/compound/test-failures/separate-compile-budget-and-own-subprocess-tree-2026-11-08.md`
  * `docs/compound/2026-10-11-unfiltered-suite-evidence-under-load.md`
  * `docs/memory/2026-11-10/stage-212-s-089-dl-ordinal17-u1k-custody-wal-fix.md`
* **Risky actions classification (`strict-safety`)**:

| ProposedAction | Target Surface | ActionRisk | Approval / Guardrail Path | Expected ActionResult |
| --- | --- | --- | --- | --- |
| Enforce `lifecycle-global -> C -> B` in `LinkCommit` and `AssociateCommit` | `internal/core/commits.go` | `moderate` (lock ordering) | Re-entrant when `ShipShipment` already holds `lifecycle-global`; fail-closed on preheld `C` without `global` | Eliminates `C`-then-`global` lock-order inversion (`703835DC`) |
| Use 64 KiB initial / 1 MiB max scanner buffer in `ReadAllEvents` | `internal/events/reader.go` | `low` (memory allocation) | Retains `1<<20` max line cap so oversized-line rejection is unchanged | Cuts per-call buffer allocation and zeroing by 16x (`76553D8D`) |
| Add contained direct `{id}.md` probe in `findArtifactWithPath` and memoize replacement-free `isAncestor` SHA pairs in complete, ungrafted repos | `internal/core/artifacts.go`, `shipment_lifecycle.go`, `shipment_gate.go` | `moderate` (gate & lookup path) | `ensureArtifactLookupContained` runs before `parseFile`; `--no-replace-objects` enforced; cache bypassed/cleared when `.git/shallow` or `.git/info/grafts` exists | Eliminates O(M * N) walks and duplicate `git merge-base` spawns while preserving both gate passes and shallow/replace safety |
| Order `_pragma=busy_timeout(30000)` first in `db.Open` and enforce persistent Page 1 WAL header & pooled schema initialization | `internal/db/connection.go`, `schema.go` | `moderate` (SQLite pool init) | Preserves `SetMaxOpenConns(4)`, WAL mode, `foreign_keys(1)`, and `_txlock=immediate`; materializes Page 1 WAL header on 0-byte `.db` files | Eliminates pooled-connection `no such table: commit_links` under `-race` (`F448AA52`) |
| Add `custodyGitTimeout` + `WaitDelay` + `fsmonitor=false`/`gc.auto=0` and blob-OID digest cache | `internal/core/custody_exec.go`, `custody_exec_test.go` | `moderate` (custody Git execution) | Fails closed on `runCtx.Err()`; caches only immutable blob OIDs; keeps `synchronous = FULL` in `custExecNewRepo` | Eliminates `os/exec.Cmd.Wait` hangs (`9CFC88E5`) and `-race` subprocess overhead (`B25BF131`) |
| Synchronize `TestU172` and `TestArchiveItemGovernance` on deterministic contention hooks | `internal/core/archive.go`, `artifact_mutation_lock_order_test.go`, `shipment_archive_governance_test.go` | `low` (test synchronization) | Uses context hook stages without altering production lock order or `defaultGateLockBoundedWait` | Eliminates full-suite load sensitivity (`63909DAE`) |

* **Review-gate capability and marker requirements**:
  * `plan-review` MUST probe subagent dispatch capability, record literal `dispatch_mode: multi-agent-dispatch` (or `single-agent-declared-degradation` if degraded), cover all required personas (`Constitution Reviewer`, `Go Reviewer`, `Scope Boundary Auditor`, `Learnings Researcher`, `Architecture Strategist`, `Agent-Native Parity Reviewer`, `Security Lens Reviewer`) plus `Adversarial Review` and `rubber-duck` review, and record literal `decision: PASS` (or `ADVISORY` with `operator_authorization: approved`) before harvest.
* **Unresolved operator decisions**: None (`DARK_MODE_ACTIVE: true` pre-authorizes CAP-3 scope and staging into `216-F` / `217-S` blocked by `216-S`).

## Plan Review

* `dispatch_mode: multi-agent-dispatch`
* `decision: PASS`
* **Gate rationale**: Dispatched all 7 `plan-review` personas (`Constitution Reviewer`, `Go Reviewer`, `Scope Boundary Auditor`, `Learnings Researcher`, `Architecture Strategist`, `Agent-Native Parity Reviewer`, `Security Lens Reviewer`) plus independent `Adversarial Review` (`mode: report-only`) and `rubber-duck` (`Correctness Reviewer`) review. All P0/P1/P2/P3 findings identified during review (`CAP3-F1` through `CAP3-F6`, `CAP3-LR-001` through `CAP3-LR-003`) were incorporated directly into Units U1–U7 above before harvest:
  1. **`CAP3-F1` (P0 — `*ShipmentLifecycleLockOrderError` type & `package core` test visibility)**: Replaced non-existent `blerrors.ErrShipmentLockOrderInverted` with `*ShipmentLifecycleLockOrderError` (`require.ErrorAs`) and placed `TestU1_CommitWritersShipmentMemberLockOrderGlobalBeforeItemLog` in `internal/core/reconstitution_review_regression_test.go` (`package core`).
  2. **`Go-Reviewer P1` & `CAP3-F2` (P1 — Compile-clean symbol-free RED harnesses for Units U3a, U3b, and U5 + Two-Pass direct `{id}.md` search across `artifactSearchDirs`)**: Restructured `findArtifactWithPath` / `FindArtifactPath` in Unit U3a as a two-pass search across `searchDirs` (Pass 1: direct `{id}.md` probe across all `searchDirs`; Pass 2: `WalkDir` fallback) so live items in `queue/` never trigger a `WalkDir` over `archive/`, and designed `TestU3a`, `TestU3b`, and `TestU5` RED harnesses using existing production symbols (`Compilation: PASS` before production edits).
  3. **`CAP3-F3` (P1 — Non-false-green RED assertion in `TestU6`)**: Updated `TestU6_AuthSeamFixtureUsesSharedSeedAndUR1BSubprocessIsBounded` to assert `require.Equal(t, first.priv, second.priv)` and `require.Equal(t, first.pub.Public, second.pub.Public)` (which is RED today and GREEN with `sync.Once` seeding) and ensured `PRAGMA wal_checkpoint(TRUNCATE)` + `ws.Close()` + `ws.Config.Reconcile = &config.ReconcileConfig{}` in `authSeamSeed`.
  4. **`CAP3-F4` & `PRRT_kwDORzozKM63xezz` (P1 — Separate `busy_timeout` ordering from persistent Page 1 WAL header / pooled schema initialization in `internal/db`)**: Placed `_pragma=busy_timeout(30000)` first in `db.Open`, materialized Page 1 (`hdr[18]==2, hdr[19]==2`) on newly created 0-byte `.db` files in `db.Open`, and verified WAL-mode and `commit_links` schema readiness in `EnsureSchema` / `EnsureSchemaWithExtensions` with a concrete concurrent pooled-connection reproducer in `TestU4_OpenPragmaOrderAndPooledSchemaInitialization` (removing the `PASSIVE` checkpoint / main-file-size heuristic).
  5. **`PRRT_kwDORzozKM63xezs` & `PRRT_kwDORzozKM63xezU` (P1 — `--no-replace-objects` and complete/ungrafted history guard in `isAncestor`, and split `-run` commands in `Unit U6`)**: Added `--no-replace-objects` and `.git/shallow` / `.git/info/grafts` checks to `isAncestor` (`Unit U3b`) with replacement-ref and shallow-deepen regression subtests, and split the top-level and subtest `-run` selectors in `Unit U6` into two valid commands.
  5. **`CAP3-F5` & `Agent-Native Parity P2` (P2 — Fail-closed `ctx.Err()` check before memo hits & timeout error preservation in `custodyCurrentBranch`/`custodyRefuseDefaultBranch`)**: Added `ctx.Err()` checks before cache hits in `isAncestor` and `custodyBlobDigest`, bounded both caches to 4,096 entries, and prevented `custodyCurrentBranch` / `custodyRefuseDefaultBranch` from masking `context.DeadlineExceeded` / `context.Canceled` as `ErrValidation`.
  6. **`Rubber-Duck P2` (Unit U7 `"contended"` hook synchronization)**: Synchronized `TestArchiveItemGovernance_WaitsForGlobalLifecycleLock` and `TestArchiveItemGovernance_WaitsForShipmentMembershipLock` on `hook("contended")` (emitted by `lockTaskFileWithHeartbeat` on `ErrTaskBusy`) and `u172ObservationWait` (`30s`).
* **Plan hardening verification**: `Requires plan hardening: yes` is satisfied by the complete `## Plan Hardening` section with `ProposedAction` / `ActionRisk` classifications and `## Runtime Verification and Closure`.

