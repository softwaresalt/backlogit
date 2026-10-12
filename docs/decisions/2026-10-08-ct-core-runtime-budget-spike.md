---
title: 'CT internal/core runtime budget spike (T4)'
source: docs/decisions/2026-10-08-ct-core-runtime-budget-spike.md
doc_type: decision
chunk_strategy: h1-h2-h3
schema_version: "1.0"
---

# CT internal/core runtime budget spike (T4)

## Scope and result

Task 198.004-T (plan unit T4) measured the runtime of the `internal/core` test package on the Windows host, ranked its 25 slowest tests, classified each one, and listed follow-up proposals. No code, test, plan, or backlog file was changed. The package passed.

- Total package time: 788.962 seconds (about 13 minutes 9 seconds), from the package-level pass event.
- Package result: pass. 1445 tests passed, 0 failed, and 11 were skipped. No test failed, so none is reported as a failure or waived.
- Host condition: alone. This session ran no other command during the run. Host-wide load was not sampled.
- Run: started 2026-10-12T22:57:23Z and ended 2026-10-12T23:10:42Z (about 13 minutes 19 seconds of wall time), exit code 0. The stderr capture is empty.
- Run command: `go test -json -count=1 -timeout 40m ./internal/core`. The saved output is in the git-ignored `logs/` tree and is not pasted here.

The plan records 907.6 seconds for a quiet host (197-S closure). This run took 118.6 seconds less. This spike does not explain the difference.

## Method

- Ranks come from the saved JSON. Each test's elapsed time is taken from its pass or fail event. Skipped tests are not ranked. Subtests are ranked under their own names, so a wrapper and the test it wraps both appear where both are slow.
- Rank 25 is 5.36 seconds and rank 26 is 5.35 seconds, so the cut-off has no tie.
- Each class is judged from the test body and its helpers, not from a profile. The subtest timings in the same JSON were used to pick the dominant cost. Where a test fits more than one class, the dominant cost is named in the class notes.
- Ranks 1 and 20 are source-tree AST walks. None of the four classes names that cost. `recovery-enabled-fixture` is recorded as the nearest class because the cost is set by the size of the tree the test is given. It is a best fit, not an exact one.

## Ranked tests

| Rank | Test | Seconds | Class |
| --- | --- | ---: | --- |
| 1 | `TestU172_ItemLogBarrierSetterUnreachableFromProduction` | 170.57 | recovery-enabled-fixture |
| 2 | `TestNextGateEvidenceCounter_LockSurvivesPastStaleTTLWhileHeld` | 22.25 | sleep-or-poll |
| 3 | `TestUR2_GenericBulkAndCascadePathsRefuseGovernedEdges` | 17.61 | fsync-heavy |
| 4 | `TestUR1B_FailClosedValidationAndAuthoritativeActiveSlot` | 12.47 | fsync-heavy |
| 5 | `TestP1C4_BlockedShipmentMembershipIsImmutable` | 10.57 | fsync-heavy |
| 6 | `TestNormalizeBlockedShipmentForRecovery_RejectsMalformedCanonicalOwnershipJournal` | 10.48 | recovery-enabled-fixture |
| 7 | `TestU20_ReconcileShipmentToShippedConcurrency` | 8.58 | fsync-heavy |
| 8 | `TestUR2_GovernedWriterBoundaryRejectsBypassAndCorrelatesWrites` | 8.18 | fsync-heavy |
| 9 | `TestUR2_GovernedWriterBoundaryRejectsBypassAndCorrelatesWrites/lower_writer_surfaces_refuse_block_unblock_and_generic_activation` | 7.96 | fsync-heavy |
| 10 | `TestPrepareShipmentReconcileEvidence_RejectsUntrustedOrUnreachableTrustedRefTips` | 7.78 | real-subprocess |
| 11 | `TestPrepareShipmentReconcileEvidence_RejectsWrongDeliveryMergeRoleOrShipment` | 7.38 | real-subprocess |
| 12 | `TestPrepareShipmentReconcileEvidence_RejectsClosurePathTraversalAndSymlinkEscape` | 7.07 | real-subprocess |
| 13 | `TestPrepareShipmentReconcileEvidence_RejectsReachableNonMergeCommit` | 6.97 | real-subprocess |
| 14 | `TestPrepareShipmentReconcileEvidence_HappyPathUsesDefaultTrustedRefAndBuildsCanonicalEvent` | 6.92 | real-subprocess |
| 15 | `TestP1C7_BlockedEnvelopeRequiresCanonicalMetadata` | 6.84 | fsync-heavy |
| 16 | `TestPrepareShipmentReconcileEvidence_PersistsExpectedClosureHash` | 6.76 | real-subprocess |
| 17 | `TestPrepareShipmentReconcileEvidence_RejectsAmbiguousDuplicateShipmentSections` | 6.72 | real-subprocess |
| 18 | `TestPrepareShipmentReconcileEvidence_RejectsInvalidEvidenceRefs` | 6.47 | real-subprocess |
| 19 | `TestErrShipmentReconcileEvidence_SentinelWrapping` | 6.25 | real-subprocess |
| 20 | `TestWorkspaceLiteralGuard_NoHardcodedLegacyStorageRoot` | 5.73 | recovery-enabled-fixture |
| 21 | `TestP021RecoveryCAS_RefusesDriftAndRestoresExactPreimage` | 5.62 | recovery-enabled-fixture |
| 22 | `TestUT2_CaptureSitesRestoreStdlibLogState` | 5.41 | fsync-heavy |
| 23 | `TestUR10_SubprocessCrashReopenRecovery` | 5.38 | real-subprocess |
| 24 | `TestAppendGateEvidence_ConcurrentSameItem_NoDuplicateCounters` | 5.37 | sleep-or-poll |
| 25 | `TestUT1_ConcurrentSameItemCounterContract` | 5.36 | sleep-or-poll |

Rank 25 wraps rank 24, and rank 22 is a wrapper over three capture-site subtests. Both wrappers are expected duplicates and count as one finding each.

## Class notes

- **real-subprocess (ranks 10 to 14, 16 to 19, 23).** The reconcile-evidence fixture, `setupShipmentReconcileEvidenceFixture`, builds a git repository through `runGit`, which runs a dozen or more git commands per test (init, config, add, commit, merge, branch, checkout). Rank 23 re-executes the test binary three times and waits on file markers. Process startup dominates.
- **fsync-heavy (ranks 3, 4, 5, 7, 8, 9, 15, 22).** Each subtest builds a blocked-shipment fixture and then commits durable writes: SQLite WAL rows, artifact files, and fsync'd event appends. Subtest timings are one to four seconds each. Rank 4 also has a subprocess subtest (`processes`, 2.9 seconds), but the fixture writes dominate across its other subtests. Rank 7 is dominated by `ReconcileVsAssociateCommit` (4.41 seconds), whose reconcile append syncs the file and its directory. Rank 22's 5.41 seconds is the sum of its children, and 3.70 seconds of that is `shipped_event_append_failure`.
- **sleep-or-poll (ranks 2, 24, 25).** Rank 2 sleeps on purpose for the gate-lock heartbeat interval plus 2 seconds, about 20 seconds in total. Ranks 24 and 25 each race 20 goroutines. Most of their time is backoff polling in the counter lock and the event-stream lock, which starts at 20 ms and doubles to a 250 ms cap. The per-append fsync is the smaller share.
- **recovery-enabled-fixture (ranks 6, 21, plus ranks 1 and 20 as nearest class).** Ranks 6 and 21 run recovery journal fixtures. Rank 21 writes a lifecycle journal and then runs `recoverPendingShipmentOperations` for each subtest.

## Rank 1 finding

Rank 1, `TestU172_ItemLogBarrierSetterUnreachableFromProduction` (170.57 seconds), walks the whole module tree from the module root. It skips only dot-directories, `vendor`, `testdata`, and `node_modules`, so it does not skip the git-ignored `logs/` directory. That directory holds about 13,100 Go source files (about 355 MB), most of them in one scratch copy under `logs/2a355f83-proof`. The test parses every non-test `.go` file it reaches. Its cost is therefore set by git-ignored scratch state, not by production code. The test passed.

Rank 20, `TestWorkspaceLiteralGuard_NoHardcodedLegacyStorageRoot` (5.73 seconds), parses the non-test Go files of six `internal` packages and does not reach `logs/`.

## Follow-up proposals

Each follow-up below is a proposal only. No stash entry was captured inside this task. Ship captures them at the post-merge closure and records the stash IDs there and in a comment on 198.004-T.

| Follow-up | Title | Supporting ranks |
| --- | --- | --- |
| FU-1 | Limit the module-wide walk in TestU172 to tracked Go packages so it skips the git-ignored logs tree | 1 |
| FU-2 | Parse the production sources once per package and share the AST between the two source-guard tests | 1, 20 |
| FU-3 | Build the reconcile-evidence git repository once per package and clone or copy it into each test | 10, 11, 12, 13, 14, 16, 17, 18, 19 |
| FU-4 | Make the gate-lock heartbeat interval injectable so the stale-TTL test does not wait about 20 seconds | 2 |
| FU-5 | Evaluate a test-only cap, or a wake on release, for the counter-lock backoff used by the 20-goroutine tests | 24, 25 |
| FU-6 | Build the blocked-shipment fixture once per table group instead of once per subtest | 3, 4, 5, 8, 9, 15 |
| FU-7 | Build recovery-journal fixtures directly instead of through the full blocked-shipment setup | 6, 21 |
| FU-8 | Replace the re-executed test-binary helper in the UR10 test and the UR1B processes subtest with a lighter helper | 4, 23 |
| FU-9 | Profile the top 25 with a CPU profile to confirm the classes before any fix is chosen | 1, 3, 7, 22 |

## Manifest

```text
SPIKE_RUN_COMMAND: go test -json -count=1 -timeout 40m ./internal/core
SPIKE_HOST_CONDITION: alone
SPIKE_RUN_EVIDENCE_PATH: logs/ct-spike/internal-core-2026-10-12.json
SPIKE_TOTAL_PACKAGE_SECONDS: 788.962
SPIKE_FOLLOWUP_COUNT: 9
```

## Limits

- This is one run on one host. Timings are load-sensitive. The plan records 907.6 seconds on a quiet host and 1656 seconds under suite load, and this quiet run took 788.962 seconds.
- The classes are based on source reading. No profile was taken, so FU-9 is the check that should precede any fix.
- Rank 1 depends on the contents of the git-ignored `logs/` tree on this host. A host with a different `logs/` tree will report a different rank 1 time.
