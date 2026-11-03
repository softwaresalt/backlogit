---
chunk_strategy: h1-h2-h3
description: 'Implementation plan for the 2026-10-08 Stage fold-ins: null-aware legacy-import checks under 171-F (97AB4978), claim crash recovery for cascaded non-queued member-parents under 189-F (E52607F5), and the CI evidence fold-in for the upstream pipeline-topology hand-off under 183-F (95379DBA).'
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-10-08-stage-foldins-plan.md
title: 'Implementation Plan: 2026-10-08 Stage fold-ins into 152-S, 189-S, and 183-S'
---

# Implementation Plan: 2026-10-08 Stage fold-ins

## Objective

Add the fold-in work decided in
`docs/decisions/2026-10-08-stage-foldins-and-dispositions.md` (F1-F3) to the
existing features and shipments. No new shipment is created.

## Problem Frame

* 97AB4978: in the legacy-import upgrade path that 171.002-T lands,
  `json.Unmarshal` turns present-but-null `status`, `created_at`, and
  `updated_at` into Go zero values, which the upgrade then defaults. The 171-F
  plan contract says present-but-null members are invalid, not defaultable.
* E52607F5: claim crash recovery rejects a mid-claim state in which the
  bounded parent cascade moved a non-queued member-parent to `active`.
* 95379DBA: new CI evidence of the upstream numeric-adjacency predecessor
  defect is not in the upstream hand-off document.

## Requirements Trace

| Req | Source | Requirement | Units |
|---|---|---|---|
| R1 | 97AB4978 | Present-but-null `status`, `created_at`, or `updated_at` in a legacy import is classified invalid, in both the upgrade and its dry-run | F1a, F1b |
| R2 | E52607F5 | Claim crash recovery accepts an unmarked `active` state for a member whose preimage is not `queued` and restores the exact preimage | F2a, F2b |
| R3 | 95379DBA | The upstream hand-off cites PR #485 and CI run 37728825570 job 113153060905 as further evidence | F3 |

## Implementation Units

### F1a: Null-aware legacy-import tests (RED)

Parent 171-F, shipment 152-S. Domain: tests. Posture: test-first. Depends on
171.002-T. File: `internal/events/checkpoint_create_schema_test.go`.

Add two scenarios to the legacy-import matrix: upgrade and dry-run. Each
scenario has three table rows (null `status`, null `created_at`, null
`updated_at`), and each row expects the invalid classification with no
checkpoint written. The test file is created by 171.002-T.

AC:

1. The new rows fail on the 171.002-T result and pass only after F1b.
2. Existing rows are unchanged. gofmt and vet are clean.

### F1b: Null-aware legacy-import fix

Parent 171-F, shipment 152-S. Domain: code. Depends on F1a. File:
`internal/events/memory.go`.

Detect key presence with a raw `map[string]json.RawMessage` pass before
defaulting. A present key whose value is JSON `null` is invalid. Only an
absent key is defaulted.

AC:

1. F1a passes. `go test -count=1 ./internal/events/...` passes.
2. Absent-field defaulting behavior is unchanged.

### F2a: Child-before-parent claim recovery regression test (RED)

Parent 189-F, shipment 189-S. Domain: tests. Posture: test-first. File:
`internal/core/shipment_claim_cascade_recovery_test.go` (new).

Build a shipment whose manifest lists a `done` member-parent after its
`queued` child. Simulate a claim crash after the child activates and the
cascade moves the parent to `active`, then run journal recovery. Scenarios:
recovery rolls back both to the exact preimage; a parent `active` with a
marker for a different shipment still fails closed; a committed-terminal
claim recovery leaves a non-cascaded `done` member at `done`.

AC:

1. The first scenario fails on `main` with `ErrShipmentConflict`. The second
   and third pass on `main` and must stay green after F2b.
2. One file, three scenarios; gofmt and vet clean.

### F2b: Claim recovery accepts cascaded non-queued member-parents

Parent 189-F, shipment 189-S. Domain: code. Depends on F2a. File:
`internal/core/shipment_recovery.go` (`memberRecoveryCandidates`).

In the `rollback` plus `claim` case, when the preimage is not `queued`, insert
one candidate between the existing first and last entries: the preimage with
`Status: active` and no marker change, `ignoreUpdatedAt: true`. The preimage
stays both first (compensated outcome) and last (committed outcome), because
`validateShipmentLifecycleRecoveryOutcome` reads `[0]` and the last entry.
Leave every other branch unchanged. Do not change `ClaimShipment` or the
marker producer.

AC:

1. F2a passes. `go test -count=1 ./internal/core/...` passes, including the
   173-F marker tests.
2. The scheduler-baseline marker contract tests in `tests/integration` stay
   green.
3. The task records that the bounded parent cascade writes only `status`
   (and `updated_at`) on a non-queued member-parent. If it writes any other
   field, the candidate is widened to match and the task notes say why.

### F3: Upstream hand-off CI evidence

Parent 183-F, shipment 183-S. Domain: docs. File:
`docs/decisions/2026-09-30-a592fc1c-pipeline-topology-dag-predecessor-upstream-handoff.md`.

Add an evidence entry: on PR #485 (153-S), the CI pipeline-topology ambient
job pinned to `autoharness==1.5.0` reported `PREDECESSOR_NOT_SHIPPED` for
153-S naming 152-S, although 153-S's only `item_deps` edge is 154-S
(shipped). The locally installed 1.5.0 passed the same run with
`predecessor_source=explicit`. Cite CI run 37728825570 job 113153060905 and
source stash 95379DBA. Note F88FE051 (CI pin bump) as related and out of
scope.

AC:

1. The hand-off names the run, job, PR, and stash ID.
2. 183.002-T (pre-delivery content review) blocks on this task.
3. Docline lint and P-008 markdownlint are clean for the file.

## Dependency Graph

```text
171.002-T -> F1a -> F1b
F2a -> F2b
F3 -> 183.002-T
```

## Decisions

See the deliberation F1-F3. The 173-F decision carries the E52607F5 amendment.

## Risks

* F2b widens the recovery candidate set. Mitigation: the added candidate
  requires a non-queued preimage, an unchanged marker, and status `active`.
  Rollback still restores the exact preimage.
* F1a and F1b depend on 171.002-T landing first in 152-S.

## Constitution Check

* Single domain and the 2-hour rule per unit: yes.
* Test-first for behavior changes: yes.
* No new shipments: yes.

Constitution Check: pass

## Plan Hardening Signals

* F2b changes crash-recovery acceptance (concurrency and recovery contract).

Requires plan hardening: yes

## Runtime Verification and Closure

The host shipments' closures (152-S, 189-S, 183-S) cover these units. The
189-S closure states that the E52607F5 amendment is in force.

## Plan Hardening

### Context consulted

* `docs/decisions/2026-09-28-173f-marker-lifecycle-option-a-decision.md`
  producer rules and the E52607F5 amendment.
* `internal/core/shipment_recovery.go` `memberRecoveryCandidates`.

### Protected invariants

* Only `ClaimShipment` writes the marker; non-queued preimages stay
  byte-identical apart from the cascade status.
* Recovery still fails closed on a marker for a different shipment.

### Risky actions

* F2b: recovery acceptance. Mitigation: F2a scenario 2 pins fail-closed
  behavior.

### Added verification

* F2b AC includes the marker contract tests.

### Closure, monitoring, and rollback

* Rollback is a revert of the host shipment merge.

### Review-gate capability risks

* None beyond the host shipments' gates.

### Unresolved operator decisions

* None.

## Plan Review

* dispatch_mode: multi-agent-dispatch
* decision: ADVISORY
* attempt: 1
* reviewers: Constitution Reviewer, Go Reviewer, Scope Boundary Auditor, Architecture Strategist
* findings and revisions: F2b candidate now inserted between first and last entries so the preimage stays the committed outcome for untouched members (Go Reviewer P1), with an F2a committed-terminal scenario; F2b AC records the fields the cascade writes; F1b wrong-JSON-type clause dropped (outside 97AB4978); F1a framed as two scenarios and notes that 171.002-T creates the test file.
* operator_authorization: approved (operator APPROVED this scope and directed autonomous work without routine confirmations, relayed by the Orchestrator on 2026-10-08)