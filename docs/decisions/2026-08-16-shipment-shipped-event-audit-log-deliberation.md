---
chunk_strategy: h1-h2-h3
description: "Deliberation on fixing shipment shipped-event audit-log completeness across active to shipped to archived transitions."
doc_type: decision
schema_version: "1.0"
source: docs/decisions/2026-08-16-shipment-shipped-event-audit-log-deliberation.md
title: "Shipment shipped-event audit-log completeness deliberation"
ms.date: 2026-08-16
ms.topic: concept
topic: "Fix shipment audit-log completeness for active -> shipped -> archived transitions"
depth: standard
decision_status: decided
promoted_to: plan
tags:
  - shipment-lifecycle
  - audit-log
  - data-integrity
  - doctor
---

## Problem Frame

Source: stash `0115F71F` (medium, bug), observed downstream from autoharness
stash `84D8E6AB` / shipment `114-S`.

A shipment can persist `archived_status: shipped` while its append-only JSONL
event log omits the `shipment_status_changed: shipped` event. The audit trail is
then incomplete: durable frontmatter says the shipment shipped, but no shipped
event records the transition.

Technical root cause (as originally observed):

* `internal/core/shipment.go` `moveShipmentStatusWithHeadGuard` persists the
  shipment status transition through `persistArtifactWithGuard`, then emits the
  audit event with the best-effort helper
  `appendItemEvent(ctx, ws, shipmentID, "shipment_status_changed", {status})`.
* `appendItemEvent` (shipment.go:660) delegates to `appendItemEventWithCommit`,
  which on lock, append, or index failure only calls `slog.Warn` and returns.
  The append is fire-and-forget: a failure is swallowed and never surfaced to the
  caller.
* `ShipShipment` (`internal/core/shipment_lifecycle.go:285`) calls
  `moveShipmentStatusWithHeadGuard(active -> shipped)` inside its locked closure,
  then, after the closure returns, calls `archiveItems`, which stamps
  `archived_status: shipped`. If the shipped-event append failed silently, the
  archival still proceeds, producing the exact reported inconsistency.

> [!NOTE]
> Baseline reconciliation (review-fix cycle 3, 2026-08-17): current source has since
> introduced the error-returning per-`ws` append seam (`ws.shipmentEventAppend`,
> defaulting to `appendItemEventErr`) that returns shipped-append failure via
> `shipmentEventAppendError` from `moveShipmentStatusWithHeadGuard`, so the shipped
> transition is no longer fire-and-forget. The best-effort description above is
> retained as the original observed root cause for provenance. The remaining planned
> work is the failure taxonomy and rollback around that seam plus the report-only
> doctor audit and CLI/MCP surfaces; see the plan's "Baseline reconciliation" section.

Success criteria:

* The `shipment_status_changed: shipped` event is durable before archival is
  allowed to proceed; archival cannot continue without it.
* On append failure, either the shipment and release-scope state are restored to
  active/unarchived, or an explicit indeterminate reconciliation error is
  returned (never a silent success).
* Behavior is identical across the shared CLI and MCP core path.
* A report-only doctor audit can detect historical archived shipments that carry
  `archived_status: shipped` without a shipped event, and never rewrites
  historical JSONL.

Scope boundaries (explicitly out of scope):

* No write-ahead journal, exactly-once, `OpID`/`PrevOpID`, or CAS-reconcile
  machinery (descoped at the root in the 099-S cycle and re-affirmed by the
  crash-window spike). Single-file replace is already atomic on both platforms.
* No rewriting or synthesizing of historical `.backlogit/` JSONL. The doctor
  audit is report-only.
* No change to the 142-F governed-registry work.

## Research Findings

Learnings retrieval (docs/compound, docs/decisions, docs/design-docs,
docs/exec-plans, docs/closure) returned HIGH confidence prior art:

* This scope is effectively the completion of 140-F Unit 1 (deferred from
  `106.033-T`): a reversible mutation envelope for a ShipShipment late-stage
  failure. Parent plan: `docs/exec-plans/2026-08-14-shipshipment-rollback-cas-plan.md`.
* F5 governed-mutation recovery contract already exists and should be reused
  rather than reinvented: `docs/design-docs/governed-mutation-recovery-contract.md`,
  `docs/exec-plans/2026-08-07-f5-idempotent-multi-mutation-envelope-plan.md`. F5
  wrapped commit-association, create-item plus dependency, and shipment
  membership, but NOT the shipment status-transition / archival envelope
  (`moveShipmentStatusWithHeadGuard`). That is the gap this work closes.
* Two-class durable-write contract:
  `docs/compound/2026-07-28-durable-writes-two-class-contract-commit-then-surface.md`.
  `ErrWriteNotApplied` (failure before the durable write) is safe to compensate
  or roll back. `ErrWriteIndeterminate` (append may have partially landed) must
  never be rolled back or blindly retried; instead commit-then-surface the built
  result wrapped in the indeterminate error. Indeterminate dominates any joined
  classification.
* Error-returning appender already exists: `appendItemEventErr` /
  `appendItemEventWithActorErr` in `internal/core/gate_evidence.go`. On the gated
  completion path a failed append already rolls back the transition so a
  completion never persists without its audit record. This is the exact pattern
  to extend to the shipment shipped-event.
* Report-only doctor precedent: 133-F `doctor --check-over-archived-features`
  (`docs/closure/2026-08-01-133-shipshipment-cascade-fix-closure.md`). It is a
  check-only audit cross-referencing shipment manifests against event provenance,
  registered through `DoctorOptions`, and reads authoritative archived state from
  raw Markdown, never the DB projection (`loadArtifact` omits `archived_status`).
  This plan applies the same read-raw-Markdown principle but via the full
  canonical queue-and-archive artifact scan (parsing each path once) rather than a
  per-ID `findArtifact` lookup, to stay duplicate-ID safe.
* Test-seam patterns:
  `docs/compound/2026-07-29-durable-writes-test-seam-patterns.md` (package-level
  `var` seams to inject `ErrWriteIndeterminate`, `t.Cleanup` restore, and tests
  overriding a package-global seam must not run `t.Parallel`).
* CLI/MCP parity:
  `docs/compound/2026-07-04-core-extraction-shared-eventwriter-append-serialization.md`
  (thread the caller's `*events.EventWriter` through the shared core function;
  never mint a fresh EventWriter inside the shared core function).
* Audit all entry points:
  `docs/compound/security-issues/2026-08-09-audit-all-entry-points-sharing-guarded-state-transition.md`
  (the generic `UpdateArtifactWithGate` path once bypassed ShipShipment guards;
  verify no alternate writer produces `archived_status: shipped` while skipping
  the durable event).

Existing rollback machinery available for reuse in `ShipShipment`:
`snapshotShipArtifacts` / `restoreShipArtifacts` over `rollbackIDs` (which already
include the shipment ID plus release scope), and the closure rollback defer that
fires `restoreShipArtifacts` when the closure returns an error.

## Options Evaluated

### Option A: Reuse F5 core.MutationEnvelope for the whole status-to-archival path

Wrap the active-to-shipped transition, the shipped-event append, and archival in
a single `core.MutationEnvelope` with ordered `Apply` / `Compensate` closures and
`errors.MutationPartialError` classification.

* Pros: single reusable abstraction (the 140-F plan-review advisory explicitly
  preferred this); consistent classification and MCP `mutation_partial` mapping;
  future status transitions inherit the guarantee.
* Cons: larger blast radius; ShipShipment already has a bespoke, heavily
  review-hardened locked closure with its own snapshot/rollback and membership
  lock (106-F, 133-F). Rewrapping the whole path risks regressing that hardening.
* Effort: high.

### Option B: Error-returning shipped-event append inside the existing envelope, classified per the two-class contract

Replace the best-effort `appendItemEvent` for the shipment status transition with
an error-returning append. Return the append error from
`moveShipmentStatusWithHeadGuard`. Classify it: `ErrWriteNotApplied` propagates so
the existing closure rollback restores active/unarchived state (archival never
runs); `ErrWriteIndeterminate` halts archival and surfaces an explicit
indeterminate reconciliation error without rolling back. Route the shipped-event
append through the shared ws-configured writer for parity. Add a report-only doctor
audit.

* Pros: minimal, surgical change that reuses existing primitives
  (`appendItemEventErr`, `snapshotShipArtifacts`/`restoreShipArtifacts`, the
  `MutationPartialError` classes, the `DoctorOptions` pattern); preserves the
  existing hardened closure; directly satisfies all four stash requirements.
* Cons: not the fully generalized envelope; a second future transition would
  repeat the classification wiring (acceptable, only the shipped transition is in
  scope now).
* Effort: medium.

### Option C: Best-effort to error-returning swap only, always roll back on failure

Swap the appender and always roll back the shipment to active on any append error.

* Pros: smallest change.
* Cons: violates the two-class contract by rolling back on
  `ErrWriteIndeterminate`, which can leave an active shipment with a
  partially-written shipped event. Rejected as unsafe.
* Effort: low.

## Trade-off Comparison

| Criterion | Option A | Option B | Option C |
|---|---|---|---|
| Complexity | High | Medium | Low |
| Blast radius on hardened closure | High | Low | Low |
| Two-class contract correctness | Yes | Yes | No (unsafe) |
| Reuse of existing primitives | Partial | High | Partial |
| Alignment with stash requirements | Yes | Yes | Partial |
| Regression risk to 106-F/133-F hardening | Elevated | Low | Low |

## Decision

Adopt Option B. Integrate an error-returning shipped-event append into the
existing shipment status-transition envelope and classify failures with the
two-class durable-write contract, reusing the existing snapshot/rollback closure
in `ShipShipment` for the `NotApplied` compensation and surfacing an explicit
indeterminate reconciliation error for the `Indeterminate` case. Achieve CLI/MCP
parity by passing the same ws through the shared core (no new EventWriter
parameter). Add a report-only doctor audit that reads raw Markdown to detect
archived shipments with `archived_status: shipped` but no shipped event, and the
shipped-but-unarchived indeterminate residue.

Covering release unit: a single feature (net reliability and data-integrity
capability, consistent with 140-F, 060-S shipment-state-integrity, and 067
archived-from-integrity being features). The bug is decomposed into
implementation, classification, tests, and the doctor audit surface.

Rationale: Option B satisfies every stash requirement while making the smallest,
safest change to a closure that has already absorbed multiple review-hardening
passes. It honors the learnings library warnings (never roll back an indeterminate
append; read archived state from Markdown; report-only doctor; no reintroduced WAL
machinery) and reuses existing, tested primitives instead of new infrastructure.

## Rejected Alternatives

* Option A rejected for now: the fully generalized envelope is the right long-term
  direction, but rewrapping the hardened ShipShipment closure exceeds this bug's
  scope and risks regressing 106-F/133-F guarantees. If a second transition later
  needs the same guarantee, promote the classification wiring into the F5 envelope
  as a follow-up.
* Option C rejected as unsafe: unconditional rollback on append failure violates
  the two-class contract for indeterminate writes.

## Unresolved Questions

* Whether the indeterminate path should also write a `.backlogit/reconcile/`
  sidecar artifact (117-S precedent) or only return the reconciliation error. The
  hard requirement is the explicit error; the reconcile artifact is optional
  alignment and is left to plan hardening to decide, kept minimal to avoid scope
  creep.
* Exact scope of the "all entry points" audit: at minimum verify that
  `UpdateArtifactWithGate` and any generic move/update path cannot drive a
  shipment to `shipped` while bypassing the durable event; the doctor audit is the
  standing safety net for any historical or future bypass.

## Risks and Mitigations

* Risk: rolling back an indeterminate append corrupts the audit trail. Mitigation:
  classify explicitly; never roll back `ErrWriteIndeterminate`; commit-then-surface.
* Risk: regressing the hardened ShipShipment closure. Mitigation: keep the change
  surgical inside the existing snapshot/rollback machinery; do not restructure the
  membership lock or the release-scope flow.
* Risk: doctor audit reads the DB projection and misses `archived_status`.
  Mitigation: read raw Markdown frontmatter via the full canonical
  queue-and-archive artifact scan (parsing each path once, `artifactRef` extended
  to carry `archived_status`), not a per-ID `findArtifact` second lookup; this
  follows the 133-F report-only precedent while avoiding the duplicate-ID mismatch
  a per-ID lookup would risk.
* Risk: CLI/MCP divergence. Mitigation: route through the shared
  `appendItemEventErr(ctx, ws, ...)` so both surfaces use the same ws-configured
  writer; do not add an EventWriter parameter or mint a fresh writer.
* Risk: in-process envelope cannot survive process kill or power loss between the
  status persist and the append. Mitigation: state this boundary honestly; the
  doctor audit is the detection surface and recovery is manual reconciliation, not
  idempotent rerun (ShipShipment refuses a non-active shipment).
