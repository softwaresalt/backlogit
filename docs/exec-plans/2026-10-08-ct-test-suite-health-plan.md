---
chunk_strategy: h1-h2-h3
description: 'Implementation plan for CT test-suite health: stabilize the gate-evidence counter concurrency test, fix the slog/log capture leak, stabilize the release client timeout test, spike the internal/core runtime budget, and verify the full suite green.'
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-10-08-ct-test-suite-health-plan.md
title: 'Implementation Plan: CT test-suite health'
---

# Implementation Plan: CT test-suite health

## Objective

Return `go test ./... -count=1` to green on the Windows host, with useful
diagnostics, and measure the internal/core runtime budget. Source:
`docs/decisions/2026-10-08-ct-test-suite-health-deliberation.md`.

## Problem Frame

See the deliberation. The full-suite run on 2026-10-08 failed only on a
load-sensitive concurrency test. internal/core used 92% of the timeout, and
earlier diagnostics were lost to a log-capture leak.

## Requirements Trace

| Req | Source | Requirement | Units |
|---|---|---|---|
| R1 | 8F41D60D | The counter concurrency test asserts uniqueness, not universal success | T1 |
| R2 | D8EF5443 | Log capture restores slog and log state | T2 |
| R3 | 92AB6879 | The release timeout test is independent of load | T3 |
| R4 | 072-DL context | The internal/core runtime budget is measured and follow-ups are captured | T4 |
| R5 | DB48A817, FF1F3AC7, 885263D2 | A full-suite green run is recorded | T5 |

## Implementation Units

### T1: Stabilize the gate-evidence counter concurrency test

Domain: tests. Posture: characterization-first. Stash 8F41D60D. File:
`internal/core/gate_evidence_formal_test.go`
(`TestAppendGateEvidence_ConcurrentSameItem_NoDuplicateCounters`).

Keep n=20. Treat `errors.Is(err, bkerrors.ErrGateInProgress)` (package
`internal/errors`) as a valid outcome; confirm the formal-required wrapper
keeps the `%w` chain. Any other error fails. Require at least one success and
a persisted `EventGatePassed` count equal to the number of successes, with no
duplicate counter.

AC:

1. `go test -count=30 -run TestAppendGateEvidence_ConcurrentSameItem ./internal/core`
   passes, and still passes with `-cpu 1`.
2. Reverting the 106-F unlock-after-append fix still fails the test on the
   duplicate-counter assertion (record this in the task).
3. No production file changes.

### T2: Restore slog and log state after log capture

Domain: tests. Posture: test-first. Harness: ready (red assertion is AC1).
Stash D8EF5443. Files:
`internal/core/gate_baseref_warn_test.go`,
`internal/core/shipment_shipped_event_durability_test.go`.

Add one helper in `gate_baseref_warn_test.go` (package `core`) that captures
`slog.Default()`, `log.Writer()`, `log.Flags()`, and `log.Prefix()` and
restores them in `t.Cleanup`: slog first, then `log.SetOutput`, `SetFlags`,
and `SetPrefix`, so a non-default previous slog handler can't re-redirect
`log`. Use it at the three sites (lines 40, 64, and 395 at d2f3ff97). These
are the only `slog.SetDefault` sites in internal/core tests at d2f3ff97.

AC:

1. After a capture site's cleanup, `log.Writer()`, `log.Flags()`, and
   `log.Prefix()` equal the values saved before the capture. This fails on
   `main`.
2. `go test -count=1 ./internal/core -run 'BaseRef|ShippedEventDurability'`
   passes.

### T3: Stabilize TestClientLatestRespectsContextTimeout

Domain: tests. Posture: test-first. Harness: ready (red assertion is the
`errors.Is(err, context.DeadlineExceeded)` check). Stash 92AB6879. File:
`internal/release/release_test.go`.

The server handler waits on `r.Context().Done()` or 2s. The test asserts
`errors.Is(err, context.DeadlineExceeded)` and `elapsed < 1s`.

AC:

1. `go test -count=200 -run TestClientLatestRespectsContextTimeout ./internal/release`
   passes.
2. Removing the context propagation in `Client.Latest` still fails the test.

### T4: internal/core runtime budget spike

Domain: spike (docs output). Context 072-DL. Output:
`docs/decisions/2026-10-08-ct-core-runtime-budget-spike.md`.

Run `go test -json -count=1 ./internal/core` on the Windows host. Rank the 25
slowest tests and classify each (real subprocess, recovery-enabled fixture,
fsync-heavy, or sleep or poll). Recommend follow-ups and capture each as a
stash entry. Time box: 2 hours. No code changes.

AC:

1. The findings list total package time, the top 25 with classifications, and
   the stash IDs captured.

### T5: Full-suite green verification

Domain: verification-only. Depends on T1, T2, and T3. T4 is independent;
T4 may reuse T5's run by adding `-json` for internal/core instead of running
a second 27-minute package run.

Run `go test ./... -count=1 -timeout 30m` on the Windows host after CX has
merged, and record the per-package times in the CT closure. The closure
states that the DB48A817, FF1F3AC7, and 885263D2 isolation fixes held, which
is the evidence Stage uses to archive them.

Red-run policy: if one package fails on a test outside CT's scope, rerun that
package once. If it passes, record both runs and treat the suite as green
with the flake captured as a new stash entry. If it fails again, capture a
stash entry, record CT closure as verification-incomplete, and keep the three
entries active. CT does not widen to fix it.

AC:

1. EXIT=0 (or a green rerun under the red-run policy), with the internal/core
   time recorded.
2. The closure names the three stash IDs and the run evidence path.

## Dependency Graph

```text
T1, T2, T3 -> T5; T4 independent
Shipments: 198-S (CT) blocks-on 197-S (CX)
152-S retains blocks-on 197-S (CX) and 198-S (CT)
```

## Decisions

* D1-D6 are in the deliberation. queue_position is 150.
* The operator-authorized PR #487 finding-2 correction makes the existing
  T5 after-CX-merge requirement an explicit shipment prerequisite.
  queue_position alone is not evidence of dependency readiness.

## Risks

* T1 could weaken the test so that it no longer detects the duplicate-counter
  bug. Mitigation: T1 AC2 (revert check).
* T5 can fail on a new flake. Mitigation: capture it as a stash entry, and
  don't widen CT.

## Constitution Check

* One domain and the 2-hour rule per unit: yes.
* No production behavior change: yes.

Constitution Check: pass

## Plan Hardening Signals

* Concurrency test semantics (T1).

Requires plan hardening: yes

## Runtime Verification and Closure

T5 is the runtime verification. The closure attaches the run log path and
per-package times.

## Plan Hardening

### Context consulted

* `internal/core/gate_evidence_formal.go` `nextGateEvidenceCounter` and the
  106-F F1 finding in the test comment.
* 072-DL and 174.073-T.

### Protected invariants

* No duplicate gate-evidence counter is ever persisted.
* `ErrGateInProgress` remains the fail-closed result of a bounded-wait
  timeout.

### Risky actions

* T1 loosening an assertion. Mitigated by AC2.

### Added verification

* Repeat counts in the T1 and T3 AC.

### Closure, monitoring, and rollback

* Rollback is a revert of the CT merge.

### Review-gate capability risks

* None.

### Unresolved operator decisions

* None.

## Plan Review

* dispatch_mode: multi-agent-dispatch
* decision: ADVISORY
* attempt: 1
* reviewers: Constitution Reviewer, Go Reviewer, Scope Boundary Auditor, Architecture Strategist
* findings and revisions: T2 and T3 declare harness-ready red assertions; T2 helper location and restore order fixed, AC compares against saved values; T1 names bkerrors.ErrGateInProgress; T5 no longer waits on T4 and has a red-run policy; T5 runs after CX merges.
* residual advisory disposition: Architecture Strategist's CT blocks-on CX
  recommendation is now adopted under the operator's 2026-10-08 PR #487
  finding-2 correction authorization. The native `198-S blocks-on 197-S`
  edge withholds CT from dependency-aware queue results until CX is resolved,
  but queue filtering accepts done, accepted, archived, shipped, abandoned,
  and rejected. `ClaimShipment` does not check dependencies.
  Orchestrator/Ship MUST separately verify that 197-S is exactly `shipped`
  immediately before claiming 198-S; abandoned or any other terminal status
  does not satisfy T5's after-CX-merge requirement. Queue positions and
  shipment membership remain unchanged. No implementation scope is added.
* operator_authorization: approved (operator APPROVED this scope and directed autonomous work without routine confirmations, relayed by the Orchestrator on 2026-10-08)