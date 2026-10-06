---
chunk_strategy: h1-h2-h3
schema_version: "1.0"
title: "backlogit_ship_shipment MCP timeout means result unknown; in 195-S and 196-S the governed close kept running"
description: "An MCP -32001 Request timed out on backlogit_ship_shipment makes the outcome unknown: the call may still be running, may have completed, or may have been cancelled or failed. Never retry or restore. Observe through lock quiescence, then classify the resulting state from item-log, hook, and doctor evidence."
doc_type: learning
source: docs/compound/2026-10-06-ship-shipment-mcp-timeout-result-unknown.md
docline:
    date: 2026-10-06T00:00:00Z
    severity: high
    tags:
        - shipment
        - post-merge-closure
        - mcp
        - timeout
        - p-007
        - ship-agent
---

# backlogit_ship_shipment MCP timeout means result unknown

## Problem

Two consecutive post-merge closures each timed out on the governed close:

* `195-S` (merge `58f5bdba`)
* `196-S` (merge `651b066b`)

The MCP client returned `MCP error -32001: Request timed out`. In both cases
the server was still executing the call and later ran it to completion.

Those two observations are not a contract. `handleShipShipment`
(`internal/mcp/tools.go`) passes the MCP request context into
`core.ShipShipment`. A future timeout could therefore coincide with
cancellation or a failure rather than continued execution.

`ShipShipment` runs these steps in order:

1. The completion gate, twice.
2. The status transition to `shipped`.
3. Commit traceability on every archive candidate.
4. Deepest-first `archiveItems`.
5. Post-ship consistency.
6. The post-ship hook.

On this workspace, each artifact write takes roughly 20–40 seconds because of
index and lock work. A ten-member shipment therefore needs about ten minutes,
well past the client's request timeout.

While the call is in flight, the working tree looks torn:

* The shipment is `status: shipped` but still in the queue.
* Some members are archived and others are not.
* `doctor --check-shipped-event-completeness` reports
  `shipped_unarchived_residue` for the shipment. Its own text says this "can be
  transient during an in-flight ship".

## Wrong responses

Each of these turns a transient in-flight state into real damage:

* Retrying `backlogit_ship_shipment`. This races a second governed call against
  the first.
* Running `git restore .backlogit/archive/`, or any P-007 restore. Nothing was
  deleted, and the restore would discard the in-flight writes.
* Treating the state as the 143-F halted-archival branch. That branch requires a
  returned `mutation_partial` / `indeterminate` / `shipped-event-append`
  envelope, and no envelope was returned.
* Hand-archiving the remaining members or the shipment record.

## Rule

Treat a ship timeout as "result unknown". The call may still be running, may
have completed, or may have been cancelled or failed. Do not assume any of
these outcomes; observe and classify.

1. Do not retry or mutate anything. Keep the shipment-reconcile lock held.
2. Observe until quiescence. Recently touched `.backlogit/.locks/` files and
   archive mtimes that are still advancing mean the call is still running.
   Quiescence means those signals have stopped advancing.
3. After quiescence, classify the state. Completion requires, first, the
   shipment record at `.backlogit/archive/{shipment_id}.md` with
   `archived_status: shipped`, and an `archived` event in the shipment item
   log. Steps 4–6 complete the check.
4. Confirm the post-ship hook event (`event_type: ship_shipment`) in
   `.backlogit/hooks_queue.jsonl`. The `emit_hook_event` post-hook writes it
   at priority 50, before the webhook (priority 80) and index-stale
   (priority 90) post-hooks (`internal/hooks/builtin_post.go`,
   `internal/hooks/webhook.go`). It is not the last step. Its presence proves
   only that core archival and post-ship consistency finished and post-hook
   processing began. It does not prove the call returned. Completion proof is
   still quiescence (step 2) together with the state checks in steps 3, 5,
   and 6.
5. Reconstruct the missing envelope from state:
   * `shipment_status: shipped`
   * `returned_ids: []` (no `returned_to_backlog` events)
   * `archived_ids` equal to the manifest plus the shipment record
6. Re-run `doctor --check-shipped-event-completeness` and confirm no finding for
   the shipment.
7. Only then run safe-close and post-mode checks: the non-member change set,
   parentage, and P-007. Record the timeout and the reconstruction in the
   safe-close report.

Any other quiescent state is not completion. Examples are lock files gone
quiet with an incomplete archive and no hook event, or a shipment that never
reached `shipped`. Treat it as a cancelled or failed call with a possible
partial mutation: halt, follow P-007 and the doctor audit, and hand off for
operator reconciliation. Do not retry.

## Side effects that are expected, not non-member mutations

* `.backlogit/hooks_queue.jsonl` gains the post-ship hook event.
* `.backlogit/stash.jsonl` can be rewritten with line-ending-only changes. Git
  shows it as modified, but it has no content diff, and the file drops out of
  `git status` after normalization.

Neither file is a queue or archive artifact, so neither enters the closure's
allowed-set comparison.

## Evidence

* `.backlogit/reconcile/196-S-safe-close-20261006T225014Z.md`: timeout,
  in-flight observation, and envelope reconstruction.
* `docs/closure/195-S-claim-start-proof-post-merge-closure.md`, Shipment
  reconciliation: the same timeout during the 195-S close.
* `internal/core/shipment_lifecycle.go`: `ShipShipment` step order and the
  deepest-first `archiveItems`.
