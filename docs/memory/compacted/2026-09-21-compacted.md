---
title: "Compacted memory: 2026-09-21 155-S wave 3 and quality-gate blockers (Ship halts)"
date_range: "2026-09-21 to 2026-09-21"
doc_type: compacted-memory
source_count: 8
---

# Compacted memory: 2026-09-21

Compacts 8 Ship halt and blocker records for shipment 155-S (feature 174-F). The originals were moved to docs/archive/memory/ (not deleted).

## Decisions made

- Wave 7 halted at WAVE_RED_DELIVERABLE_EARLY_GREEN. The selector ^TestUR3_ (owned by 174.041-T) was already GREEN, but its canonical mapping named 174.047-T and 174.048-T as green-makers with closing wave 8. The mapping was routed to Stage for amendment. The selector was not weakened and 174.048-T was not pulled forward.
- Quality-gate reassessment: the raw count of 518 gofmt findings was invalid, because CRLF checkouts inflated it. The canonical count was 9 pre-existing findings, none introduced by 155-S. The 56 golangci-lint findings came from a local v2.13.2 binary. The CI pin is v1.64.8, which reported zero findings with the pinned version.
- Resumed Step 4.3 blocker: the same blocking gate persisted after the operator-selected checkpoint was restored and resolved.
- Wave 3 harness reconciliation: the four preserved harness files were reconciled to the Stage-reviewed contract. Completed tasks at that point: 174.039-T and 174.052-T.
- Wave 3 cycle 4 was the final authorized harness cycle. Its three P1 findings required a halt, with no implied fifth cycle and no residual-risk acceptance.
- Wave 3 harness review circuit breaker: three review-fix cycles were consumed, so Ship stopped instead of making a fourth repair.

## Files modified or created

- internal/core/shipment_blocked_harness_helpers_test.go
- internal/core/shipment_blocked_lifecycle_harness_test.go
- internal/core/shipment_blocked_recovery_harness_test.go
- internal/core/shipment_blocked_writer_harness_test.go
- internal/core/shipment_blocked_shape_test.go (wave 1 scaffold, commit e495c4ab; claimed through 155-S)
- Stage amendment HEAD recorded as 8ebb1340; the Ship branch was at c7b7f5a0 at the latest rev10 halt.

## Key learnings

- Early-green finding: wave 7 RED selectors were partially GREEN because of the mapping inconsistency. Convergence needs every open selector to remain RED.
- Windows CRLF materialization distorts raw gofmt output. Canonical LF or blob checks are the valid evidence basis.
- Source-level and writer-harness contracts need explicit control positives for non-shipment surfaces, so shipment-only guards do not over-block ordinary member transitions.
- Review findings from the same contract surface repeated across rev9, rev10, and cycle 4: lock contention proofs, bulk per-item result semantics, member drift and compare-and-swap refusal, and recovery evidence that must come from the post-reopen suffix.
- Report-only review of HEAD edfc0494 returned NOT READY with P1 items: ClaimShipment did not serialize with membership writers, startup rollback recovery lacked a compare-and-swap drift check, and normalize_blocked_shipment had no backlog registry mapping.

## Failed or rejected approaches

- Pulling 174.048-T forward to make wave 7 converge (rejected).
- Accepting residual P1 risk without authorization (rejected).
- Relying on a scheduler-dependent lock proof (rejected). The final review found it could pass if the competing goroutine was scheduled late.
- Exceeding the three-cycle review-fix limit.
- Treating the raw 518-file gofmt count as a real formatting failure.

## Outcomes and open items

- 155-S remained active. 154-S remained queued and was not claimed or mutated. PR 449 was not touched.
- Stage amendment needed for: the ^TestUR3_ closing-contract mapping; a concrete seam for the deterministic lock-contention proof; a clarified shipment-only positive control for MoveShipmentStatus; and compensated recovery correlated status evidence.
- Operator disposition was needed for the pre-existing lint and format baseline. The options were a Stage-approved baseline work unit or an explicitly recognized gate waiver mechanism.

## Sources

- docs/memory/2026-09-21-ship-155s-wave7-early-green-blocker.md
- docs/memory/2026-09-21/155-s-quality-gate-reassessment.md
- docs/memory/2026-09-21/155-s-resumed-quality-gate-blocker.md
- docs/memory/2026-09-21/155-s-wave-3-amended-contract-review-blocker.md
- docs/memory/2026-09-21/155-s-wave-3-cycle-4-review-blocker.md
- docs/memory/2026-09-21/155-s-wave-3-harness-review-circuit-breaker.md
- docs/memory/2026-09-21/155-s-wave-3-rev10-bulk-review-blocker.md
- docs/memory/2026-09-21/155-s-wave-3-rev9-review-blocker.md
