---
chunk_strategy: h1-h2-h3
description: "Deliberation for CC0EBB59 — reconcile backlogit shipment-claim activation semantics with the P-002.6 wave scheduler via a record-only claim mode / explicit scheduler baseline (in-repo portion)"
doc_type: learning
schema_version: "1.0"
source: docs/decisions/2026-09-13-shipment-claim-scheduler-reconciliation-deliberation.md
title: "Deliberation: Shipment-claim activation reconciliation (record-only claim mode)"
docline:
    stash_id: CC0EBB59
    status: decided
    created_at: 2026-09-13T17:31:00Z
---

## Deliberation: Shipment-claim activation reconciliation

**Depth**: standard. **Route**: `deliberate` (`requires deliberation: true`).
**Solo group** (Step 1.5): distinct code surface from the concurrency group —
this is shipment lifecycle (`internal/core/shipment_lifecycle.go`) + scheduler
classification, not the artifact-mutation persist path. Independent covering
feature and shipment.

### Problem Frame

When a shipment is claim-activated, its manifest tasks transition to an active
state. The P-002.6 wave scheduler then classifies those claim-activated manifest
tasks as **active residuals**, conflating "activated by a shipment claim" with
"pre-existing in-flight residual work." This produces incorrect scheduler
baselines and wave admission decisions.

Verified in-repo: shipment claim/activation lives in
`internal/core/shipment_lifecycle.go` (claim path ~lines 46–105). The scheduler
that consumes activation state (P-002.6 wave scheduler) is **autoharness Ship-
workflow infrastructure**.

### Scope Split (workspace containment, P-017)

* **In-repo (backlogit) — harvestable now**: give the shipment lifecycle an
  explicit **record-only claim mode** and/or a persisted **scheduler baseline
  marker** so that claim-activated manifest tasks are distinguishable from
  organic active residuals. This is the backlogit half of the contract and the
  authorized in-repo reliability defect.
* **Out-of-repo (autoharness) — NOT edited here**: the wave scheduler's
  consumption of that marker (treating record-only / baseline-marked tasks as
  non-residual). Recorded as a cross-workspace follow-up; Stage does not modify
  autoharness.

The in-repo change is designed to be **consumable** by the scheduler without
forcing an autoharness change to land first: it adds a new, additive marker with
a safe default (existing behavior preserved when the marker is absent).

### Options

**Option A — Explicit persisted scheduler-baseline marker on claim activation
(CHOSEN).** On claim, persist a marker/timestamp on each activated manifest task
identifying it as claim-activated (vs organic-active). Provide a read API the
scheduler can consult. Existing consumers ignoring the marker see no behavior
change.

* Pros: additive, backward-compatible, testable entirely in-repo; gives the
  scheduler an authoritative baseline without a lifecycle-semantics rewrite.
* Cons: requires the scheduler to eventually consult the marker (autoharness
  follow-up) to fully realize the benefit.

**Option B — New "record-only" claim mode that activates the shipment without
transitioning manifest tasks to the residual-classified active state.**

* Pros: strongest separation.
* Cons: introduces a second claim semantics; higher risk of diverging lifecycle
  paths and of surprising existing claim consumers. Heavier than needed for the
  bounded defect.

**Option C — Change the scheduler classification only (autoharness).** Rejected
for this run: out of workspace; would leave backlogit unable to express the
distinction.

### Decision

Adopt **Option A** (persisted baseline marker) as the primary in-repo mechanism,
and expose a minimal **record-only claim** entry that sets the marker without
altering default claim behavior (a thin, backward-compatible slice of Option B
layered on Option A's marker). Default path unchanged; marker is additive.
Autoharness scheduler consumption is a recorded cross-workspace follow-up and a
Ship-time / out-of-workspace concern — **not** a Stage blocker.

### Scope Boundary

In-repo `internal/core/shipment_lifecycle.go` + tests + operator docs. No
autoharness edit. No change to default claim behavior when the new marker/mode is
not requested.
