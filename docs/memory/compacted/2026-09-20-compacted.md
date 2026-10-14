---
title: "Compacted memory: 2026-09-20 shipment-claim convergence and 155-S wave 1 quality-gate blocker"
date_range: "2026-09-20 to 2026-09-20"
doc_type: compacted-memory
source_count: 2
---

# Compacted memory: 2026-09-20

Compacts 2 source files. The originals were moved to docs/archive/memory/ (not deleted).

## Decisions made

Shipment-claim and wave-scheduler convergence (Stage, PR 449):
- Model M2 was chosen. ClaimShipment keeps its all-members-active semantics. Dependency-gated wave admission is added without changing those semantics, through three additive parts: (i) a scheduler-baseline marker (154-S and 173-F, in-repo), (ii) external autoharness P-002.6 marker consumption (P-017), and (iii) the deferred 6434A4D7 claim-time dependency guard plus a shipped-only readiness gate.
- Rejected: M1 (in-place change, backward-incompatible), M3 (record-only verb, dropped on 2026-09-13), and M4 (active to pending, unsafe).
- Bootstrap set: {155-S, 154-S}. The P-002.6 active-residual halt bypass was NOT approved and stayed BLOCKED.

155-S wave 1 (Ship):
- The operator authorized the P-002.6 active-residual bypass only for the bootstrap set {155-S, 154-S}. This invocation covered 155-S only.
- 155-S was claimed on feat/155-s-s14-resumable-shipment-blocked-lifecycle-status. The exact waiver was recorded as a shipment comment before the claim.

## Files modified or created

- New: docs/decisions/2026-09-20-shipment-claim-wave-scheduler-convergence-deliberation.md
- Rewritten: docs/exec-plans/2026-09-17-baseline-convergence-plan.md (Execution Readiness BLOCKED, Plan Hardening, final review ADVISORY)
- Rewritten: docs/decisions/2026-09-19-baseline-convergence-decomposition.md
- Rewritten: .backlogit/queue/175.099-T.md, 175.100-T.md, 175.101-T.md (governed-claim dirty-path footprint)
- Backlog: edge 176-S depends_on 154-S; labels readiness-blocked and do-not-claim-until-convergence on 176-S and 157-S; labels bootstrap-exception and convergence-prerequisite on 154-S.
- Ship: harness scaffold internal/core/shipment_blocked_shape_test.go; harness and claim commit e495c4ab.

## Key learnings

- core.ClaimShipment activates the shipment and every queued member with no dependency check. About 60 test sites lock in all-members-active behavior.
- Ship wave admission halts on any active member, so claiming a multi-member shipment causes an immediate WAVE_NO_PROGRESS. The same mismatch had blocked 140-S.
- The queue filter uses a six-status cascade (done, accepted, archived, shipped, abandoned, rejected).
- Wave 1 blocker: golangci-lint run reported 56 pre-existing findings, and gofmt -l . reported 518 pre-existing CRLF-formatted Go files. Neither was introduced by the shipment.
- Routing was ROUTING_DEGRADED: the configured route was unavailable, and the runtime default was used as the operator authorized.

## Failed or rejected approaches

- Claiming unrelated multi-member shipments before the marker exists.
- Waiving the lint and format baseline. No quality-gate waiver was authorized, so wave 1 could not converge.
- Plan-review attempt 1 FAIL (2 P1), remediated through machine-consumable signals.

## Outcomes and open items

- PR 449 cannot reach review-ready until the operator explicitly approves the P-002.6 waiver and the bootstrap executes.
- 174.039-T and 155-S remained active, with no tasks completed and 154-S untouched.
- Operator disposition was needed for the pre-existing lint and format baseline before Ship could resume at Step 4.3.
- Post-refresh note (2026-09-26, recorded in the source): 155-S later shipped (PR 450 at 2c8759c3; closure PR 451) and is archived. Only 154-S remained to bootstrap, and its waiver was still pending.

## Sources

- docs/memory/2026-09-20-shipment-claim-wave-scheduler-convergence-session.md
- docs/memory/2026-09-20/155-s-wave-1-quality-gate-blocker.md
