---
chunk_strategy: h1-h2-h3
description: 'Session memory for Stage Phase 2 (206-F / 207-S, stash DE3E7B67): staged primitive shipment manifest replanning operations (remove, move, reorder) into 206-F and 207-S, wired shipment sequencing edge 206-S -> 207-S, and captured Phase 2b follow-up stash 40CC5C6B.'
doc_type: memory
docline:
  date: 2026-10-30T09:11:00Z
  feature_id: 206-F
  phase: complete
  session_id: stage-2026-10-30-phase2-207s
  shipment_id: 207-S
  stash_id: DE3E7B67
  status: resolved
schema_version: "1.0"
source: docs/memory/2026-10-30/stage-207-s-manifest-reshuffle-memory.md
title: 'Stage Session Memory: Queued-shipment manifest replanning operations (206-F / 207-S)'
---

## Session Context

* **Agent**: `_Stage`
* **Branch**: `chore/stage-207-S` (branched from `main` at `14b3f7862a08b4dd75f8d3726bf9a71a4530bf20`)
* **Dark Factory Activation**: `DARK_MODE_ACTIVE = true` (`2026-10-30T05:55:05Z`), Phase 2 of 3 (`DE3E7B67` -> `206-F` / `207-S`; `last_completed = 206-S`).
* **Model Route**: `gemini-4-argon` / `google` / `high` / `long_context`.
* **Hook Events**: Polled and acknowledged up to `seq: 4279` (`206-S` shipped).
* **Checkpoints**: 0 anomalies, 0 active `stage` checkpoints on startup.

## Artifacts Produced

* **Deliberation**: `docs/decisions/2026-10-30-shipment-manifest-reshuffle-deliberation.md` (`decision_status: decided`, Option A selected).
* **Implementation Plan**: `docs/exec-plans/2026-10-30-shipment-manifest-reshuffle-plan.md` (`Requires plan hardening: yes`, `Constitution Check: pass`, `Plan Review: PASS`).
* **Harvested Feature**: `206-F` (*Replan queued shipment manifests without hand edits (remove, move, reorder)*, priority `high`, harvested from `DE3E7B67` which is now archived in `.backlogit/archive/stash.jsonl`).
* **Harvested Tasks**:
  * `206.001-T` (U1, `code`, `size: M`, `complexity: medium`): Implement core `RemoveFromShipment` and `MoveShipmentItem` with force/reason and dry-run guards in `internal/core/shipment_ops.go` and `internal/core/shipment_recovery.go` (including unconditional refusal of `blocked` shipments even with `Force: true` in both live and `DryRun` modes, governed requeue `active -> queued` + clearing `scheduler_baseline_claim` when force-removing or force-moving an eligible unstarted claim-assigned `active` member to a `queued` shipment so `ClaimShipment` on the target shipment activates and stamps `scheduler_baseline_claim` cleanly while refusing already-started active members, read-only zero-write `--dry-run` preview refusing pending recovery journals and avoiding SQLite cache-miss writes, and durable `remove_item` and `move_item` recovery journals validated by `validateShipmentLifecycleJournalRecord` / `loadShipmentOperationJournals` in `shipment_ops.go`, durably updated to `Phase: "committed"` after all writes and `phase: "committed"` audit event appends succeed and before `cleanupShipmentOperationJournal`, and recovered/finalized through the real `lockShipmentLifecycleGlobal` lifecycle barrier via `reconcileShipmentLifecycleIntent` in `shipment_recovery.go` — restoring `Preimage.Shipment`, `Preimage.Related` for `move_item`, and `Preimage.Members` including `status` and `scheduler_baseline_claim` in Markdown and SQLite across crash-between-writes, left-behind `Phase: "committed"` journal finalization, `ErrWriteIndeterminate` journal retention, `ErrWriteNotApplied` in-memory preimage rollback + journal removal, and failure-after-first-audit-append rollback/aborted events in `TestU1_RemoveFromShipment_QueuedAndForceGuards` and `TestU1_MoveShipmentItem_AtomicTransferAndGuards`).
  * `206.002-T` (U2, `code`, `size: M`, `complexity: medium`, blocked by `206.001-T`): Implement core `ReorderShipment` in `internal/core/shipment_manifest.go` with explicit `--items` and `--auto` topological sort over the two-phase dependency graph `G` (emitting only root covering feature(s) `isRootCoveringFeature` in the leading prefix, including every remaining manifest member not already emitted as a root covering feature — nested/sub-epic dotted features `100.001-F`, tasks, and subtasks — as vertices in `G`, first adding all hard intra-manifest `blocks` edges `B -> A` with cycle detection, and then iterating candidate default `P -> C` parent-before-child edges in deterministic manifest order and admitting `P -> C` into `G` if and only if `P` is not reachable from `C` in the current graph `G` containing all hard `blocks` edges plus previously admitted parent edges, e.g. nested-feature `[100-F, T, 100.001-F]` -> `[100-F, 100.001-F, T]`, flat `[T, S]` where `T` depends on `S` with absent parent -> `[S, T]`, and cross-parent `[F, A, B, a, b]` with `A depends_on b` and `B depends_on a` -> `[F, b, A, a, B]`, unconditional refusal of `blocked` shipments even with `Force: true`, and read-only zero-write `--dry-run` preview).
  * `206.003-T` (U3, `code`, `size: M`, `complexity: medium`, blocked by `206.002-T`): Expose CLI subcommands and MCP tools for shipment `remove`, `move`, and `reorder`, update the authoritative `.autoharness/backlog-registry.yaml` (drift-ignored in `.autoharness/drift-ignore`) directly with `remove_from_shipment`, `move_shipment_item`, and `reorder_shipment`, wire parity tests in `internal/cli/registry_parity_test.go`, and regenerate `docs/cli-reference/`.
* **Assembled Shipment**: `207-S` (*Replan queued shipment manifests without hand edits (206-F)*, priority `high`, manifest `["206-F", "206.001-T", "206.002-T", "206.003-T"]`, sequenced after `206-S` via `blocks` dependency `207-S -> 206-S`).
* **Phase 2b Follow-Up Stash**: `40CC5C6B` (`kind: feature`, `priority: medium`) capturing declarative `shipment reconstitute --plan`, `adopt` manifest/stash rename propagation, `doctor` manifest integrity checks, and `stash correct --reason` / `stash repoint`.
