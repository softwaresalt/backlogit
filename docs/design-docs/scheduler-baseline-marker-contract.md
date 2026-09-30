---
chunk_strategy: h1-h2-h3
description: "Operator and consumer contract for the scheduler_baseline_claim marker (scheduler-baseline-marker/v1): pinned key and value shape, the copy-pasteable read recipe with exact JSON paths, the normative option A three-part predicate, the lifecycle table with verification status, and residuals R1-R3 (173.005-T)."
doc_type: design
status: draft
created: 2026-09-30
schema_version: "1.0"
source: docs/design-docs/scheduler-baseline-marker-contract.md
title: "Scheduler-Baseline Marker Contract (scheduler-baseline-marker/v1)"
---

# Scheduler-Baseline Marker Contract (scheduler-baseline-marker/v1)

## Summary

When `ClaimShipment` activates a shipment, it writes a marker on every
manifest member it activates. External schedulers, including the autoharness
wave scheduler, read the marker to tell a claim-activated item from an item
that became active by any other (organic) route. This document is the
published contract for that marker.

Sources: the plan
`docs/exec-plans/2026-09-13-shipment-claim-scheduler-reconciliation-plan.md`
(unit U5) and the decision
`docs/decisions/2026-09-28-173f-marker-lifecycle-option-a-decision.md`
(option A). Feature `173-F`, shipment `154-S`.

## Key and value shape

* **Key:** `scheduler_baseline_claim`, read at
  `custom_fields.scheduler_baseline_claim` (constant
  `schedulerBaselineClaimKey`, `internal/core/shipment_lifecycle.go`).
* **Value:** the ID of the activating shipment, as a non-empty string (for
  example `"154-S"`).
* The key is **reserved but not write-protected**. Generic update paths can
  still write it, so consumers must never rely on the raw value alone. They
  apply the predicate below.
* A missing `custom_fields` object or a missing key means "not marked".

## Stability

This is contract version `scheduler-baseline-marker/v1`. Any change to the
key, the value shape or the predicate is a **breaking change**. It needs a
new contract version and a consumer notice before it ships.

## Read recipe

Run the reads in this order. The JSON paths below are the exact paths the
U0b transport tests assert (`internal/cli/claim_marker_read_surface_test.go`
and `internal/mcp/claim_marker_read_surface_test.go`).

1. **Active shipment.** Run

   ```text
   backlogit shipment list --status active --format json
   ```

   or call MCP `backlogit_list_shipments` with `{"status":"active"}`. The
   result is a JSON array.
   * Length 0: there is no active shipment, so no item is claim-activated.
   * Length greater than 1: indeterminate. Fail closed.
   * Length 1: the active shipment ID is `.[0].id`.
2. **Manifest.** The shipment's manifest is `.[0].custom_fields.items[]`.
   There is no top-level `items` field.
3. **Each item.** Run `backlogit get <id> --format json` or call MCP
   `backlogit_get_item`, and read `.status` and
   `.custom_fields.scheduler_baseline_claim`.

MCP results arrive as JSON text content. Parse that text before applying the
paths above.

The recipe reads are not atomic. If a re-read of the active shipment's ID or
manifest differs from the first read, treat the result as indeterminate.

## Normative consumer predicate (option A)

The predicate below is copied verbatim from the decision artifact. It is
normative.

An item is **claim-activated** if and only if all three hold:

1. its `status` is `active`;
2. `custom_fields.scheduler_baseline_claim` equals the ID of the **currently
   active shipment** (backlogit allows at most one active shipment at a time;
   see `ensureShipmentActiveSlotAvailable`);
3. the item's ID is in that shipment's current manifest (`items`).

Every other active item is **organic-active**. When there is no active
shipment, no item is claim-activated. The marker is advisory: the backlogit
claim gate stays authoritative.

**Active-shipment cardinality guard** (added per plan-review attempt 6 P2-6):
if a consumer's active-shipment read returns more than one active shipment,
that is an invariant violation and the claim-activation state is
**indeterminate**; the R3 consumer rule applies (fail closed, defer to the
claim gate). The key is reserved but not write-protected, so consumers rely on
the predicate, never on the raw value alone.

**Withdrawn rule.** The earlier rule "non-empty marker ⇒ claim-activated" is
explicitly withdrawn. A non-empty marker on its own proves nothing: stale
markers persist (R1), and the key is not write-protected.

## Lifecycle (option A)

| Event | Marker behavior | Verification status |
|---|---|---|
| Claim of a member whose preimage status is `queued` | Written with the claiming shipment ID | Verified by U0a (`internal/core/shipment_claim_marker_test.go`) |
| Re-claim, including over a stale foreign value | Overwritten with the new shipment ID | Verified by U0a |
| Claim rollback, in-process snapshot restore | Removed: the exact preimage is restored | Verified by U3 (`internal/core/shipment_claim_marker_rollback_test.go`) |
| Claim rollback, journal recovery | Removed: the exact preimage is restored | Verified by U3 and U0c (`internal/core/shipment_claim_marker_recovery_test.go`) |
| Marker-only write on an already-active member-parent | Written without a status change | Verified by U0a and U1 (`internal/core/shipment_claim_marker_u1_behavior_test.go`), but unaudited: no event is appended (`setArtifactStatusWithClaimMarker`, `internal/core/shipment_lifecycle.go`) |
| Kept through block, unblock, return, ship, abandon, normalize, and generic update/move | Kept unchanged (a generic update that explicitly writes `custom_fields` can still change it, because the key is not write-protected) | Kept by construction: these paths clone and persist `custom_fields` unchanged (`BlockShipment`, `UnblockShipment` and `ReturnBlockedItem` in `internal/core/shipment.go`; `ShipShipment` in `internal/core/shipment_lifecycle.go`; `NormalizeBlockedShipment` in `internal/core/shipment_recovery.go`; `UpdateArtifact` in `internal/core/artifacts.go`). The regression net is task `182.001-T` under feature `182-F`; read that task's status for the current verification state. |

Only `ClaimShipment` writes the marker. It never marks or unmarks a member
whose preimage status is not `queued`. Known gap (deferred stash `E52607F5`):
the pre-existing bounded parent cascade can still move such a member-parent's
status, for example to `active` when a queued child member activates first.
That member-parent stays unmarked, so the predicate does not classify it as
claim-activated, and a claim crash mid-way through that window is not
recoverable by U1b. Treat that state as indeterminate and defer to the claim
gate. Claim crash recovery (U1b) accepts a member that carries the recovering
journal's own shipment ID as a valid mid-claim state, and fails closed on any
other divergence.

## Accepted residuals

The residual text below is copied verbatim from the decision artifact.

* **R1: stale markers persist.** Returned, blocked, shipped, abandoned and
  archived items keep their last marker. The predicate neutralizes this. Raw readers
  that ignore the predicate will be wrong.
* **R2: any activation route for an item that carries the active shipment's
  marker and is in its manifest.** Such an item reads as claim-activated,
  however it became active. This R2 wording replaces the original, per
  attempt 6 P2-3: organic activation *while blocked* is guarded and cannot
  happen. Examples:
  * a member marked by an earlier claim of the same shipment, which was then
    unblocked to `queued` and activated organically before the re-claim;
  * an item returned with `ReturnBlockedItem`, then activated organically,
    then added back to the active shipment with `AddItemToShipment`.

  R2 is narrow, and it errs toward hiding a residual rather than halting.
  The operator can restore such an item to `queued` before claiming.
* **R3: CLI/MCP divergence after a partial compensation.** A double-fault
  claim compensation restores files and index rows independently, so for the
  IDs it reports as unrestored, `backlogit get --format json` (frontmatter)
  and MCP `backlogit_get_item` (index) can disagree until journal recovery
  completes. `backlogit shipment list` is index-backed on both transports, so
  it can also be stale. This R3 wording replaces the original "run any
  lifecycle operation" remedy, per attempt 6 P2-7, and is split in two:
  * **Transport side effects (added per plan-review attempt 7 P2-D).** Every
    CLI read (`backlogit get`, `backlogit shipment list`) opens a recovering
    workspace: `core.NewWorkspace` runs `recoverPendingShipmentOperations`
    before the read. A CLI read can therefore roll back a pending claim
    journal as a side effect; this is sanctioned, fail-closed convergence, not
    a consumer-invoked lifecycle command. A fresh CLI read almost never shows
    the frontmatter/index split, which is observable only in-process or
    through a long-lived MCP server. Starting an MCP server (`backlogit mcp`,
    `openMCPServer` → `core.NewWorkspace`) also opens a recovering workspace,
    so server **startup** can roll back a pending journal; only reads on an
    already-running server skip recovery. A consumer that must avoid recovery
    side effects reads through an MCP server that is already running and that
    it did not start.
  * **Consumer rule (external agents, including the autoharness scheduler).**
    Treat the claim-activation state as **indeterminate** when any of these
    holds:
    * the frontmatter read and the index read disagree;
    * a pending claim journal exists under `<storage-root>/ops/` (the storage
      root may be `.backlogit`, `.backlog` or an override; `backlogit doctor`
      is the portable check);
    * `backlogit doctor` reports a shipment-lifecycle journal finding or
      conflict;
    * the active-shipment read returns more than one active shipment;
    * a recipe read exits non-zero or errors (in particular with an `open
      workspace: recover shipment operations` error), or an MCP tool call
      returns an error;
    * a re-read of the active shipment's ID or manifest differs from the
      first read (the recipe reads are not atomic).

    While indeterminate, the consumer does not classify, route or act on the
    marker. It defers to the backlogit claim gate, which is authoritative. A
    consumer never invokes a lifecycle command to clear the condition.
  * **Operator remediation (operator only, one specific trigger).**
    1. Run the CLI `backlogit sync` in a fresh process. Its workspace open
       (`core.NewWorkspace`) runs `recoverPendingShipmentOperations` before it
       rehydrates the shared index. An already-running MCP server's
       `backlogit_sync_index` only rehydrates; it does not run journal
       recovery.
    2. If an MCP server is running, call `backlogit_sync_index` as well, so
       both transports read the recovered state.
    3. If step 1 fails with a recovery error, stop. Inspect with
       `backlogit doctor` (a diagnostic workspace that does not run recovery)
       and escalate. Do not substitute another lifecycle operation.

## Mixed-binary caveat

The mixed-binary caveat (attempt 6 P2-9) applies to **any** binary that
opens the workspace, CLI or MCP server, including one a consumer invokes
or starts for a read: it must include U1b, because an older binary can re-create the F1 wedge.

The plan's rollout checkpoint (upgrade every binary before the first claim)
governs this.

## Adoption and scope

* **Not operative before rollout.** The autoharness consumer should not treat
  this contract as operative before the rollout checkpoint is recorded. See
  `docs/compound/workflow-issues/stable-contract-before-two-agent-adoption-2026-04-05.md`.
* **SQL is secondary.** The SQL `query` surface can read the marker with
  `json_extract` (for example
  `json_extract(custom_fields, '$.scheduler_baseline_claim')`), but the recipe
  above is the contract.
  SQL reads the index only, so it is subject to the same R3 caveats.
* **Advisory marker.** The marker is advisory; the backlogit claim gate is
  authoritative.
* **Full defect resolution needs the autoharness follow-up.** This contract
  publishes the producer side only. The autoharness scheduler change that
  consumes it is a separate follow-up.
