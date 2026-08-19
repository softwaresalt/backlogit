---
chunk_strategy: h1-h2-h3
doc_type: decision
source: docs/decisions/2026-08-18-shipment-shipped-prevention-hardening-deliberation.md
schema_version: "1.0"
title: "Prevention hardening for non-ShipShipment shipped transitions and archive stamping"
description: "Deliberation on closing the two non-ShipShipment paths that can produce archived_status shipped without a durable shipped event"
topic: "Prevention hardening follow-up to 143-F / 127-S shipped-event durability"
depth: "deep"
decision_status: "decided"
promoted_to: "plan"
source_stash_id: "47B48DB0"
linked_artifacts:
  - "docs/exec-plans/2026-08-18-shipment-shipped-prevention-hardening-plan.md"
tags:
  - "shipment-lifecycle"
  - "prevention-hardening"
  - "audit-log"
  - "archive"
  - "governed-envelope"
---

## Problem Frame

Shipment feature 143-F (shipment 127-S, both now archived) shipped the DETECTION
half of shipped-event integrity: the governed `ShipShipment` path appends a
durable `shipment_status_changed` event with `status: shipped`, and a report-only
doctor audit (`check_shipped_event_completeness`) flags any archived shipment
whose `archived_status: shipped` lacks that durable event. Detection tells us
residue exists; it does not stop residue from being created.

Two non-ShipShipment paths in the current `origin/main` code can still PRODUCE
exactly the residue the doctor flags:

* **Generic status transition (guard 1)**: `UpdateArtifactWithGate`
  (`internal/core/gate_transition.go:109-116`) only rejects a shipment moving to
  `shipped` when `ws.formalGateEnforced()` is true. When formal enforcement is
  OFF (the default unless `BACKLOGIT_FORMAL_GATE_REQUIRED` or workspace opt-in is
  set), a generic `move_item` / `update_item` can drive a shipment to `shipped`
  with no durable event and no gate. The gate broker is not even wired when
  disabled, so a gate-level rule is a no-op in exactly the case that must be
  covered.
* **Archive stamping (guard 2)**: `ArchiveItem` (`internal/core/archive.go:222-223`)
  copies the pre-archive status into `archived_status` unconditionally, so a
  shipment already sitting at `shipped` (however it got there) is archived as
  `archived_status: shipped` with no durable-event verification. The
  status-transition validation hook is registered only for `HookUpdateArtifact`,
  not `HookArchiveItem`, so archive silently bypasses transition checks.

This item is the PREVENTION complement raised by the Architecture Strategist
plan-review persona and the adversarial multi-model review during 143-F, and was
scope-deferred to respect the 143-F audit-log-completeness bug boundary.

Who cares: operators and agents relying on the shipped-event audit trail for
release traceability; the doctor audit becomes a true safety net only when the
paths that create residue are closed.

Success criteria:

* Both non-ShipShipment paths refuse to produce `shipped` / `archived_status: shipped`
  without a durable shipped event, independent of the formal-gate flag.
* The governed `ShipShipment` envelope remains the one sanctioned producer and is
  unaffected.
* Legitimate `done` / `abandoned` closures and the P-015 single-artifact safe-close
  path are NOT blocked.
* MCP and CLI behave identically; there is no MCP force lever.
* Guards fail closed on missing or unrecognized evidence.

## Research Findings

Grounded in the merged `origin/main` code (143-F/127-S ground truth; the stash's
cited 143-F plan/review docs are not present on `origin/main`, so the merged code
and tests are authoritative) and the compound library (HIGH-confidence retrieval):

* Shared predicate already exists: `shippedEventPresence(...)`
  (`internal/core/doctor.go:613-650`) is the durable-event check the doctor uses.
  Both guards should reuse this single predicate so prevention and detection scan
  the same literal contract (`shipment_status_changed` / `status: shipped`).
* Enforce guard 1 in the core mutation validator, not the gate. The status-transition
  hook does not fire on archive, so guard 2 needs its own explicit check on the
  archive seam (`2026-07-20-ship-gate-descoped-archived-member-exemption`,
  `2026-07-30-task-only-typed-metadata-seam-enforce-before-schema`).
* Read `archived_status` and provenance from Markdown, not the DB index
  (`selectCols`/`loadArtifact` omit `archived_status`); use one authoritative load
  for both the guard check and the mutation
  (`2026-07-28-attach-commit-repersist-must-reload-from-markdown`).
* Durable-event append must follow the two-class commit-then-surface contract:
  never roll back an indeterminate write; surface `ErrWriteIndeterminate` vs
  `ErrWriteNotApplied` (`2026-07-28-durable-writes-two-class-contract-commit-then-surface`).
* Governed MCP/CLI parity is mandatory and must be tested through the authoritative
  registry against durable JSONL + Markdown, not just the index
  (`2026-08-15-governed-parity-fixtures-must-dispatch-authoritative-registry`).
* Scope the refusal precisely to `archived_status == shipped` reached outside the
  governed path; do not block `done`/`abandoned`/P-015 safe-close
  (`2026-07-31-p015-single-artifact-safe-close-for-partial-feature-shipments`).
* Merged is not operative: the pinned `backlogit` binary must be rebuilt before the
  guards protect real `.backlogit/` operations; verify at closure
  (`2026-08-01-self-hosted-cli-version-skew-merged-fix-not-yet-operative`).

## Options Evaluated

### Option A: Two local guards at their seams reusing the shared predicate

Enforce guard 1 in the core generic transition path (`UpdateArtifactWithGate`,
with a defense-in-depth check in `isValidShipmentTransition`) and guard 2 in
`ArchiveItem`, both reusing `shippedEventPresence`. Gate-independent, Markdown-authoritative,
fail-closed, scoped to `shipped`.

* Pros: closes both real holes at the exact seam each traverses; covers all
  surfaces because MCP/CLI funnel through the core functions; reuses the same
  predicate detection uses so contract cannot drift; smallest blast radius.
* Cons: two enforcement points rather than one; requires careful scoping so
  legitimate closures are not blocked.
* Effort: medium.
* Fit: strongly matches all success criteria and every compound gotcha.

### Option B: Centralize both rules in the shipment-status validator

Route both guards through `isValidShipmentTransition` / a single shipment-status
validator.

* Pros: single conceptual rule location.
* Cons: `ArchiveItem` does not flow through the transition validator (hook gap),
  so guard 2 would silently do nothing — this is the documented failure mode from
  `2026-07-20-ship-gate-descoped-archived-member-exemption`. Does not actually
  cover the archive path.
* Effort: low but incorrect.
* Fit: fails the archive success criterion.

### Option C: Make the formal gate default-on / enforce via the gate broker

Rely on the existing formal gate by defaulting enforcement on.

* Pros: reuses existing gate machinery for guard 1.
* Cons: the gate broker is not wired when disabled, so it cannot cover the OFF
  case that is the whole point; changes an operator-facing default; does nothing
  for the archive path. Broad, risky, and off-target.
* Effort: medium.
* Fit: fails gate-independence and archive criteria.

## Trade-off Comparison

| Criterion | Option A | Option B | Option C |
|---|---|---|---|
| Covers generic-transition hole (gate OFF) | Yes | Partial | No |
| Covers archive-stamping hole | Yes | No | No |
| Reuses detection predicate (no drift) | Yes | Partial | No |
| Avoids operator-facing default change | Yes | Yes | No |
| Blast radius | Low | Low | High |
| Alignment with compound gotchas | Full | Fails hook-gap | Fails gate-wiring |

## Decision

Adopt Option A and synthesize a covering FEATURE: "Prevention hardening: reject
non-ShipShipment shipped transitions and archive stamping" (next id `144-F`). The
stash is task-shaped, and its natural parent 143-F is archived, so a focused
prevention-hardening covering feature is synthesized rather than attaching a task
to an archived feature.

Decompose into two guard tracks, each red-harness-first (TDD), plus cross-cutting
parity tests and docs, mirroring the 143-F decomposition convention:

* Guard 1 track: RED harness (tests) then core implementation.
* Guard 2 track: RED harness (tests) then core implementation.
* Governed MCP/CLI parity + integration tests once both guards land.
* Docs: design-doc rationale plus a compound learning graduating the
  prevention/detection pairing and the ArchiveItem hook-gap.

Enforce in core seams, gate-independent, reuse `shippedEventPresence`, load
`archived_status` from Markdown, fail closed, and scope strictly to
`archived_status == shipped`.

## Rejected Alternatives

* Option B is rejected: the archive path bypasses the transition validator, so a
  centralized transition rule cannot enforce guard 2.
* Option C is rejected: it cannot cover the gate-OFF case (broker unwired when
  disabled), changes an operator-facing default, and ignores the archive path.

## Unresolved Questions

* Final sentinel-error taxonomy for the two refusals (for example
  `ErrShipmentShippedRequiresEnvelope` and `ErrArchiveShippedRequiresEvent`) and
  whether they live in `internal/errors` alongside `ErrShipmentConflict`.
* Whether to extract `shippedEventPresence` into a shared non-doctor location so
  both `gate_transition.go` and `archive.go` can import it without a doctor
  dependency cycle — resolve during planning.
* Whether guard 1 also hardens `isValidShipmentTransition` as defense-in-depth or
  relies solely on the `UpdateArtifactWithGate` seam.

## Risks and Mitigations

* False positives blocking legitimate closures. Mitigation: scope to
  `archived_status == shipped`; reuse the exact predicate detection uses; add
  explicit tests for `done`, `abandoned`, and P-015 safe-close.
* Durable-write indeterminacy on any new append. Mitigation: honor the
  commit-then-surface two-class contract; never roll back an indeterminate write.
* MCP/CLI behavior drift. Mitigation: governed-parity tests dispatched through the
  authoritative registry against durable JSONL + Markdown; no MCP force lever.
* Merged-but-not-operative binary skew. Mitigation: rebuild the pinned binary and
  verify the embedded commit at closure; the doctor audit remains the after-the-fact
  net.
