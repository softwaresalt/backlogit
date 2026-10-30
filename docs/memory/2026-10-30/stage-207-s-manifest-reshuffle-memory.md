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
  * `206.001-T` (U1, `code`, `size: M`, `complexity: medium`): Implement core `RemoveFromShipment` and `MoveShipmentItem` with force/reason and dry-run guards in `internal/core/shipment_ops.go` and `internal/core/shipment_recovery.go` (including unconditional refusal of `blocked` shipments even with `Force: true` in both live and `DryRun` modes, status-preserving removal that clears matching `scheduler_baseline_claim == shipmentID` with full member preimage rollback, read-only zero-write `--dry-run` preview refusing pending recovery journals and avoiding SQLite cache-miss writes, and durable correlated `move_item` recovery journal validated by `validateShipmentLifecycleJournalRecord` in `shipment_ops.go` and recovered through the real `lockShipmentLifecycleGlobal` lifecycle barrier — restoring both shipment manifests AND the moved member artifact's full preimage including `scheduler_baseline_claim` across crash, `ErrWriteIndeterminate`, and failure-after-first-audit-append rollback/aborted events).
  * `206.002-T` (U2, `code`, `size: M`, `complexity: medium`, blocked by `206.001-T`): Implement core `ReorderShipment` in `internal/core/shipment_manifest.go` with explicit `--items` and `--auto` topological sort (emitting only root covering feature(s) `isRootCoveringFeature` in the leading prefix and including every remaining manifest member not already emitted as a root covering feature — nested/sub-epic dotted features `100.001-F`, tasks, and subtasks — as Kahn graph vertices honoring all intra-manifest `blocks` edges and default `P -> C` parent-before-child edges, e.g. nested-feature `[100-F, T, 100.001-F]` -> `[100-F, 100.001-F, T]` and flat `[T, S]` where `T` depends on `S` with absent parent -> `[S, T]`, unconditional refusal of `blocked` shipments even with `Force: true`, and read-only zero-write `--dry-run` preview).
  * `206.003-T` (U3, `code`, `size: M`, `complexity: medium`, blocked by `206.002-T`): Expose CLI subcommands and MCP tools for shipment `remove`, `move`, and `reorder`, update the authoritative `.autoharness/backlog-registry.yaml` (drift-ignored in `.autoharness/drift-ignore`) directly with `remove_from_shipment`, `move_shipment_item`, and `reorder_shipment`, wire parity tests in `internal/cli/registry_parity_test.go`, and regenerate `docs/cli-reference/`.
* **Assembled Shipment**: `207-S` (*Replan queued shipment manifests without hand edits (206-F)*, priority `high`, manifest `["206-F", "206.001-T", "206.002-T", "206.003-T"]`, sequenced after `206-S` via `blocks` dependency `207-S -> 206-S`).
* **Phase 2b Follow-Up Stash**: `40CC5C6B` (`kind: feature`, `priority: medium`) capturing declarative `shipment reconstitute --plan`, `adopt` manifest/stash rename propagation, `doctor` manifest integrity checks, and `stash correct --reason` / `stash repoint`.
