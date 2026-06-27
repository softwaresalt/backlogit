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

## Next steps
- Open PR, request Copilot review, drive CI green, resolve Copilot threads. HALT at merge gate
  for operator approval (merge commit strategy, P-009). Post-merge closure ships 060-S and
  archives 061-F + tasks in a separate step.
