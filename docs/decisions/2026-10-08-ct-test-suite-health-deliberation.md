---
title: 'Deliberation: CT test-suite health'
doc_type: decision
source: docs/decisions/2026-10-08-ct-test-suite-health-deliberation.md
schema_version: "1.0"
chunk_strategy: h1-h2-h3
description: 'Stage deliberation for CT, test-suite health after the 2026-10-08 full-suite failure: the gate-evidence counter concurrency flake (8F41D60D), the slog/log capture leak (D8EF5443), the release client timeout flake (92AB6879), the internal/core runtime budget, and verification of the earlier isolation fixes (DB48A817, FF1F3AC7, 885263D2), with 072-DL as context.'
topic: test-suite-health
depth: standard
decision_status: accepted
promoted_to: docs/exec-plans/2026-10-08-ct-test-suite-health-plan.md
linked_artifacts:
  - docs/evidence/2026-10-08-ct-full-suite-failure.md
  - docs/decisions/2026-10-08-cx-s-ship-closure-gate-correctness-deliberation.md
---

# Deliberation: CT test-suite health

## Subject

On 2026-10-08 the Orchestrator ran `go test ./... -count=1 -timeout 30m` on
main at d2f3ff97. It failed with EXIT=1 after 28m41s (evidence
`docs/evidence/2026-10-08-ct-full-suite-failure.md`). The only failing test was
`TestAppendGateEvidence_ConcurrentSameItem_NoDuplicateCounters` in
internal/core. internal/core took 1656s of the 1800s timeout (92%). Every
other package passed. The operator asked for a CT group: the new flake entry
8F41D60D, D8EF5443, 92AB6879, and 11BE840F if still active (it is not), with
072-DL as context.

## Framing

### Flake 8F41D60D

The test starts 20 goroutines that call `ws.appendGateEvidence` for one item
with formal gates required. Each call takes the per-item counter lock
(`nextGateEvidenceCounter`, `internal/core/gate_evidence_formal.go`) with a
bounded wait of `gateEvidenceCounterBoundedWait = 10s`. The holder scans the
log for the maximum counter, signs, and durably appends before it unlocks.
Under full-suite load, 20 serialized holders can exceed 10s for the last
waiter. That waiter then gets `ErrGateInProgress`, which is the documented
fail-closed outcome. The test requires `NoError` from every goroutine, so its
result depends on host load. The invariant it exists to prove (106-F F1: no
duplicate counter persisted) did not fail.

### D8EF5443

Three `slog.SetDefault` sites in internal/core tests restore the previous slog
default, but restoring does not reset `log.SetOutput` or `log` flags. Later
output in the test binary goes to a dead buffer, which removed diagnostics
from full-suite captures.

### 92AB6879

`TestClientLatestRespectsContextTimeout` (`internal/release/release_test.go`)
uses a 25ms context deadline against a server that sleeps 200ms, and asserts
`elapsed < 150ms`. That margin is too tight under load.

### internal/core runtime budget

At 92% of a 30-minute budget, internal/core is the next likely timeout. 072-DL
and 174.073-T already moved the size/complexity fixtures to the recovery-free
seam. No current entry owns a fresh profile (11BE840F is no longer active).

### Earlier isolation fixes

DB48A817 (shared `disableExecGateForTest` default), FF1F3AC7 (durability
fixture fake gate broker), and 885263D2 (`NewWorkspaceWithoutRecoveryForTest`)
look fixed in code. They can't be closed as fixed while the suite is red.

## Options for the flake

1. Raise the production bounded wait. Rejected. 10s is already long for an
   interactive gate, and real callers must fail closed rather than queue
   without limit.
2. Shorten the critical section, for example by caching the maximum counter.
   Rejected for CT. It changes the 106-F counter-lock contract and needs its
   own deliberation.
3. Fix the test to assert its real invariant (recommended). Accept
   `errors.Is(err, ErrGateInProgress)` as a valid per-goroutine outcome,
   fail on any other error, and require at least one success. The persisted
   `EventGatePassed` count must equal the number of successes, and no counter
   may repeat. Keep n=20 so the contention coverage stays the same.

## Decisions

* D1: adopt option 3 for 8F41D60D. This is a test-only change.
* D2: D8EF5443 adds one package-local helper that captures and restores the
  slog default, `log.Writer()`, `log.Flags()`, and `log.Prefix()`. It is used
  at all three sites.
* D3: 92AB6879. The server blocks until the request context is done (bounded
  at 2s). The test asserts `errors.Is(err, context.DeadlineExceeded)` and
  `elapsed < 1s`, which separates honoring the timeout from waiting for the
  server.
* D4: a time-boxed spike profiles internal/core test durations
  (`go test -json`), ranks the slowest tests, and records follow-up stash
  entries. CT does not change the timeout or split packages.
* D5: a verification task runs the full suite after the fixes. A green run is
  the evidence that DB48A817, FF1F3AC7, and 885263D2 are fixed. The three
  entries stay active, annotated with a pointer to that task. Stage archives
  them at the next triage that sees the green closure record.
* D6: 198-S (CT) blocks on 197-S (CX); 152-S retains its blocks edges to
  both CX and CT. Ship runs `go test ./...` before every PR. With an
  intermittent internal/core failure and 92% of the timeout budget used, 152-S
  (which adds tests under internal/events and internal/core) has a real chance
  of a red quality gate for reasons it does not own. T5 requires CX to have
  merged. The native `198-S blocks-on 197-S` edge keeps CT out of
  dependency-aware queue results until CX is resolved. Queue filtering accepts
  six terminal statuses: done, accepted, archived, shipped, abandoned, and
  rejected. It does not establish the after-merge guarantee, and
  `ClaimShipment` checks queued status and the active slot, not dependencies.
  Orchestrator/Ship MUST separately read 197-S from canonical Markdown
  (not the SQLite index, which does not project `archived_status`)
  immediately before claiming 198-S and require effective shipped provenance:
  `status: shipped`, or `status: archived` with `archived_status: shipped`
  (the normal post-`ShipShipment` form). Abandoned (live or
  archived-from-abandoned), rejected, any other terminal state, and missing
  or unparsable provenance FAIL CLOSED. queue_position 150 remains an ordering
  preference, not a readiness gate. No new edge involving 141-S is added.
  Correction authority: the operator's 2026-10-08 authorization for finding 2
  on merged PR #487 supersedes the earlier direction that CT had no blocker.

## Scope Boundaries

* In scope: 8F41D60D, D8EF5443, 92AB6879, the runtime spike, and the
  verification run.
* Out of scope: changes to production lock semantics, test timeout policy, and
  package splits. These are spike outputs for later triage.
* The native Go claim-time dependency guard is already tracked by queued
  feature 184-F (source stash 6434A4D7). This correction documents the existing
  pre-claim duty and adds no implementation scope or new stash entry.

## P-021 C5/C6 Records

### Duplicate Scan (A)

| Entry | Outcome |
|---|---|
| 8F41D60D | Clean scan. No active entry names this test or the gateproof counter lock. |
| D8EF5443 | Clean scan. |
| 92AB6879 | Clean scan. |

### Late-Identifier Reconciliation (B)

| Entry | Outcome |
|---|---|
| D8EF5443 | Refs concrete (62C6A469, 072-DL, 174.073-T). Nothing to reconcile. |
| 92AB6879 | No late identifier found. 153-S closure carries no thread for this flake. PR N/A stands. |
| 8F41D60D | Captured by Stage with full evidence. No source PR. |
