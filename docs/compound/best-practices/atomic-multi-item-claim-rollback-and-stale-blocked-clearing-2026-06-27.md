---
chunk_strategy: h1-h2-h3
description: 'Atomic multi-item state transitions must snapshot pre-mutation state, track the exact set they mutate, and roll that set back on any mid-flight failure — never confirm via a fallible post-mutation read-back. Derived/stale metadata (blocked_reason) must be cleared at ALL re-entry choke points, because the lifecycle blocked->queued path bypasses the validate_status_transition hook that only allows blocked->active.'
doc_type: learning
docline:
    category: best-practices
    component: shipment-lifecycle
    date: 2026-06-27T00:00:00Z
    file_path: internal/core/shipment_lifecycle.go
    message: 'ClaimShipment activated items then confirmed via a fallible post-activation read-back, leaving a torn-state window on mid-flight failure; returned items kept a stale blocked_reason because blocked->queued bypasses the validate_status_transition hook'
    problem_type: broken-atomicity
    resolution_type: pre-mutation-snapshot + activated-set-tracking + rollback + clear-derived-metadata-at-all-choke-points
    resolved: true
    root_cause: 'multi-item activation had no rollback and relied on a fallible post-activation read-back to detect torn state; blocked_reason was only cleared on the hook-validated blocked->active path, not on the lifecycle blocked->queued return path'
    severity: high
    tags:
        - reliability
        - atomicity
        - rollback
        - state-machine
        - shipment-lifecycle
        - stale-metadata
        - blocked-reason
        - data-integrity
        - invariant-by-construction
    shipped_in: 060-S
    merge_commit: 7a51904bc159d0f16aa5a9d8866e0bd4c324717d
ingested_at: "2026-06-28T00:20:00Z"
schema_version: "1.0"
source: docs/compound/best-practices/atomic-multi-item-claim-rollback-and-stale-blocked-clearing-2026-06-27.md
title: Atomic multi-item claim needs pre-mutation snapshot + activated-set rollback (not a fallible read-back), and stale derived metadata must clear at every re-entry choke point
---

# Atomic multi-item state transitions + invalidating stale derived metadata

## Problem

Two distinct shipment-lifecycle defects shipped together as "Shipment State Integrity" (060-S / 061-F):

1. **Torn claim state.** `ClaimShipment` activated each item in the shipment scope and
   then attempted to *confirm* the result with a post-activation read-back. If activation
   failed partway through (e.g., one item could not transition), the already-activated
   items were left `active` while the shipment was in an inconsistent state — a torn
   window. Worse, the confirmation read-back was itself fallible, so the detection path
   could fail to even notice the torn state.

2. **Stale `blocked_reason` on return.** When an item was returned to the backlog
   (`blocked → queued`), its `blocked_reason` metadata lingered. The clearing logic only
   ran on the hook-validated `blocked → active` transition, so any other re-entry path
   left stale "why was this blocked" text attached to a now-unblocked item.

## Root cause

- **Atomicity:** the multi-item activation had **no rollback** and treated a *fallible
  post-mutation read* as the integrity check. Detecting torn state after the fact is
  strictly weaker than preventing it — and the detector shared the same failure modes as
  the mutation.
- **Stale metadata:** `blocked_reason` was cleared at exactly one choke point. The
  `validate_status_transition` hook only allows `blocked → active`, so the lifecycle
  `blocked → queued` return path **bypasses that hook entirely** and never reached the
  single clearing site.

## Resolution

1. **Pre-mutation snapshot + activated-set tracking + rollback** (`internal/core/shipment_lifecycle.go`):
   - `ClaimShipment` snapshots the pre-claim shipment + item state and records the exact
     IDs it activates as it goes.
   - On *any* mid-flight activation failure it calls `rollbackShipmentClaim`, which reverts
     every tracked activated item to `queued` and restores the shipment.
   - The fallible post-activation read-back was **deleted**. The invariant ("either the
     whole scope activates, or nothing does") now holds **by construction**, not by
     after-the-fact detection.

2. **Clear derived metadata at ALL choke points** (`internal/core/shipment.go` helper
   `clearStaleBlockedReason`, applied at `UpdateArtifact` → `artifacts.go:481`,
   `setArtifactStatus` and `cascadePersistedParentStatuses` → `shipment_lifecycle.go:484`,
   `:523`): every status-change path that can move an item out of `blocked` clears
   `blocked_reason`, gated on a real transition away from `blocked` (a still-blocked item
   keeps its reason).

## Prevention / reusable principles

- **Prefer "atomic by construction" over "detect-then-repair."** For any operation that
  mutates N items, snapshot the starting state, track the precise set you mutate, and roll
  *that set* back on failure. Do not depend on a post-mutation read to discover torn state —
  the read can fail the same way the mutation did.
- **Never let a fallible read-back be your integrity guarantee.** If correctness depends on
  a confirmation query that can error, the guarantee is only as strong as the query.
- **Derived/cached metadata must be invalidated at every state-entry path, not just the
  "happy" one.** Enumerate *all* transitions that can leave the source state. A validation
  hook that only covers one edge (`blocked → active`) silently skips the others
  (`blocked → queued`); the metadata clear must live on the lifecycle path, independent of
  the hook.
- **Map the choke points explicitly.** Here there were three (`UpdateArtifact`,
  `setArtifactStatus`, `cascadePersistedParentStatuses`); a fix applied to only one would
  have passed its targeted test while leaving the cascade/persist paths broken.

## Evidence

- Tests (fresh, `-count=1`): `TestClaimShipment_RollsBackOnMidFlightActivationFailure`,
  `TestClaimShipment_SuccessActivatesAllItems`, `TestClaimShipment_ActivatesIncludedScope`,
  `TestUpdateArtifact_ClearsStaleBlockedReasonOnReentry`,
  `TestUpdateArtifact_KeepsBlockedReasonWhileStillBlocked` — all PASS
  (`internal/core/shipment_atomic_test.go`, `shipment_state_integrity_test.go`).
- Live production-path confirmation: `shipment ship 060-S` returned `returned_ids: []`
  (no item left torn) and archived the full manifest cleanly (merge `7a51904b`, PR #143).
- Closure: `docs/closure/2026-06-27-060-S-shipment-state-integrity-closure.md`.
- Plan: `docs/exec-plans/2026-05-22-shipment-state-integrity-plan.md`.

## Applicability

Reuse for any multi-entity claim/activation/transaction over a state machine (shipment
claim, batch assignment, multi-row status flips) and for any cached/derived field whose
validity depends on a state the entity can leave through more than one edge.
