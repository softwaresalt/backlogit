---
chunk_strategy: h1-h2-h3
description: 'A completeness pattern discovered during 117-S formal-gate-evidence review (round 8, the most severe finding of the whole cycle): a NEW security control was added to the intended, primary entry point (ShipShipment) for a state transition, but a SEPARATE, pre-existing, general-purpose entry point (UpdateArtifactWithGate, shared by the generic move_item/update_item MCP tools) could reach the identical state transition with ZERO of the new protections, because its own applicability filter was narrowly scoped to a different artifact type and silently fell through to an ungated write for anything else.'
doc_type: learning
docline:
    date: 2026-08-09T00:00:00Z
    severity: critical
    tags:
        - security
        - review
        - core
        - mcp
        - cli
        - authorization
        - defense-in-depth
schema_version: "1.0"
source: docs/compound/security-issues/2026-08-09-audit-all-entry-points-sharing-guarded-state-transition.md
title: 'When adding a security control for a state transition, audit EVERY code path that can produce that transition, not just the intended one'
---

# Audit All Entry Points Sharing a Guarded State Transition

Graduated from shipment 117-S (Formal Gate F1 — evidence authenticity and
manifest binding; feature 106-F, tasks 106.003-T through 106.011-T; PR #333,
merge `23d88904faf917a4f4003042f185de9b4e568530`). This was the single most
severe finding across an unusually deep 10-round Copilot PR review cycle —
worse than the earlier `--force` bypass (round 6) because it required no
force flag or special privilege at all, just calling a different, ordinary,
pre-existing tool.

## Rule — A new state-transition guard is only as strong as its weakest reachable path, not its intended one

### Problem

`ShipShipment` (`internal/core/shipment_lifecycle.go`) gained extensive
formal-gate-evidence protection this shipment: member-evidence verification,
manifest-binding signing, membership locking, head-drift bracketing. All of
it lives INSIDE `ShipShipment` and its helpers.

But `ShipShipment` is not the only code path that can move a shipment artifact
to `status: shipped`. The general-purpose `UpdateArtifactWithGate`
(`internal/core/gate_transition.go`) — shared by the `backlogit_move_item`
and `backlogit_update_item` MCP tools, and their CLI equivalents — decides
whether the pre-task-completion gate applies via `gateApplies` /
`gateWouldApplyButForBroker`, both of which are hardcoded to
`artifact.ArtifactType == "task" || "subtask"`. A shipment ALWAYS fails that
check, regardless of enforcement state, and falls straight through to a plain
`updateArtifactUngated` write — completely bypassing every protection just
added to `ShipShipment`, with no force flag, no elevated privilege, just a
normal `move` call:

```
backlogit move 117-S --status shipped
```

under formal enforcement would have silently succeeded with ZERO member
verification or manifest binding, while `ShipShipment`'s own extensive
protections sat entirely unreachable.

### Fix

Add an EXPLICIT guard at the shared choke point (`UpdateArtifactWithGate`),
BEFORE the artifact-type-scoped `gateApplies` branch even runs:

```go
if peek != nil && peek.ArtifactType == "shipment" {
    if newStatus, _ := updates["status"].(string); newStatus == string(ShipmentShipped) && ws.formalGateEnforced() {
        return nil, nil, formalGateShipmentRefusal(id, "shipments must be shipped via ShipShipment, not a direct status update, while formal gate evidence is enforced")
    }
}
```

Fixing it at the SHARED entry point (not in `handleMoveItem` alone) closes the
gap for every current AND future caller of `UpdateArtifactWithGate` — the MCP
tool, the CLI command, and any code written later that reuses the same core
function — rather than patching one call site and leaving the underlying
function itself still exploitable through a different caller.

### Why this is a distinct class from "missing validation"

This is not a case of a single check being wrong or missing input validation —
it is a case of a NEW protection being added to only ONE of several REACHABLE
paths to the same effect. The review-time question that would have caught this
earlier: **"before adding a guard to function A, what ELSE in this codebase can
produce the same state change as A?"** — a `grep` for other callers/writers of
the same status field, or other functions that reach the same terminal state,
is cheap and should be a standard step whenever a security control targets a
specific state transition rather than a specific function.

### Detection technique

Look for state-mutation entry points that are SHARED/GENERIC (a `move`,
`update`, or `set-status` primitive used across many artifact types) sitting
alongside a NEWLY-HARDENED, TYPE-SPECIFIC completion path (`ShipShipment` here,
but the same shape applies to any "special lifecycle function" pattern: order
checkout vs. generic record update, workflow-engine transition vs. direct SQL
UPDATE, etc.). If the type-specific path enforces something the generic path's
OWN applicability check does not explicitly also enforce, the generic path is
a bypass, not a coincidence.

## Related

- `docs/compound/security-issues/2026-08-09-authenticate-before-filter-security-check-ordering.md` —
  a sibling completeness lesson from the same review cycle, at the level of a
  single function's internal ordering rather than cross-function reachability.
- PR #333 review round 8 (this finding) and round 6 (`--force` bypassing the
  same feature's guarantees through a DIFFERENT, narrower gap in the intended
  path itself).
