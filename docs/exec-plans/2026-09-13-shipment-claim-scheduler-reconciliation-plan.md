---
chunk_strategy: h1-h2-h3
description: "Implementation plan for CC0EBB59 — add a record-only claim mode / explicit persisted scheduler-baseline marker to backlogit shipment-claim activation so claim-activated manifest tasks are not misclassified as active residuals"
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-09-13-shipment-claim-scheduler-reconciliation-plan.md
title: "Implementation Plan: Shipment-claim activation reconciliation (record-only claim mode)"
docline:
    stash_id: CC0EBB59
    status: approved
    created_at: 2026-09-13T17:31:00Z
---

## Objective

Add an additive, backward-compatible **enabling precondition** to backlogit
shipment-claim activation that lets a claim-activated manifest task be
distinguished from an organic active residual. This shipment delivers the in-repo
marker + accessor **only**; the observable P-002.6 scheduler-misclassification
defect is fully resolved only once the autoharness scheduler consumes the marker
(out-of-workspace follow-up). This shipment is therefore an **enabling
precondition, not full defect resolution**. In-repo
(`internal/core/shipment_lifecycle.go`) only.

Source: `docs/decisions/2026-09-13-shipment-claim-scheduler-reconciliation-deliberation.md`.

## Requires plan hardening: yes

Touches shipment lifecycle state and claim semantics consumed by an external
scheduler across a trust/workspace boundary. See `## Plan Hardening`.

## Implementation Units

* **U1** Persist the **scheduler-baseline marker** on **every** claim-activated
  manifest item, stored as an additive `custom_fields` key with `omitempty`. The
  marker MUST be written **atomically within the same `setArtifactStatus`
  activation write** (not a second, separately-tracked write) so it is covered by
  the existing all-or-nothing claim invariant and the `activatedIDs` rollback
  tracking. The write seam is a **gated, off-by-default option threaded into
  `setArtifactStatus`** so that ONLY claim-driven activation writes the marker;
  all other `setArtifactStatus` callers (gate transitions, queue ops) stay
  byte-identical. Domain: code (`shipment_lifecycle.go` + the gated seam).
  * AC: the marker is set exactly on items `ClaimShipment` transitions
    `StatusQueued`→`StatusActive`; NON-claim `setArtifactStatus` callers produce
    byte-identical frontmatter; a consumer that ignores the marker observes no
    behavior change (marker is advisory + `omitempty`).
* **U2** Expose a **read accessor** the scheduler consults to distinguish a
  claim-activated item from an organic active residual. (Refinement vs the
  deliberation: because U1 sets the marker on **every** claim activation, a
  separate "record-only claim mode" is unnecessary and is dropped to avoid a
  divergent claim path — the accessor over the universal marker provides the
  distinction.) Domain: code. Depends on U1.
  * AC: the accessor reports claim-activated for marked items and organic-active
    for unmarked active items; it does not mutate state.
* **U3** Add symmetric **rollback-clears-marker** handling:
  `rollbackShipmentClaim` (`shipment_lifecycle.go:100-133`) MUST clear the marker
  for every id it reverts, **atomically within the same revert
  `setArtifactStatus(...StatusQueued...)` write** (lines ~104-107), so a
  partially-failed rollback never leaves a torn queued-but-marked item. Domain:
  code. Depends on U1.
  * AC: after a claim that fails partway and rolls back, no reverted item retains
    the baseline marker (no torn state), even if rollback itself fails midway.
* **U4** Tests: claim-activation sets the marker atomically; the gated seam leaves
  non-claim `setArtifactStatus` callers byte-identical; absent-marker consume path
  is unchanged; rollback clears the marker on every reverted id; the accessor does
  not count a claim-activated task as an organic residual. Domain: tests. Depends
  on U1, U2, U3.
* **U5** Operator docs: scheduler-baseline marker + accessor semantics and the
  cross-workspace consumption contract; explicitly labels this as an enabling
  precondition. Domain: docs.
  * AC: docs state the marker is advisory, the backlogit claim gate remains
    authoritative, and full defect resolution requires the autoharness follow-up.

## Constitution Check

* **Single-domain tasks**: code / code / tests / docs. Pass.
* **2-hour rule**: each unit < 3 files / < 5 functions / < 4 scenarios. Pass.
* **Backward compatibility**: default claim behavior unchanged when marker/mode
  not requested. Pass.
* **Workspace containment (P-017)**: in-repo only; autoharness consumption is a
  documented follow-up. Pass.

Constitution Check: pass

## Plan Hardening

* **ProposedAction**: persist a new lifecycle marker at claim activation.
  **ActionRisk**: medium (lifecycle-state schema addition). Mitigation: additive
  field with absent-default; U3 asserts existing paths unchanged.
* **ProposedAction**: expose record-only claim entry. **ActionRisk**: medium
  (second claim path). Mitigation: record-only reuses the default transition and
  only sets the marker; no divergent state machine.
* **Cross-boundary safety**: the marker is advisory data for the scheduler; the
  backlogit claim gate remains authoritative. No autoharness behavior is assumed
  to change for this shipment to be correct in-repo.
* **Rollback**: additive; revert removes the marker and the record-only entry.

## Verification

* `go test ./internal/core/... -race` including U4.
* Existing shipment lifecycle tests remain green.

## Follow-ups (out of this shipment)

* Autoharness P-002.6 wave scheduler to consult the baseline marker and exclude
  claim-activated tasks from residual classification (cross-workspace).

## Plan Review

dispatch_mode: multi-agent-dispatch
decision: FAIL

Attempt 1. Personas: Correctness + Architecture Strategist. Coverage complete.

**Gate rationale**: one P1 finding → FAIL per rubric.

Findings:
* **P1 (Correctness)** — marker/rollback invariant gap: `rollbackShipmentClaim`
  reverts status but would not clear a claim-activated marker, leaving a torn
  queued-but-marked item that misleads the scheduler accessor.
* **P2** — record-only semantics ambiguous (redundant marker-set vs a rejected
  second claim path). Pin down as a flag on `ClaimShipment`.
* **P2** — marker persistence must be atomic within the `setArtifactStatus`
  activation write, inside `activatedIDs` rollback tracking.
* **P2** — per-unit acceptance criteria missing.
* **P3** — Objective overstated the delivered outcome (enabling precondition,
  not defect resolution).

Plan hardening required: yes; present. Remediation applied in this revision
(added U3 rollback-clears-marker, atomic-write requirement in U1, record-only as
a flag in U2, per-unit AC, enabling-precondition labeling). Re-review below.

<!-- plan-review-attempt: 1 -->

## Plan Review

dispatch_mode: multi-agent-dispatch
decision: PASS

Attempt 2. Personas: Correctness + Architecture Strategist (re-review). Coverage
complete. Prior P1 (marker/rollback torn state) confirmed resolved via U3
(atomic rollback-clears-marker). The attempt-2 P2 (U1/U2 contradiction on when
the marker is written) is resolved in this revision: U1 now sets the marker on
EVERY claim activation via a gated off-by-default `setArtifactStatus` seam; the
redundant "record-only mode" is dropped and U2 reduced to the read accessor;
rollback clear is atomic within the revert write; U5 gained an AC.

**Gate rationale**: no P0/P1/P2 remain; residual items were folded into the
revision. Plan hardening required: yes; present and adequate.

<!-- plan-review-attempt: 2 -->
