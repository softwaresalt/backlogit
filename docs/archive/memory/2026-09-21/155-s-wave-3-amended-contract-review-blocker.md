# Shipment 155-S — Amended Wave 3 Contract Review Blocker

## State

- Branch: `feat/155-s-s14-resumable-shipment-blocked-lifecycle-status`
- Stage amendment HEAD: `8ebb1340bb8e4367aa9e44c9b83a2d31e2966881`
- Restored checkpoint: `checkpoint-20260921-194021.json`
- Shipment: `155-S` (`active`)
- Completed tasks: `174.039-T`, `174.052-T`
- Current wave: `174.040-T`, `174.041-T`, `174.053-T`
- `154-S`: `queued`, not claimed or mutated
- PR #449: untouched

## Reconciliation and validation

The four preserved harness files were reconciled only to the Stage-reviewed verification-contract
amendment:

- `internal/core/shipment_blocked_harness_helpers_test.go`
- `internal/core/shipment_blocked_lifecycle_harness_test.go`
- `internal/core/shipment_blocked_recovery_harness_test.go`
- `internal/core/shipment_blocked_writer_harness_test.go`

Validation:

- `git diff --check`: PASS
- Modified-file `gofmt -l`: PASS
- `go test -run=^$ -count=1 ./...`: PASS
- `go vet ./...`: PASS
- CI-pinned golangci-lint `v1.64.8` on Go 1.24: PASS
- `TestUR1B_`, `TestUR2_`, and `TestUR3_`: expected named assertion RED
- Deterministic workspace-global-lock contention proof remains excluded from R3 and assigned to
  R10 `174.048-T`.

## Remaining P1 findings

The fresh report-only review against the amended contract reported two P1 findings:

1. **R2 same-surface positive-control contradiction**
   - File/symbol: `shipment_blocked_writer_harness_test.go`,
     `TestUR2_GovernedWriterBoundaryRejectsBypassAndCorrelatesWrites`.
   - Amended `174.040-T` requires a positive non-shipment control on every named guarded surface,
     including `MoveShipmentStatus`.
   - `MoveShipmentStatus` is a shipment-specific API. The current attempted control invokes
     `MoveShipmentStatus` on a shipment and substitutes `UpdateArtifact` for the non-shipment
     member control. That does not satisfy “same surface.”
   - Requiring queued-to-active through `MoveShipmentStatus` as a positive control also conflicts
     with the harness contract that activation outside `ClaimShipment` and confirmed
     unblock-to-active must be refused.
   - The amended acceptance criterion is therefore not implementable literally for this surface
     without another contract clarification.

2. **Compensated recovery lacks correlated canonical status evidence**
   - File/symbol: `shipment_blocked_recovery_harness_test.go`,
     `requireUR3CorrelatedTerminalEvidence`.
   - Committed recovery requires correlated `shipment_status_changed` evidence, but compensated
     recovery currently requires only exact restored Markdown plus a generic correlated terminal
     event.
   - Amended `174.041-T` requires canonical Markdown plus append-only
     `shipment_status_changed` evidence and a correlated terminal outcome for both policy-aware
     outcomes. The compensated path must derive the restored status from the preimage and require
     its correlated status event before or at the terminal compensated event.

Per operator instruction, any P1 against the amended contract halts execution. No implementation,
task completion, PR work, or fifth harness correction was performed.

## Required next action

Stage must clarify `174.040-T` so the shipment-only `MoveShipmentStatus` surface is exempt from the
literal non-shipment same-surface control while retaining its shipment refusal assertions. After
that reviewed clarification, Ship can make one bounded harness reconciliation that also adds the
missing compensated-path correlated status evidence, then rerun the RED review gate.
