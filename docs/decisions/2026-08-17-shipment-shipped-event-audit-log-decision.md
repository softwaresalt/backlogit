---
title: "Decision: durable shipped-event append with class-aware rollback and report-only doctor reconciliation"
description: "Durable decision record for stash 0115F71F, distinguishing committed clean-main behavior from planned behavior and from the deferred prevention follow-up 47B48DB0"
source: ".backlogit/queue/059-DL.md"
doc_type: decision
chunk_strategy: h1-h2-h3
schema_version: "1.0"
---

## Status

Decided. Harvested into feature `143-F` on branch `chore/restage-143-shipment-audit-log`, based on
`origin/main` `3ec95ee3e7c6787762beb15c0e4b226746e50a89`.

This record supersedes the staging attempt on
`origin/chore/stage-143-shipment-audit-log-reconciled` (`2188bab2`), which asserted a baseline the
committed tree does not contain. That branch is retained as reference history only.

## Context

Stash `0115F71F` reports that a shipment can persist `archived_status: shipped` while its
append-only JSONL log omits `shipment_status_changed: shipped`. Observed in the field from
autoharness stash `84D8E6AB` / shipment `114-S`.

## Three-state separation

The single most important property of this record is that it keeps three things apart. The prior
staging attempt failed review three times largely because it blurred them.

### A. What is committed on clean `origin/main` today

| Fact | Evidence |
|---|---|
| The shipped-status path emits its audit event through the best-effort, fire-and-forget `appendItemEvent` | `internal/core/shipment.go:205`, implementation at `:660` / `:677`, warning-and-return at `:690` and `:696` |
| The status is already persisted before that emission | `internal/core/shipment.go:200` (`persistArtifactWithGuard`) |
| Archival proceeds regardless of the append outcome | `internal/core/shipment_lifecycle.go:509` returns the locked closure; `:515-527` archives unconditionally |
| An error-returning appender exists, but only for gate evidence | `appendItemEventErr` at `internal/core/gate_evidence.go:40`; seam `gateEvidenceAppend` at `internal/core/workspace.go:55-59`; dispatcher at `internal/core/gate_evidence.go:81-86` |
| There is **no** shipment-scoped `ws.shipmentEventAppend` seam | ripgrep across `internal/`, `cmd/`, `tests/` returns zero matches |
| There is **no** `shipmentEventAppendError` classification type | ripgrep returns zero matches |
| No transition out of `shipped` is permitted | `isValidShipmentTransition` at `internal/core/shipment.go:713-722` |
| `restoreShipArtifacts` silently skips an item whose log lock it cannot re-acquire | `internal/core/shipment_lifecycle.go:184-188` |
| LIFO defer order runs the non-member covering-feature fallback **after** the artifact locks are released | registration at `:365` and `:375` |

### B. What this decision plans to change

* Introduce a per-workspace `shipmentEventAppend` seam and a shipment-scoped
  `appendShipmentEventErr` that owns the item-log lock, tags a pre-append lock failure
  `ErrWriteNotApplied`, and tags a short or partial write `ErrWriteIndeterminate`.
* Route **only** the `active -> shipped` transition through it fail-closed. Claim and abandon keep
  best-effort semantics.
* Classify the captured append error per the precedence `MutationEnvelope` already uses: tagged
  indeterminate suppresses rollback; everything else, including untagged plain errors, compensates.
* Make compensation honest: an item that cannot be restored surfaces as
  `CompensationState: "partially-compensated"` naming the un-restored IDs, never a silent skip.
* Correct the adjacent defer ordering so the non-member covering-feature fallback runs with the
  artifact locks genuinely held.
* Add a report-only doctor audit emitting two distinct findings, `missing_shipped_event` and
  `shipped_unarchived_residue`, that never writes or rewrites JSONL.
* Surface the audit on the CLI and on MCP, and make the MCP recovery guidance producer-scoped so it
  names a check that can actually detect the residue.
* Update the governed-recovery contract doc, workflow policy P-007, the `shipment-reconcile` skill,
  and the Ship agent, all of which currently encode a "ship always archives" postcondition that this
  change deliberately breaks.

### C. What is deferred and to where

| Deferred item | Owner |
|---|---|
| Prevention: closing non-`ShipShipment` paths that can also produce `archived_status: shipped`, including the deliberation's minimum floor that `UpdateArtifactWithGate` must not drive a shipment to `shipped` bypassing the durable event | active stash `47B48DB0` |
| A supported reconciliation transition out of `shipped` | named closure follow-up |
| Durability of the item-level events inside `ShipShipment` (`status_changed`, `returned_to_backlog`, parent cascades) | named closure follow-up |
| Reconciling the two pre-existing drifted registry doctor params and adding a repo-wide `params`-to-`InputSchema` parity assertion | named closure follow-up |

Detection ships now because historical residue already exists and the forward fix cannot repair it.
Prevention waits because closing those paths is a different concern with a different blast radius,
and the report-only audit already flags residue regardless of which producer created it.

## Options considered

| Option | Verdict |
|---|---|
| A. Wrap the whole status-to-archival path in `core.MutationEnvelope` | Rejected: very high blast radius across the hardened `ShipShipment` locked closure, its membership lock, the head-drift guard, and the 133-F covering-feature restore |
| B. Shipment-scoped error-returning append inside the existing envelope, classified per the two-class durable-write contract, plus a report-only doctor audit | **Chosen** |
| C. Swap to an error-returning appender and always roll back | Rejected: rolling back an indeterminate write can destroy an applied append, violating the contract in `docs/compound/2026-07-28-durable-writes-two-class-contract-commit-then-surface.md` |
| D. Hoist the shipment item-log lock to the top of the locked closure | Rejected at review cycle 2: the ownership marker in `ctx` would outlive the lock, exposing the rollback log rewrite and the archival appends to lock-free execution, inverting the lock order against `lockArtifactMutations`, and holding a starving lock across two gate-broker evaluations |
| E. Retry the append before classifying | Rejected at review cycle 2: an append is explicitly not safe to blindly retry, and retrying risks a duplicate shipped event in the log this change exists to protect |
| F. Keep the append best-effort and only warn more loudly | Rejected at adversarial review: this is the status quo the bug report is about and it contradicts the stash's explicit "archival must not continue without that durable event" |

## Accepted consequences

* A ship that previously succeeded can now refuse. That is the intent, scoped to the shipped
  transition only.
* A new end state exists - `shipped` and unarchived - with **no automated forward transition**. This
  is declared, monitored by SLI 2 and SLI 3, and has a documented manual recovery procedure.
* Historical residue is reported, never repaired. The audit never synthesizes a missing event.
* Eleven tasks, roughly twenty-two hours. The decomposition was independently judged proportionate
  by the Architecture Strategist and the Scope Boundary Auditor after two prior review cycles.

## Gate record

| Gate | Outcome |
|---|---|
| Plan review, cycle 1 | FAIL - P0=3, P1=16 across five personas |
| Plan review, cycle 2 | FAIL - P0=2, P1=12 |
| Plan review, cycle 3 | **PASS** - P0=0, P1=0 |
| Adversarial multi-model review, 3 reviewers on 3 model families | **PASS** - HIGH-confidence P0/P1 = 0; 2 MEDIUM findings fixed; 11 of 13 LOW findings fixed, 2 rejected with rationale |

## Superseded lesson

The prior staging attempt tripped a three-cycle circuit breaker. The cause was not reviewer
disagreement; it was that the plan asserted a baseline the committed tree did not contain - most
importantly a shipment-scoped append seam and an error-returning shipped path that do not exist on
`origin/main`. Two durable lessons carried into this record:

1. Baseline claims must be re-derived in a clean worktree at a named SHA, and the anchors must be
   regenerated mechanically rather than transcribed. Both cycle-1 and cycle-2 reviews found
   hand-written anchors drifting by a few lines even when the substance was right.
2. Red-before-green must be expressed as a dependency edge, not as prose. The prior plan gave the
   production seam to the first task and the failing tests to the third, which inverted the
   constitutional order while reading as if it complied.

## References

* Plan: `docs/exec-plans/2026-08-17-shipment-shipped-event-audit-log-plan.md`
* Adversarial review: `docs/reviews/2026-08-17-shipment-shipped-event-audit-log-adversarial-review.md`
* Deliberation: `.backlogit/queue/059-DL.md`
* Prior art: `docs/compound/2026-07-28-durable-writes-two-class-contract-commit-then-surface.md`,
  `docs/compound/2026-07-29-durable-writes-test-seam-patterns.md`,
  `docs/compound/2026-07-20-manual-feature-harvest-provenance-backfill.md`
