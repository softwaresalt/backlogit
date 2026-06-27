---
doc_type: memory
date: 2026-06-27
agent: Ship
shipment: 060-S
feature: 061-F
branch: feat/061-shipment-state-integrity
---

# Ship Session — 060-S Shipment State Integrity

- **Date**: 2026-06-27
- **Agent**: Ship
- **Shipment**: 060-S (active) — carries feature 061-F (benign NNN-S to (NNN+1)-F ID offset; cosmetic, not corruption)
- **Branch**: `feat/061-shipment-state-integrity` (off `main` @ 2b21a529)
- **Tasks**: 061.002-T (high, rollback) + 061.001-T (medium, stale blocked metadata)

## Outcome
- [x] **061.002-T** done — `ClaimShipment` atomic rollback (commit a1ba3d4c).
- [x] **061.001-T** done — clear stale `blocked_reason` on backlog re-entry (commit a1ba3d4c).
- Feature 061-F + shipment 060-S left **active** for post-merge closure (not shipped this run).

## How the fixes integrate with CURRENT code (plan was 2026-05-22, stale)
- The plan named only `internal/core/shipment_lifecycle.go`. Grounded in current code instead:
  - **Rollback (061.002-T)**: `ClaimShipment` snapshots the pre-claim shipment (`cloneArtifact`),
    tracks `activatedIDs`, and on any mid-flight failure (item load/activate error OR the
    post-activation shipment read-back) calls new `rollbackShipmentClaim`. Items revert to
    queued in reverse order; the existing parent-status cascade in `setArtifactStatus` self-heals
    cascade-activated parents (even parents NOT in the manifest). Shipment is restored via
    `persistArtifact(preClaimShipment, relocate=true)` — `MoveShipmentStatus` cannot be used
    because `active->queued` is not a valid shipment transition. Mirrors the existing
    `rollbackReturnedBlockedArtifacts` snapshot/restore pattern.
  - **Stale blocked metadata (061.001-T)**: new `clearStaleBlockedReason(artifact, previousStatus)`
    helper applied at BOTH status-change choke points — `setArtifactStatus` (lifecycle
    return-to-backlog, e.g. `returnUnreleasedFeatureItems` during `ShipShipment`) and
    `UpdateArtifact` (operator `move`).

## Key discovery (gotcha worth remembering)
- The `validate_status_transition` pre-hook (`hooks.DefaultTransitions`) only allows
  `blocked -> active` via `UpdateArtifact` — **NOT** `blocked -> queued`. So a blocked item
  re-enters the backlog two ways: operator `move` (blocked->active, hook-guarded) or the
  lifecycle `setArtifactStatus` path which sets returned items to `queued` and BYPASSES the hook.
  The stale-`blocked_reason` clear had to live at both choke points to cover both re-entry paths.

## Quality gates
- `go test ./...` PASS; `go vet ./...` PASS; `golangci-lint run` PASS (0).
- `gofmt -l .` flags 3 changed files but it is CRLF-only noise (`core.autocrlf=true`); committed
  blobs are LF (verified 0 CR) so CI gofmt on Linux passes.

## Review gate
- code-review + Go Reviewer: no P0/P1. One P2 fixed (shipment read-back failure now routed
  through rollback). P3s (best-effort rollback limits; pre-existing `cloneArtifact` shallow
  Links/nested-slice copy not triggered here) acknowledged, out of scope.

## PR #143 + Copilot review-fix cycles (all complete)
- PR #143: https://github.com/softwaresalt/backlogit/pull/143 (branch `feat/061-shipment-state-integrity`).
- Three Copilot review-fix cycles, each TDD (red→green), each its own commit:
  - **Cycle 1 — `7a1215f0`**: cascade choke point. `cascadePersistedParentStatuses` now calls
    `clearStaleBlockedReason(parent, previous)` so a parent recomputed off `blocked` by a child
    cascade also clears stale `blocked_reason` (third choke point for 061.001-T). New test
    `TestCascadeParentStatus_ClearsStaleBlockedReason`.
  - **Cycle 2 — `c5da0e64`**: eliminated the final post-activation `GetShipment` read-back in
    `ClaimShipment`. The shipment artifact is never mutated by item activation (manifest items are
    children of the feature, not the shipment), so the already-loaded snapshot is returned directly
    — removing the only fallible op after the last activation (torn-state window closed by
    construction). Success test now asserts the returned shipment carries its full manifest.
  - **Cycle 3 — `5e895193`**: hardened rollback. (a) `activatedIDs` records each item *before*
    `setArtifactStatus` so an item persisted active before a cascade failure is still reverted
    (queued→queued revert is a safe no-op); (b) `rollbackShipmentClaim` guards a nil `claimErr`
    so rollback can never collapse to silent success; (c) mid-flight rollback test now asserts the
    cascade-activated parent's on-disk state via `loadArtifact`.
- Final confirmation Copilot review on HEAD `5e895193` posted **no new threads**. All 5 review
  threads resolved (0 unresolved). Replies posted + threads resolved via REST + GraphQL.

## Merge-gate status (HALTED for operator)
- CI all green on HEAD `5e895193`: test (1.23), test (1.24), CLI Reference Drift, Docline frontmatter gate.
- `mergeable: MERGEABLE`; `mergeState: BLOCKED` = branch protection requires an approving review
  (Copilot only COMMENTED) — expected; operator provides the approving review + admin merge.
- P-009 verified: repo allows merge commit only (`allow_squash_merge: false`,
  `allow_rebase_merge: false`).
- Both tasks `done`; feature 061-F + shipment 060-S left **active** for post-merge closure.

## Next steps
- Operator: review + merge PR #143 via **merge commit** (P-009). After merge, run post-merge
  closure as a separate step: ship 060-S and archive 061-F + tasks.
