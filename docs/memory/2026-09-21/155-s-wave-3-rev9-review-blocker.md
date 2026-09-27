---
title: 155-S Wave 3 rev9 review blocker
date: 2026-09-21
agent: ship
status: blocked
shipment: 155-S
branch: feat/155-s-s14-resumable-shipment-blocked-lifecycle-status
head: 0c995ffde583785b73bd0f1a37705b2b23d7ad15
---

## Outcome

The four preserved Wave 3 harness files were reconciled to the reviewed rev9
contracts. Compile, vet, CI-pinned lint, formatting, and the three expected RED
selectors passed. Fresh report-only Go and correctness reviews still returned
P1 findings, so Ship halted without committing the harnesses or changing task
statuses.

## Completed tasks

* `174.039-T`
* `174.052-T`

## Preserved files

* `internal/core/shipment_blocked_harness_helpers_test.go`
* `internal/core/shipment_blocked_lifecycle_harness_test.go`
* `internal/core/shipment_blocked_recovery_harness_test.go`
* `internal/core/shipment_blocked_writer_harness_test.go`

## Validation

* `git diff --check`: PASS
* Modified-file `gofmt`: PASS
* `go test -run=^$ -count=1 ./...`: PASS
* `go vet ./...`: PASS
* Go 1.24 with `golangci-lint@v1.64.8`: PASS
* `go test -count=1 -run '^TestUR1B_' ./internal/core`: expected named RED
* `go test -count=1 -run '^TestUR2_' ./internal/core`: expected named RED
* `go test -count=1 -run '^TestUR3_' ./internal/core`: expected named RED

## Blocking review findings

### P1: incomplete governed transition matrix

`shipment_blocked_writer_harness_test.go`, `urWriterTransitions` covers only
active-to-blocked and blocked-to-queued or active. The contract guards every
write whose old status is blocked or new status is blocked. The matrix must
cover all distinct blocked ingress and egress cases accepted by each surface,
while `MoveShipmentStatus` must return
`ErrShipmentBlockedRequiresEnvelope` before ordinary transition validation.

### P1: bulk result semantics discarded

`shipment_blocked_writer_harness_test.go`, the `BulkUpdateStatus` adapter
checks only the top-level error. `BulkUpdateStatus` can report item failures in
`BulkUpdateResult.Failed` with a nil error. The harness must accept the
repository's established item-level refusal result while still proving no
aggregate mutation.

### P1: member drift and CAS refusal missing

`shipment_blocked_lifecycle_harness_test.go`,
`TestUR1B_UnblockIsTargetAwareAndPreservesResumptionEvidence` does not prove
that ordinary member mutation is refused while a parent shipment is blocked,
or that unblock-to-active refuses an out-of-band member drift while preserving
the drifted aggregate and audit state.

### P1: lifecycle serialization remains scheduler-dependent

`shipment_blocked_lifecycle_harness_test.go`,
`TestUR1B_FailClosedValidationAndAuthoritativeActiveSlot` releases claim and
unblock concurrently but does not pause one operation after the authoritative
slot read and before persistence. A lock-free check-then-write implementation
can be scheduled serially and pass. The R1b lifecycle contract needs an
operation-owned deterministic barrier for workspace-handle and cross-process
cases. This finding is separate from the rev9 exclusion of recovery contention
from R3.

### P1: recovery evidence may predate reopen

`shipment_blocked_recovery_harness_test.go`,
`requireUR3CorrelatedTerminalEvidence` scans the complete event history. It
does not prove that the correlated `shipment_status_changed` and terminal
committed or compensated events were appended during recovery. The crash
fixture must capture a pre-reopen event boundary and require the evidence in
the post-reopen suffix.

## Non-blocking review findings

The reviewers also reported P2 gaps in unblock audit payload ordering,
terminal journal immutability, and exact returned artifact assertions. These
remain unresolved because P1 findings already block the harness gate.

## Decisions

* Do not accept residual P1 risk
* Do not commit or complete Wave 3 harness tasks
* Do not change shipment-member statuses
* Do not claim or mutate `154-S`
* Do not modify PR #449
* Preserve all four harness files for a governed contract or testability
  amendment

## Next action

Stage must decide ownership and provide observable seams for the deterministic
R1b serialization proof, then review a bounded contract amendment covering the
remaining transition, bulk-result, member-drift, and recovery-suffix evidence.
Ship may resume only after that amendment is reviewed and explicitly handed
back.
