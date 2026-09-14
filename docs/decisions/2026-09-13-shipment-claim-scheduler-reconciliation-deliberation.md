---
chunk_strategy: h1-h2-h3
description: "Deliberation for CC0EBB59 — reconcile backlogit shipment-claim activation semantics with the P-002.6 wave scheduler via a universal persisted scheduler-baseline marker (in-repo portion); record-only claim mode considered and dropped"
doc_type: decision
schema_version: "1.0"
source: docs/decisions/2026-09-13-shipment-claim-scheduler-reconciliation-deliberation.md
title: "Deliberation: Shipment-claim activation reconciliation (universal scheduler-baseline marker)"
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

* **In-repo (backlogit) — harvestable now**: give the shipment lifecycle a
  persisted **universal scheduler-baseline marker** written on every
  claim-activated manifest task so claim-activated tasks are distinguishable from
  organic active residuals. (A separate **record-only claim mode** was considered
  and **dropped** — see Decision — because the universal marker removes the need
  for a divergent claim path.) This is the backlogit half of the contract and the
  authorized in-repo reliability defect.
* **Out-of-repo (autoharness) — NOT edited here**: the wave scheduler's
  consumption of that marker (treating baseline-marked tasks as
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

Adopt **Option A** (persisted universal baseline marker) as the sole in-repo
mechanism. The marker is written on **every** claim activation via a gated,
off-by-default `setArtifactStatus` seam, and the claim-activated-vs-organic
distinction is surfaced through a supported CLI/MCP/data read surface the
out-of-workspace scheduler can consume. The **record-only claim mode** (Option B,
and the thin record-only entry originally floated as a layered slice) is
**dropped**: because the universal marker already distinguishes claim-activated
tasks, a second claim path would add divergent lifecycle semantics for no
additional benefit and is rejected. Default claim behavior is unchanged; the
marker is additive and advisory. Autoharness scheduler consumption is a recorded
cross-workspace follow-up and a Ship-time / out-of-workspace concern — **not** a
Stage blocker.

### Scope Boundary

In-repo `internal/core/shipment_lifecycle.go` + tests + operator docs. No
autoharness edit. No change to default claim behavior; the universal marker is
additive and advisory, and unmarked items are byte-identical to pre-change
behavior.
