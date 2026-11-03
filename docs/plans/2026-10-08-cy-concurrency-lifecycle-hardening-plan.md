---
chunk_strategy: h1-h2-h3
description: 'Implementation plan for CY concurrency and lifecycle hardening, split into three wave-aligned shipments: CY-A error classification and blocked-envelope boundary, CY-B lock and event ordering plus declaration accuracy, CY-C filesystem TOCTOU and parser hardening.'
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-10-08-cy-concurrency-lifecycle-hardening-plan.md
title: 'Implementation Plan: CY concurrency and lifecycle hardening'
---

# Implementation Plan: CY concurrency and lifecycle hardening

## Objective

Close the 13 CY deferred-scope expansions accepted in
`docs/decisions/2026-10-08-cy-concurrency-lifecycle-hardening-deliberation.md`
in three shipments that run after CX: CY-A, then CY-B, then CY-C.

## Problem Frame

Previous lifecycle and lock-order shipments (153-S, 155-S, 174-F) left
documented residuals. These include:

* Error classes that the MCP layer can misreport.
* Compensation that can run after committed bytes may have landed.
* A blocked-envelope key list with several copies.
* Event appends ordered differently from their causal mutation.
* Side effects that run after the locks are released.
* Faultline declarations that overstate the event postconditions.
* Filesystem reads and writes that pass a containment check and then use a
  different path-based operation.

## Requirements Trace

| Req | Stash | Requirement | Units |
|---|---|---|---|
| R1 | 7D8717B1 | MCP classifies ErrWriteIndeterminate before not_found, conflict, and validation; core returns to `%w`; BlockShipment marks the mutation applied before each member write; a compensation double fault is write_indeterminate | A1-A4 |
| R2 | FA6AE139 | `appendFast` failures are classified: ErrWriteIndeterminate after bytes, ErrWriteNotApplied before | A5, A6 |
| R3 | 8AF55264 | One canonical blocked-envelope key list; ungoverned artifact_type flips into or out of shipment are refused while either side is blocked | A7-A9 |
| R4 | 5247D4BC | Reconciliation and membership mutation cannot interleave | B1-B3 |
| R5 | E4908F30 | ShipShipment never acquires C while holding B; it takes C before B for its archive set | B4, B5 |
| R6 | BACD94FC | Archive events keep causal order with concurrent mutations | B6, B7 |
| R7 | DBF89C9C | Post-archive hook and stash side effects are CAS-guarded | B8, B9 |
| R8 | AB31C9C5, 010F437C | CreateItem and ArchiveItem declarations match the actual EventsJSONL behavior | B10, B11a, B11b |
| R9 | A0C733C6 | Manifest member reload reads through the contained handle it validated | C1, C2a, C2b |
| R10 | E45E6D65 (37699341) | No payload byte reaches a swapped-in outside location on Windows | C3-C5 |
| R11 | 0FFBF819 | Blocked-shipment snapshot read is handle-relative and no-follow | C6, C7 |
| R12 | 45BD3B36 | Duplicate JSON member names in a snapshot are rejected before decode | C8, C9 |

## Implementation Units

### Wave CY-A: error classification and blocked-envelope boundary

#### A1: MCP error classification ordering tests (RED)

Domain: tests. Posture: test-first. Stash 7D8717B1. File:
`internal/mcp/error_mapping_test.go`.

AC:

1. An error that wraps both ErrWriteIndeterminate and ErrShipmentConflict or
   ErrNotFound maps to `write_indeterminate`. The test fails on `main`.
2. A second scenario checks `errors.Is(err, ErrWriteIndeterminate)` on the
   real U20C3 and U20C5 error values returned through the MCP handler. It
   fails while core still renders them with `%v`.

#### A2: Classify ErrWriteIndeterminate first and restore `%w`

Domain: code. Depends on A1. Files: `internal/mcp/errors.go`,
`internal/core/shipment.go`.

Reorder `domainError` so ErrWriteIndeterminate is checked first within the
`switch`, after the existing `MutationPartialError` branch. Replace the
EW-1 `%v` rendering at the U20C3 and U20C5 sites with `%w`.

AC:

1. A1 passes. `go test -count=1 ./internal/mcp/... ./internal/core/...`
   passes.

#### A3: BlockShipment write accounting tests (RED)

Domain: tests. Posture: test-first. Stash 7D8717B1. File:
`internal/core/shipment_block_accounting_test.go` (new).

Scenarios: relocation old-file removal fails after the new file landed; a
compensation restore fails and leaves the journal at intent.

AC:

1. The relocation scenario reports write_indeterminate. The compensation
   double-fault scenario reports write_indeterminate. Both fail on `main`.

#### A4: Mark mutation applied per member and classify double faults

Domain: code. Depends on A3 and A2 (same file). File:
`internal/core/shipment.go`.

AC:

1. A3 passes. The existing BlockShipment and UnblockShipment suites pass.

#### A5: `appendFast` failure classification tests (RED)

Domain: tests. Posture: test-first. Stash FA6AE139. File:
`internal/events/stream_append_fast_classify_test.go` (new).

AC:

1. With durable writes off, an append to an item log whose open fails (for
   example a read-only log path) returns an error that satisfies
   `errors.Is(err, ErrWriteNotApplied)`. The test fails on `main`, where the
   error is unclassified. Short-write classification is already covered by
   the `syncAppendLineDetailed` tests that A6 reuses.

#### A6: Classify `appendFast` failures

Domain: code. Depends on A5. File: `internal/events/stream.go`
(`EventWriter.appendFast`).

Route `appendFast` through the existing `syncAppendLineDetailed` with a no-op
sync. That path already separates pre-write and post-write failures. Stop
discarding the `Close()` error.

AC:

1. A5 passes. Compensate in BlockShipment and UnblockShipment returns early
   for the indeterminate case through the existing `IsWriteIndeterminate`
   branch.
2. `go test -count=1 ./internal/events/... ./internal/core/...` passes.

#### A7: Canonical blocked-envelope key list tests (RED)

Domain: tests. Posture: test-first. Stash 8AF55264. File:
`internal/core/shipment_blocked_envelope_keys_test.go` (new).

AC:

1. One test asserts every consumer uses the canonical list. Another asserts
   that an ungoverned `WriteArtifactFile*` call flipping artifact_type into or
   out of `shipment` while blocked is refused. Both fail on `main`.

#### A8: Consolidate the blocked-envelope key list

Domain: code. Depends on A7. Files:
`internal/core/shipment_blocked_envelope.go`, `internal/core/artifacts.go`.

One unexported list is consumed by validation, BlockShipment, and
shipmentRecoveryCandidates. Pre-update hooks keep seeing the submitted
request. Record that decision in a code comment.

AC:

1. The A7 key-list test passes.

#### A9: Refuse artifact_type flips across a blocked envelope

Domain: code. Depends on A8. File: `internal/core/artifacts.go`.

AC:

1. The A7 flip test passes. `go test -count=1 ./internal/core/...` passes.

### Wave CY-B: lock and event ordering plus declaration accuracy

#### B1: Reconciliation versus membership interleaving characterization

Domain: tests. Posture: characterization-first. Stash 5247D4BC. File:
`internal/core/shipment_reconcile_membership_interleave_test.go` (new).

Pin today's behavior when reconciliation closure runs concurrently with
`add_to_shipment` or a membership removal.

AC:

1. The test documents the interleaving. A skipped-until-B3 assertion states
   the target: closure observes a consistent membership snapshot.

#### B2: Lock-domain decision note

Domain: docs. Depends on B1. File:
`docs/design-docs/artifact-mutation-lock-order.md`.

Record the decision that B1's evidence supports (deliberation D3 split the
note out of the characterization task). If reconciliation must take the
membership lock domain, record where that lock sits in the order (lifecycle
global, then A, then C, then B) and the deadlock argument. If B1 shows the
interleaving is already prevented, record that and B3 is closed as not
needed. Also record the archive event mechanism chosen for B5 and B7: archive
writers take C (sorted by item ID over the archive set) before B, and append
the archive event under C after the mutation. A sequence stamp was rejected
because nothing in the event writer or readers would consume it.

AC:

1. The note names the decision, the lock position (when adopted), and the
   deadlock argument. Docline and P-008 lint are clean.

#### B3: Bind reconciliation to the membership lock domain

Domain: code. Depends on B2. File:
`internal/core/shipment_reconcile.go` (closure entry point).

AC:

1. The B1 target assertion is enabled and passes.
2. A forced interleaving using the existing barrier hooks
   (`artifactMutationLockBarrierHook`, `itemLogLockBarrierHook`) passes. The
   design doc says `-race` alone is not proof.

#### B4: ShipShipment archive lock-order tests (RED)

Domain: tests. Posture: test-first. Stash E4908F30. File:
`internal/core/ship_shipment_lock_order_test.go` (new).

AC:

1. A lock-order probe built on the existing test-only barrier hooks
   (`artifactMutationLockBarrierHook`, `itemLogLockBarrierHook`) shows no
   item-log lock C acquired while batch B locks are held during
   ShipShipment. No new production hook is added. The test fails on `main`.

#### B5: ShipShipment takes C before B for its archive set

Domain: code. Depends on B4 and B2. Files: `internal/core/shipment.go` (the
ShipShipment archive loop) and `internal/core/archive.go` (reuse of a
context-held C, like the existing context-held B reuse).

Under the lifecycle global lock, ShipShipment acquires C for every archive-set
item in sorted item-ID order before it takes batch B. It appends each archive
event under C after the mutation, then releases B, then C. This follows the
canonical order (C before B) that AssociateCommit and the reconcile-to-shipped
transaction already use.

AC:

1. B4 passes. Shipped-event durability tests pass.
2. ShipShipment's C set covers every nested archive scope, including cascade
   descendants, so a nested ArchiveItem reuses the context-held C and never
   acquires C itself.

#### B6: Archive event causal order tests (RED)

Domain: tests. Posture: test-first. Stash BACD94FC. File:
`internal/core/archive_event_order_test.go` (new).

AC:

1. For standalone and cascade ArchiveItem, a concurrent C-then-B writer (for
   example AssociateCommit) cannot append its event between the archive
   mutation and the archive event. The forced interleaving uses the existing
   barrier hooks; no new production hook is added. The test fails on `main`.

#### B7: Standalone and cascade archive take C before B

Domain: code. Depends on B6 and B5 (same file). File:
`internal/core/archive.go`.

Standalone and cascade ArchiveItem acquire C for the archive set in sorted
item-ID order before batch B, append the archive events under C after the
mutation, then release B, then C. The canonical order is global, then A
(membership locks from `lockArchiveShipmentMemberships`), then C, then B.
Insert C between the membership locks and `lockArtifactMutations`, computed
from the same scope-ID snapshot used for B. The deferred event sink is retired
for these paths. Appending under C while B is held after taking B first is not
allowed; it would bring back the B-then-C overlap that 172.002-T removed.

AC:

1. B6 passes. A forced interleaving using `artifactMutationLockBarrierHook`
   and `itemLogLockBarrierHook` shows no B/C overlap.
2. A request for C while B is held and that C is not context-held fails
   closed with an error instead of acquiring C.
3. `go test -count=1 ./internal/core/... ./internal/events/...` passes.

#### B8: Post-archive side-effect CAS tests (RED)

Domain: tests. Posture: test-first. Stash DBF89C9C. File:
`internal/core/archive_side_effect_cas_test.go` (new).

AC:

1. A concurrent writer between archive commit and the stash state update
   causes the stale side effect to be refused. The test fails on `main`.

#### B9: CAS-guard post-archive stash side effects

Domain: code. Depends on B8 and B7. File: `internal/core/archive.go`.

AC:

1. B8 passes. Hook-queue append failure handling is unchanged (still
   non-fatal).

#### B10: Declaration accuracy tests (RED)

Domain: tests. Posture: test-first. Stashes AB31C9C5 and 010F437C. File:
`internal/faultline/mutation/verify_test.go`.

AC:

1. VerifySuccess accepts a successful CreateItem with no item event and a
   successful ArchiveItem whose event append degraded to a warning. The test
   fails on `main`.
2. VerifyFailure still reports event-log drift when a conditional EventsJSONL
   representation changes on a failed mutation. This passes on `main` and must
   stay green.

#### B11a: Conditional representation kind

Domain: code. Depends on B10 and B9. Files:
`internal/faultline/mutation/representations.go`,
`internal/faultline/mutation/verify.go`.

Add a conditional (optional-on-success) marker to `RepresentationSet`.
VerifySuccess does not require a conditional representation to change.
VerifyFailure still checks it for drift.

AC:

1. B10 scenario 2 stays green. The faultline suites pass.

#### B11b: Mark EventsJSONL conditional for CreateItem and ArchiveItem

Domain: code. Depends on B11a. File:
`internal/faultline/mutation/declarations.go`.

AC:

1. B10 scenario 1 passes. The faultline parity and declaration suites pass.

#### B12: Lock-order residuals closed

Domain: docs. Depends on B2, B5, B7, and B9. File:
`docs/design-docs/artifact-mutation-lock-order.md`.

AC:

1. The documented ShipShipment B-across-C residual, the archive event ordering
   exception, and the side-effect residual are marked closed, citing their
   task IDs. Lint is clean.

### Wave CY-C: filesystem TOCTOU and parser hardening

#### C1: Manifest member reload TOCTOU tests (RED)

Domain: tests. Posture: test-first. Stash A0C733C6. Files:
`internal/core/shipment_reconcile_preconditions_toctou_test.go` (new) and a
test-only seam variable (nil in production) in
`internal/core/shipment_reconcile_preconditions.go`.

AC:

1. A swap through the seam between the containment check and the read makes
   validation read the substituted file on `main`, so the test fails. After C2
   the substituted file is never read.
2. On Windows the test runs, not skips. A skip must state its reason (for
   example, missing symlink privilege), and the CY-C closure shows a run.

#### C2a: Handle-contained member read helper

Domain: code. Depends on C1. Files:
`internal/core/shipment_reconcile_member_read_windows.go` (new) and
`internal/core/shipment_reconcile_member_read_other.go` (new, `!windows`).

Open the member file once, check containment on that handle (Windows:
`GetFinalPathNameByHandle`; elsewhere the existing no-follow open), and
return the bytes read from the same handle. The existing Windows
`readFileNoFollow` checks only the leaf, so it is not enough on its own. Do
not change the shared `findArtifact`.

AC:

1. Both files build on their platforms (`GOOS=linux go vet ./internal/core`
   and the Windows build pass).

#### C2b: Preconditions read validated members through the helper

Domain: code. Depends on C2a. File:
`internal/core/shipment_reconcile_preconditions.go`.

AC:

1. C1 passes. `go test -count=1 ./internal/core/...` passes.

#### C3: Windows directory-relative creation spike

Domain: spike (docs output). Stash E45E6D65. Output:
`docs/decisions/2026-10-08-cy-c-windows-relative-create-spike.md`.

Compare `NtCreateFile` with a root directory handle against staging writes in
a verified in-workspace temp file, then renaming by handle. Time box: 2 hours.

AC:

1. The findings name the chosen approach, the evidence, and the residual risk.
2. The findings state how the target directory is pinned (a rename by handle
   still names its target directory by path).
3. If no approach removes the payload leak, C4 and C5 are re-planned through
   Stage before they are claimed.

#### C4: Windows archive write TOCTOU no-leak test (RED)

Domain: tests. Posture: test-first. Depends on C3. File:
`internal/core/shipment_reconcile_fs_windows_test.go`.

Flip `TestWriteShipmentReconcileArchiveFileHandleRelative_TOCTOUSwapLeaksRealPayload`
to assert that no payload byte reaches the outside location.

AC:

1. The test fails on `main` on Windows. It is skipped elsewhere by its build
   tag. The CY-C closure shows it ran, not skipped, on Windows.

#### C5: Windows reconcile writers use the spike's approach

Domain: code. Depends on C4. Files:
`internal/core/shipment_reconcile_fs_windows.go`,
`internal/core/shipment_reconcile_append_windows.go`.

AC:

1. C4 passes on Windows. The existing reconcile writer suites pass.

#### C6: Handle-relative snapshot read tests (RED)

Domain: tests. Posture: test-first. Stash 0FFBF819. File:
`internal/core/shipment_recovery_snapshot_nofollow_test.go` (new).

AC:

1. A symlink or reparse-point swap of a snapshot path component is refused.
   The test fails on `main`. On Windows it runs, not skips; a skip states its
   reason.

#### C7: Handle-relative no-follow snapshot read

Domain: code. Depends on C6. File: `internal/core/shipment_recovery.go`
(`readShipmentBlockedSnapshot`).

AC:

1. C6 passes. 174.047-T recovery tests pass.

#### C8: Duplicate JSON member rejection tests (RED)

Domain: tests. Posture: test-first. Stash 45BD3B36. File:
`internal/core/shipment_recovery_snapshot_dupkey_test.go` (new).

AC:

1. A snapshot with a duplicate top-level or nested member name is rejected,
   including names that differ only in case (`encoding/json` matches field
   names case-insensitively). The test fails on `main`.

#### C9: Token-level duplicate-member pre-scan

Domain: code. Depends on C8 and C7. File:
`internal/core/shipment_recovery.go` (`readShipmentBlockedSnapshot`).

AC:

1. C8 passes. Valid snapshots decode unchanged.
2. The pre-scan runs on the same bytes C7 read through the verified handle.
   There is no second read by path.

## Dependency Graph

```text
Shipments: CY-A blocks-on CX; CY-B blocks-on CY-A; CY-C blocks-on CY-B;
           189-S blocks-on CY-A
CY-A: A1->A2->A4; A3->A4; A5->A6; A7->A8->A9
CY-B: B1->B2->B3; B2->B5; B4->B5->B7; B6->B7->B9; B8->B9->B11a->B11b;
      B10->B11a; B2,B5,B7,B9->B12
CY-C: C1->C2a->C2b; C3->C4->C5; C6->C7->C9; C8->C9
```

## Decisions

* D1-D7 are in the deliberation.
* Queue positions: CY-A 450 (after 152-S at 400, before 189-S), CY-B 1400,
  and CY-C 1500.

## Risks

* Lock-domain changes (B3, B7) can deadlock. Mitigation: B2 records the order
  before any code change, and B3 and B7 are proven with forced interleavings on the existing barrier hooks.
* Windows-only behavior (C4, C5) cannot be checked on Linux CI. Mitigation:
  Windows-tagged tests, and the Windows host runs the suite at closure.
* Changes to error classes (A2, A6) affect MCP envelopes. Mitigation: A1 and
  the parity suite pin the envelopes.

## Constitution Check

* One domain and the 2-hour rule per unit: yes.
* Test-first or characterization-first before each behavior change: yes.
* Security-sensitive surfaces carry a spike and fail-closed tests: yes.

Constitution Check: pass

## Plan Hardening Signals

* Lock ordering, crash compensation, and filesystem containment changes.
* Cross-shipment sequencing with 189-S.

Requires plan hardening: yes

## Runtime Verification and Closure

Each wave closes with a full `go test ./... -count=1` on the Windows host and
the Linux CI matrix. The CY-B closure states the lock order recorded in B2.
The CY-C closure attaches the C3 findings.

## Plan Hardening

### Context consulted

* `docs/design-docs/artifact-mutation-lock-order.md`.
* The 153-S and 155-S closure residuals, and the CX plan.

### Protected invariants

* Lock order from `docs/design-docs/artifact-mutation-lock-order.md`: the
  shipment lifecycle global lock first where used, then A (shipment
  membership locks), then C (item-log lock), then B (artifact-mutation lock).
  No writer acquires C while it holds B for the same item. B2 records this
  order and places the reconciliation membership lock domain in it.
* Compensation never appends a compensated terminal after possibly-landed
  committed bytes.
* Workspace containment holds through every read and write in scope.

### Risky actions

* B3 and B7 change lock acquisition. Each is gated by a RED or
  characterization test and the B2 note.
* C5 changes Windows write primitives, gated by the C3 spike.

### Added verification

* Forced barrier-hook interleavings for B3 and B7 (`-race` is supplementary,
  not proof).
* Windows-tagged C4 at closure.

### Closure, monitoring, and rollback

* Each wave is one PR, so rollback is a revert of that wave's merge.

### Review-gate capability risks

* CY-C needs security review (Security Reviewer persona) at the Ship review
  gate.

### Unresolved operator decisions

* None. B5 and B7 use C-before-B (no sequence stamp), recorded in B2.

## Plan Review

* dispatch_mode: multi-agent-dispatch
* decision: FAIL
* attempt: 1
* reviewers: Constitution Reviewer, Go Reviewer, Scope Boundary Auditor, Architecture Strategist
* findings: protected invariant stated lock order opposite to artifact-mutation-lock-order.md; B7 allowed B-then-C overlap; -race used as proof; same-file units without edges (A2/A4, B2/B12); B11 could not express a conditional representation; A5 had no seam; C2 Windows read checked only the leaf; Windows tests could skip silently; C9 could re-read by path; C8 case-insensitive aliasing.

<!-- plan-review-attempt: 1 -->

## Plan Review

* dispatch_mode: multi-agent-dispatch
* decision: FAIL
* attempt: 2
* reviewers: Architecture Strategist (re-review of revisions)
* findings: C2 adding only a Windows file would break other builds; B7 sequence stamp had no consumer; stale unresolved-decision line.

<!-- plan-review-attempt: 2 -->

## Plan Review

* dispatch_mode: multi-agent-dispatch
* decision: ADVISORY
* attempt: 3
* reviewers: Architecture Strategist (re-review of revisions); prior rounds Constitution Reviewer, Go Reviewer, Scope Boundary Auditor, Architecture Strategist
* revisions: C2 split into C2a (Windows and non-Windows helper) and C2b; B5 and B7 adopt canonical C-before-B with sorted multi-item C acquisition (deadlock-free per reviewer); canonical order recorded as global, A, C, B; B5 AC requires ShipShipment's C set to cover nested archive scopes; B7 AC fails closed on C requested while B held.
* residual advisories: none blocking.
* operator_authorization: approved (operator APPROVED this scope and directed autonomous work without routine confirmations, relayed by the Orchestrator on 2026-10-08)