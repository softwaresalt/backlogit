---
chunk_strategy: h1-h2-h3
description: "Adversarial multi-model review of the shipment shipped-event audit-log durability plan and its decision record."
doc_type: review
schema_version: "1.0"
source: docs/reviews/2026-08-16-shipment-shipped-event-audit-log-adversarial-review.md
title: "Adversarial multi-model review: shipment shipped-event audit-log durability"
ms.date: 2026-08-16
ms.topic: concept
---

## Scope

Adversarial multi-model review of:

* Plan: `docs/exec-plans/2026-08-16-shipment-shipped-event-audit-log-plan.md`
* Decision: `docs/decisions/2026-08-16-shipment-shipped-event-audit-log-deliberation.md`

This review supplements the multi-persona plan-review gate (recorded in the plan's
`## Plan Review` section, decision PASS). It was run because the work touches durable
event ordering, a hardened rollback envelope, and doctor reconciliation.

## Dispatch

Three independent reviewers were dispatched in parallel across different model
families, each asked to adversarially break the plan and its decision record and
return P0-P3 findings grounded in the code:

* Reviewer A: GPT-family (`gpt-5.6-terra`)
* Reviewer B: Gemini-family (`gemini-3.1-pro-preview`)
* Reviewer C: Grok-family (`grok-4.6`)

Consensus weighting: HIGH = all three agree; MEDIUM = two agree; LOW = single
reviewer. Per the adversarial-review protocol, a HIGH-confidence P0/P1 is
gate-blocking.

## Per-reviewer summary

* Reviewer A (GPT): no P0; four P1s (prevention bypass, lock-vs-append boundary,
  MCP recovery pointing at the wrong doctor check, test-first ordering) and one P2
  (residue false positives).
* Reviewer B (Gemini): NO P0/P1. Validated the design as sound. Confirmed
  `completeReleaseScope` uses atomic file renames (never emits `ErrWriteIndeterminate`),
  so restricting classification to the captured shipped-append error is safe;
  confirmed the 133-F restore-under-lock ordering and the `restored = true`
  neutralization respect LIFO defer execution; confirmed `scanCanonicalArtifacts`
  natively covers both `queue/` and `archive/`. One P3 (extend `artifactRef` to
  carry `archived_status`).
* Reviewer C (Grok): no P0 if only `appendItemEventErr` is wrapped; four P1s
  overlapping A (prevention bypass incl. `ArchiveItem`, MCP recovery guidance,
  unsafe class-aware implementation without covering-feature test coverage, Unit 1
  alone equals rejected Option C) and several P2/P3.

No finding reached HIGH confidence (all three agreeing). Reviewer B found the core
design sound with no P0/P1.

## Consensus findings and dispositions

| # | Confidence | Severity | Finding | Disposition |
|---|---|---|---|---|
| 1 | MEDIUM (A, C) | P1 | Inconsistency still reachable via `UpdateArtifactWithGate` + `ArchiveItem` when formal gating is off; deferred `47B48DB0` did not cover `ArchiveItem` | Detection net (Unit 4) already flags the residue regardless of origin, satisfying the bug requirement. Prevention remains a policy-compliant defer; stash `47B48DB0` broadened to add the `ArchiveItem` guard. |
| 2 | MEDIUM (A, C) | P1 | MCP `mutation_partial` recovery text points to `check_partial_mutations`, which cannot detect a missing shipped event | Resolved in plan: Unit 6 now requires the recovery guidance to direct callers to `doctor --check-shipped-event-completeness`. |
| 3 | MEDIUM (A, C) | P1 | Class-aware rollback is easy to implement unsafely; Unit 3 omitted covering-feature-restore and other-error-rollback assertions | Resolved in plan: Unit 3 now asserts the covering-feature restore in the indeterminate scenario; other-error rollback is covered by the existing `TestShipShipment_RollsBack*` tests (noted). |
| 4 | MEDIUM (B, C) | P1/P2 | Doctor must scan `queue/` too: a shipped-but-unarchived shipment stays under `queue/` (shipped is not archive-routed), so an archive-only walk misses it | Resolved in plan: Unit 4 now walks the full canonical scan (`queue/` and `archive/`) and extends `artifactRef` to carry `archived_status`. |
| 5 | MEDIUM (A, C) | P2 | Clean pre-append lock-acquisition failure is indistinguishable from an untagged append failure at the boundary, degrading a recoverable case to indeterminate | Resolved in plan: Unit 2 notes tagging lock-acquisition as `ErrWriteNotApplied` at the appender to compensate, else it safely degrades to indeterminate + doctor detection. |
| 6 | LOW (C) | P2 | Runtime-verification ordering wording ("archived_status then event") contradicted Unit 3 ("event before archival") | Resolved in plan: runtime verification now states event before archival. |
| 7 | LOW (C) | P3 | Unit 6 partly redundant (`domainError` may already map `MutationPartialError`); false `CheckGateEvidence` MCP precedent | Resolved in plan: Unit 6 reframed to verification-plus-recovery-guidance; Unit 5b no longer cites the `CheckGateEvidence` precedent. |
| 8 | LOW (A) | P2 | "Shipped-but-unarchived" can false positive during a normal in-flight ship | Resolved in plan: Unit 4 carries the in-flight-ship transient caveat. |

## Gate outcome

No HIGH-confidence P0/P1 findings. Every MEDIUM P1 was resolved in the plan
(items 2, 3, 4) or handled by a policy-compliant defer with an actionable backlog
ID (item 1, stash `47B48DB0`). Remaining P2/P3 items were resolved as precision
fixes. Reviewer B independently validated the core design as sound.

Adversarial review result: PASS (no blocking findings). Cleared for harvest.
