---
chunk_strategy: h1-h2-h3
closure_status: READY
compaction_status: done
description: "Post-merge closure for shipment 207-S and covering feature 206-F: replan queued shipment manifests without hand edits."
doc_type: closure
schema_version: "1.0"
shipment_id: 207-S
feature_id: 206-F
source: docs/closure/207-S-206-F-post-merge-closure.md
title: "207-S / 206-F Post-Merge Closure"
---

# 207-S / 206-F Post-Merge Closure

## Release record

| Field | Value |
|---|---|
| Shipment | `207-S` |
| Covering feature | `206-F` |
| Tasks | `206.001-T` through `206.003-T` (3 tasks, 3 waves, all test-first) |
| Implementation PR | [#628](https://github.com/softwaresalt/backlogit/pull/628) (merge commit) |
| Merge commit | `38842586bc5978457d146447542d78e482e5f178` |
| Reviewed implementation HEAD | `d1d0a7cc30f007be7ab038e7b51f85f27e035591` |
| Closure branch | `post-merge/206-f-replan-queued-shipment-manifests` |
| Closure status | `READY` |
| Context compaction | `done` (P-020 `compact-context` invoked) |

## Delivered

* Core `RemoveFromShipment` and `MoveShipmentItem` with force and reason guards, unconditional blocked-shipment refusal, read-only dry-run, governed requeue of unstarted claim-assigned members, and durable `remove_item` and `move_item` recovery journals
* Core `ReorderShipment` with explicit `--items` validation and idempotent `--auto` two-phase topological ordering
* CLI `backlogit shipment remove|move|reorder`, MCP tools `backlogit_remove_from_shipment`, `backlogit_move_shipment_item` and `backlogit_reorder_shipment`, registry entries and regenerated CLI reference
* Dry-run opens the workspace read-only on both surfaces, with the lazy MCP server capturing the storage root under `workspaceMu`

## Gates

| Gate | Result |
|---|---|
| Local review (P-014) | `READY` (0 findings) at `3971d675`, and again at `5fdb526b` and `d1d0a7cc` for the two Copilot fixes |
| Copilot review (P-018) | `SATISFIED` for the merged HEAD; three threads resolved (two fixed, one deferred as `6B017269`) |
| CI | All required checks passed, including the full `test` job |
| Merge strategy (P-009) | Merge commit |
| Governed closure | `backlogit_ship_shipment` returned a client timeout; completion was confirmed by waiting for quiescence and reading the `shipment_status_changed` (`shipped`), `commit_tracked` and `archived` events for `207-S`, plus archived `206-F`. Nothing was retried or synthesized. The three tasks were already archived at completion time |

## Runtime verification and releasability

Releasability: `READY`. The change adds CLI and MCP surfaces over core manifest operations, covered by unit, parity and recovery tests that ran in CI.

* Monitoring: shipment manifest and parity tests run in the standard CI suite
* Rollback: revert the merge commit
* Owner: backlogit maintainers
* Validation window: none required

## Compaction (P-020)

`compact-context` ran with `target: all`. It moved 53 uncited memory files from completed shipments to `docs/archive/memory/` and wrote three compacted summaries in `docs/memory/compacted/`. No plans or closure records were compacted.

## Environment notes

* The full local `internal/core` suite takes about 30 minutes on Windows, so CI ran it and local gates were targeted
* Local `golangci-lint` fails to load export data (toolchain mismatch); CI is unaffected

## Follow-ups

* Stash entry `6B017269`: divergence detection in shipment manifest recovery (Copilot thread PRRT_kwDORzozKM6yDXuc, deferred under P-021 C1; requires deliberation)
* Provenance only: source stash and deliberation references were left untouched for Stage
