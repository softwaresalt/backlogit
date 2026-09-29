---
chunk_strategy: h1-h2-h3
description: "Decision for C29EBEE5 / plan-review attempt 5 finding F2: the scheduler_baseline_claim marker is kept through block, unblock and ReturnBlockedItem (option A), and consumers classify an item as claim-activated only when it is active, its marker equals the currently active shipment's ID, and it is in that shipment's manifest"
doc_type: decision
schema_version: "1.0"
source: docs/decisions/2026-09-28-173f-marker-lifecycle-option-a-decision.md
title: "Decision: 173-F scheduler-baseline marker lifecycle (option A)"
docline:
    stash_id: C29EBEE5
    status: decided
    created_at: 2026-09-29T05:30:00Z
---

## Decision: 173-F marker lifecycle — option A

**Depth**: focused (one operator decision already made).
**Route**: `deliberate`. `C29EBEE5` carries the `DEFERRED SCOPE EXPANSION`
marker, so P-021 C6 requires a deliberation artifact before re-planning.

**Subject**: plan-review attempt 5, finding F2, on
`docs/exec-plans/2026-09-13-shipment-claim-scheduler-reconciliation-plan.md`.
F2 found that the lifecycle of the `scheduler_baseline_claim` marker across
block, unblock and `ReturnBlockedItem` was undefined, and that the consumer
rule "non-empty marker ⇒ claim-activated" was unsafe.

## Problem frame

After `174-F`, `BlockShipment`, `UnblockShipment` (`internal/core/shipment.go`)
and `ReturnBlockedItem` write members with `cloneArtifact` + `persistArtifact`.
Both preserve `custom_fields`, so once a claim marks a member, the marker
survives these operations:

| Operation | Member state afterwards |
|---|---|
| Block | queued, still marked |
| Unblock to `active` | active (restored from the snapshot), still marked |
| Unblock to `queued` | queued, still marked |
| `ReturnBlockedItem` | `blocked` (with `blocked_reason`) and out of the manifest, still marked (stale). Corrected per plan-review attempt 6 P2-3; this row originally said "queued". |
| Ship / archive | terminal, still marked |

Under the old rule, a stale-marked item that later became active by an
organic route would be misclassified as claim-activated. That is the inverse
of the P-002.6 defect.

## Options

* **(A) Keep the marker, harden the consumer rule.** Block, unblock and return
  do not touch the marker, so there are no new write paths and no new
  recovery-candidate shapes for block, unblock, normalize or return. Consumers
  apply the three-part predicate below. Only claim writes the marker, and a
  re-claim overwrites any stale value.
* **(B) Clear on block and return, re-mark on unblock to active.** The raw
  marker stays exact, but this adds marker writes to Block, Unblock and
  ReturnBlocked and to each of their recovery candidates in
  `shipment_recovery.go`, plus RED coverage for each. That roughly doubles the
  size of the `173-F` blast radius.

## Chosen direction: (A)

The operator decided this on 2026-09-28T22:22 -07:00: "Decision: option A".
The attempt-5 Scope Boundary Auditor and Agent-Native Parity Reviewer had both
recommended it.

### Normative consumer predicate (published by U5)

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

### Producer rules (U1, U1b)

* Only `ClaimShipment` writes the marker. It writes it on every manifest
  member whose preimage status was `queued`, independent of manifest order.
  A member-parent that the bounded parent cascade has already activated is
  still marked.
* A re-claim overwrites a stale value with the new shipment ID.
* Members whose preimage status is not `queued` are left byte-identical: not
  marked and not unmarked.
* Block, Unblock, ReturnBlocked, Ship and normalize never write or clear the
  marker.
* Rollback, either in-process snapshot restore or journal recovery, restores
  the exact preimage. That removes a marker written by the rolled-back claim.
* Claim crash recovery (U1b) must accept a member that already carries **this
  journal's** shipment ID as a valid mid-claim state, and must keep failing
  closed on any other divergence, including a marker for a different
  shipment.

### Accepted residuals (documented by U5; not fixed in 173-F)

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

  The mixed-binary caveat (attempt 6 P2-9) applies to **any** binary that
  opens the workspace, CLI or MCP server, including one a consumer invokes
  or starts for a read: it must include U1b, because an older binary can re-create the F1 wedge. The plan's
  rollout checkpoint (upgrade every binary before the first claim) governs
  this. Attempt 7 re-reviews this text together with the rest of the
  attempt-6 remediation.

## Consequences for the plan

This decision resolves F2 and determines F6's invariant, F7's scenario (4)
and U5's rule. The revised plan and the attempt-6 review are in
`docs/exec-plans/2026-09-13-shipment-claim-scheduler-reconciliation-plan.md`
(section "Marker lifecycle decision (option A)").

Attempt 6 was a FAIL on test-first ordering (one P1, U0b before U1) and on
P2 text and feasibility corrections. It did **not** reopen this decision;
option A stands.

## Traceability

* Stash: `C29EBEE5`. The attempt-5 duplicate scan was clean, and late-ID
  reconciliation found nothing (N/A stands). Both results were re-checked in
  this session and have not changed.
* Feature `173-F`, shipment `154-S`, source stash `CC0EBB59`.
* Prior deliberations:
  `docs/decisions/2026-09-13-shipment-claim-scheduler-reconciliation-deliberation.md`
  and
  `docs/decisions/2026-09-20-shipment-claim-wave-scheduler-convergence-deliberation.md`.
