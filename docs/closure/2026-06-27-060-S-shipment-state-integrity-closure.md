---
chunk_strategy: h1-h2-h3
description: 'Post-merge operational closure for shipment 060-S — atomic ClaimShipment rollback (pre-claim snapshot + activated-ID tracking, fallible post-activation read-back eliminated) and stale blocked_reason clearing at all three status choke points (PR #143, merge 7a51904b)'
doc_type: closure
docline:
    ms.date: 2026-06-27T00:00:00Z
    ms.topic: reference
ingested_at: "2026-06-28T00:18:00Z"
schema_version: "1.0"
source: docs/closure/2026-06-27-060-S-shipment-state-integrity-closure.md
title: 060-S Shipment State Integrity — Post-Merge Operational Closure
---

# Operational Closure — Shipment 060-S (Shipment State Integrity)

- **Shipment**: 060-S — Shipment State Integrity
- **Feature**: 061-F (2 tasks: 061.001-T, 061.002-T — both done/archived)
- **PR**: #143 — *Shipment State Integrity — atomic claim rollback + stale blocked-metadata clearing*
- **Merge commit**: `7a51904bc159d0f16aa5a9d8866e0bd4c324717d` (merge commit on `main`, P-009 compliant; squash/rebase disabled repo-wide)
- **Closure branch**: `post-merge/061-shipment-state-integrity`
- **Mode**: post-merge
- **Verification**: `docs/closure/2026-06-27-060-S-shipment-state-integrity-runtime-verification.md` — **PASS**
- **Readiness**: **READY** (the change is already merged; this artifact records monitoring + rollback for the shipped scope)
- **ID-offset note**: benign `NNN-S → (NNN+1)-F` cosmetic offset; 060-S legitimately carries 061-F + 061.001-T / 061.002-T (per `docs/decisions/2026-06-25-shipment-manifest-drift-determination-deliberation.md`). The unrelated shipment 061-S (carries 062-F) was NOT part of this closure and remains `queued`.

## Summary of the change

Fixed two shipment-lifecycle state-integrity bugs (origin stashes FE806724, 36F1CB1A):

- **061.002-T — Rollback partial shipment activation.** `ClaimShipment` is now atomic: it snapshots pre-claim state, tracks the IDs it activates, and on *any* mid-flight activation failure calls `rollbackShipmentClaim` to revert activated items to `queued` and restore the shipment. The fallible post-activation read-back was eliminated, so the torn-state window is closed *by construction* rather than detected after the fact. Files: `internal/core/shipment_lifecycle.go` (`ClaimShipment`, `rollbackShipmentClaim`).
- **061.001-T — Clear stale returned-item blocked metadata.** A new `clearStaleBlockedReason` helper (`internal/core/shipment.go`) is applied at all three status choke points — `UpdateArtifact` (`artifacts.go:481`), `setArtifactStatus` and `cascadePersistedParentStatuses` (`shipment_lifecycle.go:484`, `:523`) — so an item returning to the backlog clears stale `blocked_reason` on re-entry. Gotcha captured: the `validate_status_transition` hook only permits `blocked→active`, so the lifecycle `blocked→queued` path bypasses that hook and must clear the metadata itself.

## Invariants to preserve

1. A failed `ClaimShipment` leaves **no** partial activation — shipment and every touched item revert to `queued` (no torn state).
2. A successful `ClaimShipment` activates the full included scope and never relies on a fallible post-activation read-back to confirm state.
3. Returned/re-entering items carry **no** stale `blocked_reason`; the field is cleared at every status choke point on any transition away from `blocked`.
4. An item that is still `blocked` retains its `blocked_reason` (clear is gated on a real status change).
5. `ShipShipment` re-archives pre-archived tasks with canonical `archived_from` and reports `returned_ids: []` on a clean ship.

## Pre-deploy audits

Not applicable — no migration, feature flag, config, or access change. Pure library/CLI correctness fix. Already merged.

## Deployment / rollout path

Merge-only. The fix ships as part of the `backlogit` binary; no service deploy, no data migration, no rollout window. Consumers pick it up on next binary build (repo-root `backlogit.exe` already rebuilt from `7a51904b`).

## Post-deploy checks (already executed in this closure)

- `go test ./internal/core/...` green, including the 5 claim-rollback / stale-blocked integrity tests (see verification report).
- Live `shipment ship 060-S` → `returned_ids: []`, all manifest items `archived`, `061-S`/`062-F` untouched (`queued`).
- `doctor --check-archived-from` → 0 self-referential; only the 2 known malformed legacy records (`038-DL`, `039-DL`).

## Healthy signals

- A claim that fails partway leaves **no** partially activated items — shipment + items revert to `queued`.
- Returned items re-enter the backlog with **no** lingering `blocked_reason`.
- Ships report `returned_ids: []` and reconcile pre/post both PROCEED.

## Failure signals (rollback triggers)

- A shipment or any of its items stuck in `active` after a failed/aborted claim (torn state).
- A returned item still showing a `blocked_reason` after transitioning out of `blocked`.
- A ship producing unexpected `returned_ids` or a `shipment-reconcile` HALT.

## Monitoring plan

- **CI guardrails**: the `internal/core` shipment-lifecycle / status-transition test suite (`shipment_atomic_test.go`, `shipment_state_integrity_test.go`) runs on every PR via `test (1.24)`.
- **Per-ship gate**: the `shipment-reconcile` GI/GR double-entry check (pre + post) on every Ship Step 6 closure.
- No dashboards/alerts — this is a CLI/library correctness invariant, watched by tests + the reconcile gate, not a live service metric.

## Rollback trigger & procedure

- **Trigger**: any healthy-signal inversion above (torn shipment state after a failed claim, or lingering blocked metadata on returned items) traced to this change.
- **Procedure**: `git revert 7a51904bc159d0f16aa5a9d8866e0bd4c324717d` (merge commit; revert the merge), rebuild the binary, re-run `go test ./internal/core/...`. No data migration to unwind — the change is pure in-memory/state-transition logic plus metadata clearing.

## Risky-action record

No `strict-safety`-class destructive action. The single state mutation in this closure (`shipment ship 060-S`) was gated by pre-mode reconcile (PROCEED), produced the expected fully-reconciled archive (post-mode PROCEED), and passed the P-007 deleted-file guard (0 deletions).

## Validation window & owner

- **Window**: covered indefinitely by CI + the reconcile gate; no time-boxed observation needed for a merge-only library fix.
- **Owner**: repository maintainer (softwaresalt) via CI; Ship agent for per-shipment reconcile.

## Source artifact cleanup

- `061-F` carries **no** `source_stash_id` / `source_deliberation_id` custom field → custom-field-driven cleanup is a **no-op** (logged, nothing archived).
- Origin stashes `FE806724` and `36F1CB1A` are **already absent** from the stash (harvested by Stage upstream) → nothing to remove.
- No orphaned `-DL` backlog item references this shipment.
- Durable design rationale (`docs/decisions/2026-05-22-new-bug-backlog-grouping.md`, drift determination `2026-06-25-…-deliberation.md`, exec plan `docs/exec-plans/2026-05-22-shipment-state-integrity-plan.md`) is **kept** as institutional knowledge — not retired.

## Feedback into the harness

- New compound learning authored: `docs/compound/best-practices/atomic-multi-item-claim-rollback-and-stale-blocked-clearing-2026-06-27.md`.
- Compound-refresh review: `docs/closure/2026-06-27-060-S-shipment-state-integrity-compound-refresh.md` (overlapping rollback/atomicity entries classified **keep** — distinct mechanisms).
- No new safety mode, verification pattern, or reviewer gap surfaced.

## Follow-ups

- None blocking. Pre-existing out-of-scope item: 2 malformed legacy `archived_from` records (`038-DL`, `039-DL`) remain flagged-only, tracked independently.

## Readiness

**READY** — merged change with PASS verification, green CI integrity suite, clean live ship (atomic, no torn state, no stale blocked metadata), and an explicit revert-based rollback path.
