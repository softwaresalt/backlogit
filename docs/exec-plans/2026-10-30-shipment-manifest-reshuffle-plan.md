---
chunk_strategy: h1-h2-h3
description: 'Implementation plan for feature 206-F (shipment 207-S, Phase 2, stash DE3E7B67): add core RemoveFromShipment, MoveShipmentItem, and ReorderShipment (--items and --auto topological sort) with --dry-run, --force+--reason non-queued guard, audited JSONL/SQLite events, and full CLI/MCP parity.'
doc_type: plan
docline:
  feature_id: 206-F
  shipment_id: 207-S
  source_decision: docs/decisions/2026-10-30-shipment-manifest-reshuffle-deliberation.md
  stash_id: DE3E7B67
schema_version: "1.0"
source: docs/exec-plans/2026-10-30-shipment-manifest-reshuffle-plan.md
title: 'Implementation Plan: Queued-shipment manifest replanning operations (206-F / 207-S)'
---

## Objective

Enable operators and agents to replan queued shipment manifests without hand-editing Markdown files by delivering the primitive shipment manifest operations from `DE3E7B67` (`docs/design-docs/2026-09-29-backlogit-shipment-manifest-reshuffle-feature-request.md` §1) under feature `206-F` and shipment `207-S`:

1. **`RemoveFromShipment` (`backlogit shipment remove` / `backlogit_remove_from_shipment`)**: Remove an item from a shipment's `custom_fields.items` manifest while leaving the item in `.backlogit/queue/` unassigned from the shipment manifest with its current `status` preserved (`queued` for queued shipments, `active` for claimed active shipments) and any matching `custom_fields.scheduler_baseline_claim == shipmentID` cleared inside the same compensated mutation, with a zero-write read-only `--dry-run` preview, a `--force` + `--reason` guard on non-`queued` shipments that unconditionally refuses `blocked` shipments (`ErrShipmentBlockedManifestSealed` / `ErrInvalidStatus` / `blerrors.ErrShipmentConflict`), and audited `shipment_item_removed` event logging.
2. **`MoveShipmentItem` (`backlogit shipment move` / `backlogit_move_shipment_item`)**: Atomically remove an item from a source shipment and add it to a target shipment under canonical lexicographical membership lock ordering and a durable correlated `move_item` recovery journal (`internal/core/shipment_recovery.go`), with a zero-write read-only `--dry-run` preview, a `--force` + `--reason` guard on non-`queued` endpoints that unconditionally refuses `blocked` endpoints, and correlated `shipment_item_moved` event logging with crash, `ErrWriteIndeterminate`, and partial-audit-write rollback/aborted-event recovery.
3. **`ReorderShipment` (`backlogit shipment reorder` / `backlogit_reorder_shipment`)**: Reorder a shipment's manifest either via an explicit `--items` permutation (validating exact member-set equality, root covering feature first, and subtasks after their parent task) or via `--auto` topological sorting (root covering feature first, all non-feature tasks and subtasks included as Kahn graph vertices ordered by all intra-manifest `blocks` dependencies with cycle detection, default parent-before-subtask edges unless overridden by `blocks` prerequisites, and deterministic tie-breaking), with a zero-write read-only `--dry-run` preview, a `--force` + `--reason` guard on non-`queued` shipments that unconditionally refuses `blocked` shipments, and audited `shipment_reordered` event logging.

Source decision: `docs/decisions/2026-10-30-shipment-manifest-reshuffle-deliberation.md` (Option A, stash `DE3E7B67`).

## Activation Contract & Routing Context

* `DARK_MODE_ACTIVE`: `true`
* `activation_utc`: `2026-10-30T05:55:05Z`
* `operator_request_verbatim`: `'Phase 1: Consistent CLI and MCP errors; Phase 2: Replan shipments without hand-editing manifests; Phase 3: Trustworthy metadata for cheaper model routing. Run pipeline in Dark Factory mode.'`
* `bounded_ordered_capability_scope`: `["Phase1 existing feature 166-F (completed in 206-S)", "Phase2 active stash DE3E7B67 (206-F / 207-S)", "Phase3 active stash A441A74C"]`
* `cursor`: `last_completed=206-S (Phase 1 — 166-F), next=Phase 2 (stash DE3E7B67 -> 206-F / 207-S)`
* `merge_approval_pre_authorized`: `true` (within this exact scope after current-HEAD local readiness + CI + review gates; merge commits only per P-009)
* `admin_fallback_pre_authorized`: `false`
* `routing_note`: `TOOL_OK: routing-resolver-cli` (`gemini-4-argon/google/high/long_context`). `TOOL_DEGRADED: engram` (daemon readiness timeout on `set_workspace`; using direct codebase `grep`/`view` fallback).

## Problem Frame

In backlogit today (`internal/core/shipment.go`, `internal/cli/shipment.go`, `internal/mcp/tools.go`), shipment manifest membership only supports `AddItemToShipment` (`backlogit shipment add` / `backlogit_add_to_shipment`) and active-shipment `ReturnBlockedItem` (`backlogit shipment return-blocked` / `backlogit_return_blocked`). Replanning a queued shipment after review — removing descoped tasks, moving tasks to another shipment, or restoring covering-feature-first and `blocks`-topological manifest ordering — forces manual edits to `.backlogit/queue/<id>-S.md`. Hand-editing bypasses core validation (`validateShipmentItemIDs`, one-shipment-per-item invariants, status guards), leaves no JSONL audit trail in `.backlogit/logs/`, and risks SQLite index drift.

## Requirements Trace

| ID | Requirement | Source | Implementation Unit |
|---|---|---|---|
| R1 | Implement `ShipmentManifestOptions`, `RemoveFromShipmentResult`, `MoveShipmentItemResult`, `RemoveFromShipment`, and `MoveShipmentItem` in `internal/core/shipment_manifest.go` (and `move_item` recovery in `internal/core/shipment_recovery.go`) with `queued`-only default (`blerrors.ErrShipmentConflict` when non-`queued` without `Force`, `blerrors.ErrValidation` when `Force` without non-empty `Reason`, and unconditional refusal of `blocked` shipments via `ErrShipmentBlockedManifestSealed` / `ErrInvalidStatus` / `blerrors.ErrShipmentConflict` even with `Force` in both live and `DryRun` modes), status-preserving removal that clears matching `scheduler_baseline_claim == shipmentID`, read-only zero-write `--dry-run` (refusing pending recovery journals and avoiding SQLite cache-miss writes), canonical lexicographical dual-shipment membership locking, durable correlated `move_item` recovery journal + `MutationEnvelope`, SQLite index sync (`persistArtifact` + `bldb.IndexEvent`), and `shipment_item_removed` / `shipment_item_moved` JSONL events | `DE3E7B67` & `2026-10-30-shipment-manifest-reshuffle-deliberation.md` (Option A, item 1) | U1 (`206.001-T`) |
| R2 | Implement `ReorderShipmentResult` and `ReorderShipment` in `internal/core/shipment_manifest.go` supporting mutually exclusive `--items` explicit permutation validation (exact member set equality, root covering feature first, subtasks not preceding their parent task) and `--auto` topological sorting (root covering feature first, all non-feature tasks and subtasks included as Kahn graph vertices ordered by all intra-manifest `blocks` dependencies with cycle detection, default parent-before-subtask edges unless overridden by `blocks` prerequisites, and deterministic tie-breaking), read-only zero-write `--dry-run`, `--force` + `--reason` non-`queued` guard that unconditionally refuses `blocked` shipments in both live and `DryRun` modes, and `shipment_reordered` JSONL + SQLite-indexed events | `DE3E7B67` & `2026-10-30-shipment-manifest-reshuffle-deliberation.md` (Option A, item 2) | U2 (`206.002-T`) |
| R3 | Expose CLI subcommands `backlogit shipment remove`, `backlogit shipment move`, and `backlogit shipment reorder` in `internal/cli/shipment.go` and MCP tools `backlogit_remove_from_shipment`, `backlogit_move_shipment_item`, and `backlogit_reorder_shipment` in `internal/mcp/tools.go` with isomorphic JSON output shapes, register `remove_from_shipment`, `move_shipment_item`, and `reorder_shipment` directly in `.autoharness/backlog-registry.yaml`, add CLI/MCP parity tests in `internal/cli/registry_parity_test.go`, and regenerate `docs/cli-reference/` via `go run ./cmd/gen-docs docs/cli-reference` | `DE3E7B67` & `2026-10-30-shipment-manifest-reshuffle-deliberation.md` (Option A, item 3) | U3 (`206.003-T`) |

## Constraints

* **2-Hour Rule and Width Isolation**: Each unit stays within the `code` domain, touches at most 2–3 core source/test files (plus `.autoharness/backlog-registry.yaml` and deterministic `go run ./cmd/gen-docs docs/cli-reference` output in U3), modifies at most 4 functions, and keeps `go test` completely green at the end of every wave.
* **Authoritative `.autoharness/backlog-registry.yaml` update**: `.autoharness/backlog-registry.yaml` is listed in `.autoharness/drift-ignore` (see commit `a86b8065`). U3 updates `.autoharness/backlog-registry.yaml` directly with `remove_from_shipment`, `move_shipment_item`, and `reorder_shipment` (no test-only `pendingInRepoRegistryOperations` workaround in `internal/cli/registry_parity_test.go`) so both agents and `TestRegistryParity_*` consume the authoritative operation registry.
* **Blocked-manifest seal preservation**: Because `persistArtifact` (`internal/core/shipment.go:2062-2069`) and `guardBlockedShipmentMembershipMutation` (`internal/core/artifacts.go:1042-1053`) unconditionally reject manifest changes while a stored shipment is `blocked`, any `blocked` shipment (`status == "blocked"` or carrying a blocked-manifest seal) is unconditionally refused (`ErrShipmentBlockedManifestSealed` / `ErrInvalidStatus` / `blerrors.ErrShipmentConflict`) even when `Force: true` — both in live execution and in `DryRun: true` preview — requiring governed `UnblockShipment` first. `Force: true` with non-empty `Reason` overrides only `active` (and terminal `shipped`/`abandoned`) non-`queued` shipments, never `blocked`.
* **Read-only zero-write preview (`DryRun: true`)**: When `opts.DryRun == true`, all three operations (`RemoveFromShipment`, `MoveShipmentItem`, `ReorderShipment`) MUST execute a dedicated read-only preview path that (a) does NOT call `lockShipmentLifecycleGlobal` (which creates lock files and runs recovery) — instead inspecting the shipment lifecycle recovery journal directory read-only and refusing with a pending-recovery error (`blerrors.ErrShipmentConflict`) if any unfinished recovery journal exists — and (b) does NOT call `GetShipment` or `loadArtifact` (which call `db.UpsertItem` on a SQLite cache miss) — instead reading the shipment and member artifacts directly from Markdown via a read-only file loader (`readArtifactFromDiskReadOnly`) that never writes to SQLite, filesystem locks, or JSONL logs.
* **Never-nil slice wire-shape invariant**: All `ItemsBefore`, `ItemsAfter`, `FromBefore`, `FromAfter`, `ToBefore`, and `ToAfter` slices on result structs and `custom_fields.items` MUST be non-nil (`[]string{}` when empty, never `nil`) so JSON marshaling on both CLI and MCP always emits `[]` rather than `null` (`docs/compound/2026-07-03-cli-mcp-honest-fallback-map-and-registry-drift-test.md` Rule 3).
* **Genuine tests-first RED -> GREEN**: Every unit specifies the exact failing test selector (`^TestU1_`, `^TestU2_`, `^TestU3_`) that compiles cleanly and is observed RED before the production implementation turns it GREEN. No `harness-exempt` tasks exist in this plan (`closed_exempt_set: []`).

## Implementation Units

### U1: Implement core `RemoveFromShipment` and `MoveShipmentItem` (`code`) — `206.001-T`

* **Domain**: `code`
* **Changes**:
  * **First (RED phase, scaffolded by `harness-architect`)**:
    1. Create `internal/core/shipment_manifest.go` with exported types `ShipmentManifestOptions`, `RemoveFromShipmentResult`, `MoveShipmentItemResult`, and stub functions `RemoveFromShipment(ctx context.Context, ws *Workspace, shipmentID, itemID string, opts ShipmentManifestOptions) (*RemoveFromShipmentResult, error)` and `MoveShipmentItem(ctx context.Context, ws *Workspace, itemID, fromShipmentID, toShipmentID string, opts ShipmentManifestOptions) (*MoveShipmentItemResult, error)` returning `nil, blerrors.ErrNotImplemented`.
    2. Create `internal/core/shipment_manifest_test.go` with unit tests:
       * `TestU1_RemoveFromShipment_QueuedAndDryRunAndForceGuard`:
         * Removing a task from a `queued` shipment with `--dry-run` (`DryRun: true`) returns `Status: "dry_run"`, `DryRun: true`, `ItemsBefore: [featID, task1ID, task2ID]`, `ItemsAfter: [featID, task2ID]`, leaves the stored shipment manifest unchanged in Markdown and SQLite, and appends no `shipment_item_removed` event.
         * Read-only preview guarantees on `RemoveFromShipment` (`DryRun: true`): (a) when an unfinished shipment lifecycle recovery journal exists on disk, `DryRun: true` refuses with a pending-recovery error (`errors.Is(err, blerrors.ErrShipmentConflict)`) without running recovery or modifying the journal; (b) on a SQLite cache miss (e.g., shipment row deleted from SQLite while present on disk in Markdown), `DryRun: true` reads from Markdown via `readArtifactFromDiskReadOnly` and performs zero SQLite writes, zero lock-file creations, and zero JSONL appends.
         * Live removal (`DryRun: false`) on a `queued` shipment returns `Status: "removed"`, updates the shipment's Markdown frontmatter and SQLite index to `[featID, task2ID]`, leaves `task1ID` in `.backlogit/queue/` unassigned from the shipment manifest with its current `status` preserved (`StatusQueued`), and records a `shipment_item_removed` event in `.backlogit/logs/<shipID>.jsonl` and SQLite `events`.
         * Removing an item not present in the shipment fails with `errors.Is(err, blerrors.ErrNotFound)`.
         * Removing an item from an `active` shipment fails with `errors.Is(err, blerrors.ErrShipmentConflict)` when `Force: false`, fails with `errors.Is(err, blerrors.ErrValidation)` when `Force: true` and `Reason: "   "`, and succeeds when `Force: true` and `Reason: "operator descope"` (recording `force: true` and `reason: "operator descope"` in the `shipment_item_removed` event delta).
         * Claimed-member forced removal: when `shipA` has been claimed via `ClaimShipment` (so `task1ID` has `status: active` and `custom_fields.scheduler_baseline_claim == shipA`), forced removal (`Force: true, Reason: "operator descope"`) leaves `task1ID` in `.backlogit/queue/` unassigned from `shipA`'s manifest with its current `status` preserved (`active`) and clears `custom_fields.scheduler_baseline_claim` on `task1ID` in both Markdown and SQLite inside the same compensated mutation.
         * Blocked-manifest seal preservation: removing an item from a `blocked` shipment (`status == "blocked"` or carrying a blocked-manifest seal) is unconditionally refused (`ErrShipmentBlockedManifestSealed` / `ErrInvalidStatus` / `errors.Is(err, blerrors.ErrShipmentConflict)`) even when `Force: true` and `Reason: "override"` — in both live (`DryRun: false`) and preview (`DryRun: true`) modes — requiring governed `UnblockShipment` first.
       * `TestU1_MoveShipmentItem_AtomicTransferAndGuards`:
         * Moving `task1ID` from queued `shipA` to queued `shipB` with `DryRun: true` returns `Status: "dry_run"`, `FromBefore`/`FromAfter`, `ToBefore`/`ToAfter`, and mutates neither shipment on disk nor in SQLite.
         * Read-only preview guarantees on `MoveShipmentItem` (`DryRun: true`): (a) refuses with a pending-recovery error (`errors.Is(err, blerrors.ErrShipmentConflict)`) if an unfinished recovery journal exists, without running recovery; (b) on a SQLite cache miss for `shipA`, `shipB`, or `task1ID`, reads directly from Markdown and performs zero SQLite writes, zero lock-file creations, and zero JSONL appends.
         * Live move (`DryRun: false, Reason: "rebalance scope"`) atomically removes `task1ID` from `shipA` and appends it to `shipB` in both Markdown and SQLite, and records correlated `shipment_item_moved` events on both `shipA` and `shipB` carrying `correlation_id`, `item_id`, `from_shipment_id`, `to_shipment_id`, and `reason`.
         * Moving when `fromShipmentID == toShipmentID` fails with `errors.Is(err, blerrors.ErrValidation)`; moving an item not in `fromShipmentID` fails with `errors.Is(err, blerrors.ErrNotFound)`; moving when either `shipA` or `shipB` is `active` fails with `errors.Is(err, blerrors.ErrShipmentConflict)` unless `Force: true` and non-empty `Reason` are supplied.
         * Blocked-manifest seal preservation: moving when either `shipA` or `shipB` is `blocked` is unconditionally refused (`ErrShipmentBlockedManifestSealed` / `ErrInvalidStatus` / `errors.Is(err, blerrors.ErrShipmentConflict)`) even when `Force: true` and `Reason: "override"` — in both live (`DryRun: false`) and preview (`DryRun: true`) modes.
         * Durable recovery protocol for partial move and audit writes: (a) simulated crash between writing `shipA` and `shipB` leaves the durable `move_item` recovery journal on disk, and the next `recoverPendingShipmentOperations` / `lockShipmentLifecycleGlobal` barrier restores both `shipA` and `shipB` to their exact preimage manifests; (b) when a manifest or audit write returns `ErrWriteIndeterminate`, `MoveShipmentItem` does NOT run in-memory compensation, retains the `move_item` recovery journal on disk, and returns `ErrWriteIndeterminate` so the next barrier recovers both manifests to their preimages; (c) when the second `shipment_item_moved` audit append fails with `ErrWriteNotApplied` after both manifests and the first shipment's committed audit event were written, `MoveShipmentItem` rolls back both manifests to their preimages, appends a compensating `phase: "aborted"` event carrying the same `correlation_id` to the first shipment's log, and removes the recovery journal.
    3. Observe RED: `go test -run=^$ -count=1 ./...` exits 0 and `go test -count=1 -run '^TestU1_' ./internal/core` fails with `ErrNotImplemented`.
  * **Second (GREEN phase, implemented by `build-feature`)**:
    1. In `internal/core/shipment_manifest.go`, implement `validateShipmentManifestQueueGuard(shipment *models.Artifact, opts ShipmentManifestOptions, action string) error`:
       * First, if `shipment.Status == models.StatusBlocked` or `hasBlockedShipmentEnvelope(shipment)` (blocked-manifest seal), unconditionally return `fmt.Errorf("shipment %s membership is sealed while blocked; run governed unblock before %s: %w", shipment.ID, action, blerrors.ErrShipmentConflict)` (preserving the blocked-manifest seal even when `opts.Force == true`, in both live and `DryRun` modes).
       * Next, when `shipment.Status != models.StatusQueued`: if `!opts.Force`, return `fmt.Errorf("%s on non-queued shipment %s (status=%s) requires --force and --reason: %w", action, shipment.ID, shipment.Status, blerrors.ErrShipmentConflict)`; if `strings.TrimSpace(opts.Reason) == ""`, return `fmt.Errorf("%s on non-queued shipment %s with --force requires non-empty --reason: %w", action, shipment.ID, blerrors.ErrValidation)`.
    2. Implement read-only preview helpers `ensureNoPendingShipmentRecoveryReadOnly(ws *Workspace) error` and `readArtifactFromDiskReadOnly(ws *Workspace, id string) (*models.Artifact, error)` that inspect the recovery journal directory and parse Markdown files directly from disk without creating lock files, running recovery, or calling `db.UpsertItem`.
    3. Implement `RemoveFromShipment`:
       * Validate non-empty `shipmentID` and `itemID` (`blerrors.ErrValidation`).
       * When `opts.DryRun == true`: call `ensureNoPendingShipmentRecoveryReadOnly(ws)` (refusing with `blerrors.ErrShipmentConflict` if any unfinished recovery journal exists), load the shipment and member artifact via `readArtifactFromDiskReadOnly`, enforce `validateShipmentManifestQueueGuard`, verify `containsString(itemsBefore, itemID)` (`blerrors.ErrNotFound`), and return `&RemoveFromShipmentResult{ShipmentID: shipmentID, ItemID: itemID, Status: "dry_run", DryRun: true, Force: opts.Force, Reason: strings.TrimSpace(opts.Reason), ItemsBefore: itemsBefore, ItemsAfter: itemsAfter}, nil` with zero disk, lock, or SQLite writes.
       * When `opts.DryRun == false`: acquire `lockShipmentLifecycleGlobal(ctx, ws)`, run `recoverPendingShipmentOperations(ctx, ws)`, acquire `lockShipmentMembership(ctx, ws, shipmentID)`, load shipment via `GetShipment(ctx, ws, shipmentID)`, and enforce `validateShipmentManifestQueueGuard`.
       * Compute `itemsBefore := NormalizeShipmentItems(shipment)`; if `!containsString(itemsBefore, itemID)`, return `fmt.Errorf("item %s is not in shipment %s: %w", itemID, shipmentID, blerrors.ErrNotFound)`.
       * Compute `itemsAfter := removeString(itemsBefore, itemID)` (guaranteed non-nil `[]string{}`).
       * Load the member artifact; preserve its current `Status` (`queued` for queued shipments, `active` for claimed active shipments), and if `member.CustomFields["scheduler_baseline_claim"] == shipmentID`, delete `"scheduler_baseline_claim"` from `member.CustomFields` and include the member frontmatter/SQLite update with preimage rollback compensation inside the same `MutationEnvelope` alongside `"frontmatter-update"` (`persistArtifact(ctx, ws, shipment, false)` + rollback compensation) and `"jsonl-append"` (`NewWorkspaceEventWriter` + `bldb.IndexEvent` for `EventType: "shipment_item_removed"` with delta `item_id`, `items_before`, `items_after`, optional `reason`, and optional `force`).
    4. Implement `MoveShipmentItem` (and `move_item` recovery journal handling in `internal/core/shipment_recovery.go`):
       * Validate non-empty `itemID`, `fromShipmentID`, `toShipmentID`, and `fromShipmentID != toShipmentID` (`blerrors.ErrValidation`).
       * When `opts.DryRun == true`: call `ensureNoPendingShipmentRecoveryReadOnly(ws)`, load `fromShipment`, `toShipment`, and `itemID` via `readArtifactFromDiskReadOnly`, enforce `validateShipmentManifestQueueGuard` on both shipments (unconditionally refusing `blocked` shipments even with `Force: true`), verify `containsString(fromBefore, itemID)` (`blerrors.ErrNotFound`) and no conflicting third-shipment membership via read-only scan, and return `&MoveShipmentItemResult{..., Status: "dry_run", DryRun: true}, nil` with zero writes.
       * When `opts.DryRun == false`: acquire `lockShipmentLifecycleGlobal(ctx, ws)`, run `recoverPendingShipmentOperations(ctx, ws)`, and acquire `lockShipmentMembership` on `min(fromShipmentID, toShipmentID)` then `max(fromShipmentID, toShipmentID)`.
       * Load `fromShipment` and `toShipment` via `GetShipment`, enforce `validateShipmentManifestQueueGuard` on both, verify `containsString(fromBefore, itemID)` (`blerrors.ErrNotFound`), and verify `itemID` is a valid non-shipment artifact not assigned to any third active/queued/blocked shipment.
       * Compute `fromAfter := removeString(fromBefore, itemID)` and `toAfter` (appending `itemID` if not already present; if `member.CustomFields["scheduler_baseline_claim"] == fromShipmentID` and `toShipment.Status == models.StatusQueued`, or when moving across shipments, update/preserve member claim metadata consistently inside the preimage-tracked mutation).
       * Execute the durable, correlated move protocol:
         (a) Before mutating either shipment manifest, persist a durable `move_item` recovery journal recording `correlation_id`, `source_shipment_id`, `target_shipment_id`, `item_id`, and exact preimage snapshots of both shipment manifests, and append `phase: "intent"` `shipment_lifecycle` events to both shipments' logs.
         (b) Persist both updated shipment manifests and sync SQLite (`persistArtifact`). On `ErrWriteIndeterminate`, do NOT run in-memory compensation — leave the recovery journal in place and return `ErrWriteIndeterminate` so the next `lockShipmentLifecycleGlobal` barrier recovers both manifests to their preimages.
         (c) Append `phase: "committed"` `shipment_item_moved` events carrying the shared `correlation_id` only after both manifest writes succeed; if the second audit append fails with `ErrWriteNotApplied`, roll back both manifests to their preimages AND append a compensating `phase: "aborted"` event carrying the same `correlation_id` to the first shipment's log (so the append-only log never leaves an uncompensated committed move event), then remove the recovery journal; if an audit append returns `ErrWriteIndeterminate`, keep the journal for barrier recovery.
* **Affected files**:
  * `internal/core/shipment_manifest.go` (new)
  * `internal/core/shipment_recovery.go`
  * `internal/core/shipment_manifest_test.go` (new)
* **Acceptance criteria**:
  * RED phase compiles cleanly (`go test -run=^$ -count=1 ./...`) and `go test -count=1 -run '^TestU1_' ./internal/core` fails before the GREEN implementation.
  * GREEN phase passes `go test -count=1 -run '^TestU1_' ./internal/core` and `go test -count=1 ./internal/core/...`.
* **Execution posture**: test-first.

### U2: Implement core `ReorderShipment` with explicit `--items` and `--auto` topological sort (`code`) — `206.002-T`

* **Domain**: `code`
* **Depends on**: U1 (`206.001-T`)
* **Changes**:
  * **First (RED phase, scaffolded by `harness-architect`)**:
    1. In `internal/core/shipment_manifest.go`, declare `ReorderShipmentResult` and stub `ReorderShipment(ctx context.Context, ws *Workspace, shipmentID string, opts ShipmentManifestOptions) (*ReorderShipmentResult, error)` returning `nil, blerrors.ErrNotImplemented`.
    2. In `internal/core/shipment_manifest_test.go`, add `TestU2_ReorderShipment_ExplicitAndAutoTopological`:
       * **Auto topological ordering (`opts.Auto: true`)**: Create a shipment whose initial manifest is scrambled (`[subtask2_1, task3, task1, feat, task2, subtask1_1]`) where `task3` has a `blocks` dependency on `task2` and `task2` has a `blocks` dependency on `task1`, `subtask1_1` has `ParentID: task1`, and `subtask2_1` has `ParentID: task2`. Verify:
         * `DryRun: true, Auto: true` returns `Status: "dry_run"`, `Mode: "auto"`, `ItemsAfter: [feat, task1, subtask1_1, task2, subtask2_1, task3]`, without modifying the stored shipment on disk or emitting an event.
         * Read-only preview guarantees on `ReorderShipment` (`DryRun: true`): (a) when an unfinished recovery journal exists on disk, `DryRun: true` refuses with a pending-recovery error (`errors.Is(err, blerrors.ErrShipmentConflict)`) without running recovery; (b) on a SQLite cache miss for the shipment or any manifest member, `DryRun: true` reads from Markdown via `readArtifactFromDiskReadOnly` and performs zero SQLite writes, zero lock-file creations, and zero JSONL appends.
         * `DryRun: false, Auto: true` persists `ItemsAfter: [feat, task1, subtask1_1, task2, subtask2_1, task3]` in Markdown and SQLite and appends a `shipment_reordered` event (`mode: "auto"`, `items_before`, `items_after`).
         * **Subtask with absent parent and `blocks` edge (`[T, S]` -> `[S, T]`)**: For a flat manifest `[T, S]` (or `[feat, T, S]`) where task `T` has a `blocks` dependency on subtask `S` and `S`'s parent task is absent from the manifest, `ReorderShipment` with `Auto: true` includes `S` as a vertex in the Kahn dependency graph, honors the `S -> T` prerequisite edge, and produces `[S, T]` (or `[feat, S, T]`).
       * **Explicit permutation ordering (`opts.Items`)**:
         * Supplying a valid permutation `[feat, task2, subtask2_1, task1, subtask1_1, task3]` succeeds (`Mode: "explicit"`, `Status: "reordered"`).
         * Supplying an `--items` list that omits a member, adds a foreign ID, duplicates an ID, places a task before the root covering feature `feat`, or places a subtask before its parent task fails with `errors.Is(err, blerrors.ErrValidation)`.
         * Supplying neither `--items` nor `--auto` (or both `--items` and `--auto`) fails with `errors.Is(err, blerrors.ErrValidation)`.
       * **Non-`queued` and `blocked` guards**: Reordering an `active` shipment fails with `errors.Is(err, blerrors.ErrShipmentConflict)` without `Force`, fails with `errors.Is(err, blerrors.ErrValidation)` when `Force: true` and `Reason == ""`, and succeeds when `Force: true` and `Reason: "fix manifest order"`. Reordering a `blocked` shipment fails unconditionally (`ErrShipmentBlockedManifestSealed` / `ErrInvalidStatus` / `errors.Is(err, blerrors.ErrShipmentConflict)`) even when `Force: true` and `Reason: "fix manifest order"` — in both live (`DryRun: false`) and preview (`DryRun: true`) modes.
    3. Observe RED: `go test -run=^$ -count=1 ./...` exits 0 and `go test -count=1 -run '^TestU2_' ./internal/core` fails with `ErrNotImplemented`.
  * **Second (GREEN phase, implemented by `build-feature`)**:
    1. In `internal/core/shipment_manifest.go`, implement `ReorderShipment`:
       * Validate `strings.TrimSpace(shipmentID) != ""` and `(len(opts.Items) > 0) != opts.Auto` (returning `blerrors.ErrValidation` if neither or both are set).
       * When `opts.DryRun == true`: call `ensureNoPendingShipmentRecoveryReadOnly(ws)` (refusing if any unfinished recovery journal exists) and load the shipment and all manifest member artifacts via `readArtifactFromDiskReadOnly` without acquiring filesystem locks, running recovery, or writing to SQLite.
       * When `opts.DryRun == false`: acquire `lockShipmentLifecycleGlobal(ctx, ws)`, run `recoverPendingShipmentOperations(ctx, ws)`, acquire `lockShipmentMembership(ctx, ws, shipmentID)`, and load the shipment via `GetShipment(ctx, ws, shipmentID)` and member artifacts.
       * Enforce `validateShipmentManifestQueueGuard` (which unconditionally refuses `blocked` shipments even when `opts.Force == true`, in both live and `DryRun` modes).
       * Load all manifest member artifacts (`itemsBefore := NormalizeShipmentItems(shipment)`).
       * For `mode == "explicit"`: verify `len(opts.Items) == len(itemsBefore)` with exact set bijection (no duplicates, no missing or extra IDs), verify any root covering feature (`isRootCoveringFeature`) precedes all non-root-feature items, and verify every subtask whose parent task is in the manifest appears after that parent task; otherwise return `fmt.Errorf("...: %w", blerrors.ErrValidation)`.
       * For `mode == "auto"`: compute deterministic topological order:
         * Place root covering feature(s) (`isRootCoveringFeature(art)`) first (preserving relative manifest order).
         * Include all non-feature manifest members (both `task` and `subtask` items) as vertices in the Kahn topological sort graph over intra-manifest `blocks` dependencies (`dep.Type == "" || dep.Type == "blocks"`):
           - Every intra-manifest `blocks` edge `A depends_on B` (whether `A` and `B` are tasks or subtasks) adds a directed prerequisite edge `B -> A` in the graph.
           - For any subtask `S` whose parent task `P` is also in the manifest, add a default ordering edge `P -> S` unless `P` transitively depends on `S` via `blocks` edges (so explicit `blocks` prerequisites are never overridden or turned into false cycles).
           - Priority-queue tie-breaking prefers ready subtasks of the most-recently emitted parent task when no `blocks` prerequisite is violated, otherwise preserves original manifest index order; fail closed with `blerrors.ErrValidation` if a cycle is detected.
       * On `opts.DryRun`, return `&ReorderShipmentResult{ShipmentID: shipmentID, Mode: mode, Status: "dry_run", DryRun: true, Force: opts.Force, Reason: strings.TrimSpace(opts.Reason), ItemsBefore: itemsBefore, ItemsAfter: itemsAfter}, nil`.
       * Otherwise, persist via `MutationEnvelope` (`"frontmatter-update"` + `"jsonl-append"` with `EventType: "shipment_reordered"`) and return `&ReorderShipmentResult{..., Status: "reordered"}, nil`.
* **Affected files**:
  * `internal/core/shipment_manifest.go`
  * `internal/core/shipment_manifest_test.go`
* **Acceptance criteria**:
  * RED phase compiles cleanly (`go test -run=^$ -count=1 ./...`) and `go test -count=1 -run '^TestU2_' ./internal/core` fails before the GREEN implementation.
  * GREEN phase passes `go test -count=1 -run '^TestU1_|^TestU2_' ./internal/core` and `go test -count=1 ./internal/core/...`.
* **Execution posture**: test-first.

### U3: Expose CLI subcommands and MCP tools for `remove`, `move`, and `reorder` + update `.autoharness/backlog-registry.yaml` and regenerate CLI reference docs (`code`) — `206.003-T`

* **Domain**: `code`
* **Depends on**: U2 (`206.002-T`)
* **Changes**:
  * **First (RED phase, scaffolded by `harness-architect` in `*_test.go`)**:
    1. In `internal/cli/registry_parity_test.go`, add `TestU3_ShipmentManifestCLIAndMCP_Parity`:
       * Set up a workspace and MCP server (`mcpinternal.NewServer(ws)`), create two features (`featA`, `featB`), two shipments (`shipA`, `shipB`), and tasks (`taskA1`, `taskA2` with `taskA2` blocked by `taskA1`).
       * Invoke `backlogit shipment reorder <shipA> --auto --dry-run` via CLI (`NewRootCommand()`) and `backlogit_reorder_shipment` via MCP (`s.InvokeTool`), and assert both succeed and return isomorphic JSON keys (`shipment_id`, `mode`, `status`, `dry_run`, `items_before`, `items_after`).
       * Invoke `backlogit shipment reorder <shipA> --auto` via CLI and `backlogit_reorder_shipment` (`items` explicit) via MCP, verifying manifest order updates.
       * Invoke `backlogit shipment move <taskA2> --from <shipA> --to <shipB> --reason "split scope"` via CLI and `backlogit_move_shipment_item` via MCP (moving back with `--dry-run` and live), asserting isomorphic JSON keys (`item_id`, `from_shipment_id`, `to_shipment_id`, `status`, `from_items_before`, `from_items_after`, `to_items_before`, `to_items_after`).
       * Invoke `backlogit shipment remove <shipA> <taskA2> --reason "descope"` via CLI and `backlogit_remove_from_shipment` via MCP, asserting isomorphic JSON keys (`shipment_id`, `item_id`, `status`, `items_before`, `items_after`) and verifying refused non-queued shipment behavior without `--force` + `--reason`.
    2. Observe RED: `go test -run=^$ -count=1 ./...` exits 0 and `go test -count=1 -run '^TestU3_ShipmentManifestCLIAndMCP_Parity$' ./internal/cli` fails because `shipment remove`, `shipment move`, `shipment reorder`, and the 3 MCP tools are not yet registered.
  * **Second (GREEN phase, implemented by `build-feature`)**:
    1. In `internal/cli/shipment.go`, register `newShipmentRemoveCmd()`, `newShipmentMoveCmd()`, and `newShipmentReorderCmd()` on `NewShipmentCmd()`:
       * `backlogit shipment remove <shipment-id> <item-id> [--reason <text>] [--force] [--dry-run]` (`cobra.ExactArgs(2)`).
       * `backlogit shipment move <item-id> --from <shipment-id> --to <shipment-id> [--reason <text>] [--force] [--dry-run]` (`cobra.ExactArgs(1)`, `--from` and `--to` marked required).
       * `backlogit shipment reorder <shipment-id> [--items <id,id,...>] [--auto] [--reason <text>] [--force] [--dry-run]` (`cobra.ExactArgs(1)`).
    2. In `internal/mcp/tools.go`, register:
       * `backlogit_remove_from_shipment` (`shipment_id` required, `item_id` required, `reason` string, `force` bool, `dry_run` bool) -> `s.handleRemoveFromShipment`.
       * `backlogit_move_shipment_item` (`item_id` required, `from_shipment_id` required, `to_shipment_id` required, `reason` string, `force` bool, `dry_run` bool) -> `s.handleMoveShipmentItem`.
       * `backlogit_reorder_shipment` (`shipment_id` required, `items` string, `auto` bool, `reason` string, `force` bool, `dry_run` bool) -> `s.handleReorderShipment`.
    3. In `.autoharness/backlog-registry.yaml` (which is drift-ignored in `.autoharness/drift-ignore`), add the authoritative operation entries for `remove_from_shipment` (`mcp_tool: "backlogit_remove_from_shipment"`, `cli_command: "backlogit shipment remove {{shipment_id}} {{item_id}} --reason {{reason}}"`), `move_shipment_item` (`mcp_tool: "backlogit_move_shipment_item"`, `cli_command: "backlogit shipment move {{item_id}} --from {{from_shipment_id}} --to {{to_shipment_id}} --reason {{reason}}"`), and `reorder_shipment` (`mcp_tool: "backlogit_reorder_shipment"`, `cli_command: "backlogit shipment reorder {{shipment_id}} --items {{items}}"`), with full `params` mappings so both agents and `TestRegistryParity_*` in `internal/cli/registry_parity_test.go` consume the authoritative registry directly without test-only workarounds.
    4. Run `go run ./cmd/gen-docs docs/cli-reference` to update `docs/cli-reference/backlogit_shipment.md` and generate `docs/cli-reference/backlogit_shipment_remove.md`, `docs/cli-reference/backlogit_shipment_move.md`, and `docs/cli-reference/backlogit_shipment_reorder.md` so CI's `CLI Reference Drift` check passes with zero diff.
* **Affected files**:
  * `.autoharness/backlog-registry.yaml`
  * `internal/cli/shipment.go`
  * `internal/mcp/tools.go`
  * `internal/cli/registry_parity_test.go`
  * `docs/cli-reference/backlogit_shipment.md`
  * `docs/cli-reference/backlogit_shipment_remove.md` (new, generated)
  * `docs/cli-reference/backlogit_shipment_move.md` (new, generated)
  * `docs/cli-reference/backlogit_shipment_reorder.md` (new, generated)
* **Acceptance criteria**:
  * RED phase is observed failing in `go test -count=1 -run '^TestU3_ShipmentManifestCLIAndMCP_Parity$' ./internal/cli` before `internal/cli/shipment.go` and `internal/mcp/tools.go` are wired.
  * GREEN phase passes `go test -count=1 -race ./internal/core/... ./internal/cli/... ./internal/mcp/...` (including all `TestRegistryParity_*` tests against `.autoharness/backlog-registry.yaml`).
  * `go run ./cmd/gen-docs docs/cli-reference` followed by `git diff --exit-code docs/cli-reference/` exits 0, and `backlogit docs lint --path docs/cli-reference` reports 0 violations.
* **Execution posture**: test-first.

## Dependency Graph

* **Shipment sequencing**: `206-S` (shipped) -> `207-S` (`207-S` carries an explicit `blocks` dependency on `206-S`).
* **Wave 1**: `206.001-T` (U1 — Core `RemoveFromShipment` and `MoveShipmentItem` + durable `move_item` recovery journal + unit tests `TestU1_*`)
* **Wave 2**: `206.002-T` (U2 — blocked by `206.001-T`; Core `ReorderShipment` with `--items` and `--auto` topological sort + unit tests `TestU2_*`)
* **Wave 3**: `206.003-T` (U3 — blocked by `206.002-T`; CLI `shipment remove|move|reorder`, MCP `backlogit_remove_from_shipment|backlogit_move_shipment_item|backlogit_reorder_shipment`, authoritative `.autoharness/backlog-registry.yaml` update, parity test wiring, and `docs/cli-reference/` regeneration)

No cycles exist in the task or shipment dependency graph (`206.001-T -> 206.002-T -> 206.003-T`).

## Decisions and Rationale

* **Modular `internal/core/shipment_manifest.go` file**: `internal/core/shipment.go` is already ~2,950 lines. Placing the manifest replanning operations in `internal/core/shipment_manifest.go` keeps the core package maintainable while reusing `lockShipmentLifecycleGlobal`, `lockShipmentMembership`, `NormalizeShipmentItems`, `MutationEnvelope`, and `persistArtifact`.
* **Canonical lexicographical lock order and durable `move_item` recovery journal in `MoveShipmentItem`**: Always acquiring `lockShipmentLifecycleGlobal` first, then `lockShipmentMembership` in lexicographical order (`min(fromID, toID)` then `max(fromID, toID)`), and persisting a durable `move_item` recovery journal (`internal/core/shipment_recovery.go`) with preimages of both shipments and correlated `intent`/`committed`/`aborted` lifecycle events eliminates AB-BA deadlocks and prevents torn manifests or uncompensated audit events across crashes and `ErrWriteIndeterminate`.
* **Authoritative `.autoharness/backlog-registry.yaml` update**: Because `.autoharness/backlog-registry.yaml` is listed in `.autoharness/drift-ignore` (see commit `a86b8065`), updating `.autoharness/backlog-registry.yaml` directly in U3 keeps the operation registry authoritative for both agents and `internal/cli/registry_parity_test.go` without test-only mappings.

## Risks and Mitigations

* **Risk 1 (Partial state on cross-shipment move failure, crash, or indeterminate write)**: A crash between manifest writes, an `ErrWriteIndeterminate` result, or a failure on the second audit append could leave the item in neither shipment or leave an uncompensated `shipment_item_moved` event in the first shipment's log.
  * **Mitigation**: `MoveShipmentItem` persists a durable `move_item` recovery journal with both shipment preimages and `phase: "intent"` lifecycle events before mutating either manifest; skips in-memory compensation on `ErrWriteIndeterminate` so the next `lockShipmentLifecycleGlobal` barrier recovers both manifests; and if the second `phase: "committed"` audit append fails with `ErrWriteNotApplied`, rolls back both manifests to their preimages and appends a compensating `phase: "aborted"` event with the shared `correlation_id` to the first shipment's log before removing the journal.
* **Risk 2 (Topological sort cycle, absent-parent subtask edges, or nondeterminism in `ReorderShipment --auto`)**: Cyclic `blocks` dependencies among manifest members could infinite-loop, subtasks whose parents are absent from the manifest could have their `blocks` edges ignored, or unsequenced items could reorder nondeterministically across runs.
  * **Mitigation**: Include all non-feature manifest members (both `task` and `subtask` items) as vertices in Kahn's algorithm honoring all intra-manifest `blocks` edges, add default `parent -> subtask` edges only when the parent task is in the manifest and does not transitively depend on the subtask via `blocks`, break ties deterministically (ready subtasks of the most-recently emitted parent task first, then original manifest index), and fail closed with `blerrors.ErrValidation` on cycles.

## Plan Hardening

* `Requires plan hardening`: `yes`
* `Constitution Check`: `pass`

### System-Wide Invariants

* **I1 (Queued-by-default manifest protection & unconditional blocked-manifest seal)**: `RemoveFromShipment`, `MoveShipmentItem`, and `ReorderShipment` MUST unconditionally refuse any `blocked` shipment (`status == "blocked"` or carrying a blocked-manifest seal) with `ErrShipmentBlockedManifestSealed` / `ErrInvalidStatus` / `blerrors.ErrShipmentConflict` even when `opts.Force == true` — both in live execution and in `DryRun: true` preview — requiring governed `UnblockShipment` first. For other non-`queued` shipments (`active`, `shipped`, `abandoned`), they MUST refuse with `blerrors.ErrShipmentConflict` unless `opts.Force == true` AND `strings.TrimSpace(opts.Reason) != ""` (`blerrors.ErrValidation` when `Force` is set without a reason).
* **I2 (Single-shipment membership, status-preserving removal, & durable move conservation)**: `RemoveFromShipment` leaves the item in `.backlogit/queue/` unassigned from the shipment manifest with its current `status` preserved (`queued` for queued shipments, `active` for claimed active shipments) and clears any matching `custom_fields.scheduler_baseline_claim == shipmentID` inside the same compensated mutation. `MoveShipmentItem` MUST preserve total membership across `(fromShipment, toShipment)` via a durable `move_item` recovery journal (`internal/core/shipment_recovery.go`) with exact preimages of both manifests and correlated `intent` / `committed` / `aborted` events — never running in-memory compensation on `ErrWriteIndeterminate` and always compensating both manifests plus appending a correlated `phase: "aborted"` event if the second audit append fails with `ErrWriteNotApplied`. `ReorderShipment` (`--items` and `--auto`) MUST be a strict permutation of `NormalizeShipmentItems(shipment)` (exact same cardinality and member set).
* **I3 (Covering-feature-first, intra-manifest `blocks` topological order, & parent-before-subtask ordering)**: Whenever a shipment contains a root covering feature (`isRootCoveringFeature`), `ReorderShipment` MUST place it before all tasks and subtasks. In `--auto` mode, all non-feature manifest members (both `task` and `subtask` items) are vertices in the Kahn dependency graph honoring every intra-manifest `blocks` edge (`B -> A` when `A depends_on B`), with default `P -> S` edges for in-manifest parent tasks `P` unless `P` transitively depends on `S` via `blocks` edges.
* **I4 (Read-only dry-run purity & audit completeness)**: When `DryRun == true`, `RemoveFromShipment`, `MoveShipmentItem`, and `ReorderShipment` MUST execute a dedicated read-only preview path that checks for unfinished recovery journals read-only (refusing if pending recovery exists) and reads shipment/member Markdown files via `readArtifactFromDiskReadOnly` without calling `lockShipmentLifecycleGlobal`, `GetShipment`, or `loadArtifact` — mutating zero Markdown files, zero SQLite rows (even on a SQLite cache miss), zero filesystem lock files, and zero JSONL event logs. When `DryRun == false`, every manifest mutation MUST atomically update both the Markdown file and the SQLite index (`persistArtifact`) and append an indexed JSONL event (`shipment_item_removed`, `shipment_item_moved`, or `shipment_reordered`).
* **I5 (Never-nil JSON slice parity)**: Every manifest slice on `RemoveFromShipmentResult`, `MoveShipmentItemResult`, and `ReorderShipmentResult` MUST be non-nil (`[]string{}` when empty) so CLI and MCP JSON outputs never emit `null` for array fields.

### Learnings Consulted

* `docs/compound/go-patterns/f015-shipment-stash-patterns.md` — Always read shipment items via `NormalizeShipmentItems` and keep empty slices non-nil (`[]string{}`).
* `docs/compound/2026-07-03-cli-mcp-honest-fallback-map-and-registry-drift-test.md` — Keep CLI and MCP output shapes isomorphic (`[]` never `null`) and satisfy `internal/cli/registry_parity_test.go` (including `TestRegistryParity_FlagAndPositionalParity`).
* `docs/compound/2026-10-19-lock-held-side-effect-ordering.md` — Maintain strict lock acquisition ordering (`lockShipmentLifecycleGlobal` before lexicographically ordered `lockShipmentMembership`).
* `docs/compound/workflow-issues/cli-reference-drift-check-manual-edits-bypass-gen-docs-2026-04-25.md` — Regenerate `docs/cli-reference/` via `go run ./cmd/gen-docs docs/cli-reference` whenever Cobra commands or flags change.

### Strict-Safety Action Classifications (`strict-safety`)

| Unit | ProposedAction | ActionRisk | Safeguard |
|---|---|---|---|
| U1 (`206.001-T`) | Create `internal/core/shipment_manifest.go`, update `internal/core/shipment_recovery.go`, and create `internal/core/shipment_manifest_test.go` implementing `RemoveFromShipment` and `MoveShipmentItem` | `safe_write` | Confined to workspace `internal/core/`; uses durable `move_item` recovery journal + `MutationEnvelope` rollback and `t.TempDir()` in tests |
| U2 (`206.002-T`) | Extend `internal/core/shipment_manifest.go` and `internal/core/shipment_manifest_test.go` with `ReorderShipment` (`--items` and `--auto`) | `safe_write` | Pure permutation validation and deterministic Kahn topological sort over all non-feature tasks and subtasks; tested in `t.TempDir()` |
| U3 (`206.003-T`) | Wire CLI subcommands in `internal/cli/shipment.go`, MCP tools in `internal/mcp/tools.go`, authoritative registry entries in `.autoharness/backlog-registry.yaml`, parity tests in `internal/cli/registry_parity_test.go`, and regenerate `docs/cli-reference/` | `safe_write` | `.autoharness/backlog-registry.yaml` is drift-ignored; verified by `TestRegistryParity_*` and `git diff --exit-code docs/cli-reference/` |

## Runtime Verification and Closure

* **Changed runtime surfaces**:
  * CLI: `backlogit shipment remove`, `backlogit shipment move`, `backlogit shipment reorder` (`internal/cli/shipment.go`).
  * MCP: `backlogit_remove_from_shipment`, `backlogit_move_shipment_item`, `backlogit_reorder_shipment` (`internal/mcp/tools.go`).
  * Core: `core.RemoveFromShipment`, `core.MoveShipmentItem`, `core.ReorderShipment` (`internal/core/shipment_manifest.go`, `internal/core/shipment_recovery.go`).
* **Runtime verification expectations**:
  * Run `go test -count=1 -race ./internal/core/... ./internal/cli/... ./internal/mcp/...` and `go vet ./...`.
  * Verify `go run ./cmd/gen-docs docs/cli-reference` produces zero `git diff` on `docs/cli-reference/` and `backlogit docs lint --path docs/cli-reference` reports 0 violations.
* **Operational closure expectations**:
  * Produce `docs/closure/207-S-206-F-post-merge-closure.md` conforming to the `{shipment_id}-{feature_id}-post-merge-closure.md` filename contract, with top-level `closure_status` and `compaction_status`.
  * Invoke `compact-context` (`target: all`) per P-020 before finalizing `compaction_status` and advancing the Dark Factory cursor to Phase 3 (`A441A74C`).
  * **Rollback trigger and procedure**: If shipment manifest operations or CLI reference drift checks regress in CI, revert the `207-S` merge commit via `git revert -m 1 <merge_sha>` on a reviewed PR.

## Plan Review

* dispatch_mode: single-agent-declared-degradation
* TOOL_DEGRADED: reviewer-subagent-dispatch — single-agent persona pass
* decision: PASS
* attempt: 1
* operator_authorization: approved
* reviewers: Constitution Reviewer, Go Reviewer, Scope Boundary Auditor, Learnings Researcher, Architecture Strategist, Agent-Native Parity Reviewer, Concurrency Reviewer, Security Lens Reviewer, Schema-CLI-Docs Coupling Reviewer (complete sequential rubric pass across all 9 applicable personas, plus PR `#627` Copilot review hardening).
* plan_hardening: `Requires plan hardening: yes` confirmed and satisfied by `## Plan Hardening` (Invariants I1–I5, learnings trace, and `strict-safety` `ProposedAction` / `ActionRisk` classifications).
* constitution_check: `Constitution Check: pass` confirmed; all 11 constitutional principles mapped with zero violations.
* findings:
  * P0: 0, P1: 0, P2: 0, P3: 3 (applied in place, plus 6 PR `#627` review refinements incorporated in place).
  * P3-1 (`Concurrency Reviewer`, U1 `MoveShipmentItem`): Acquire `lockShipmentMembership` on `fromShipmentID` and `toShipmentID` in canonical lexicographical order (`min(from, to)` then `max(from, to)`) under `lockShipmentLifecycleGlobal` and persist a durable `move_item` recovery journal with correlated `intent`/`committed`/`aborted` events so concurrent opposite-direction moves cannot deadlock and partial/indeterminate writes recover cleanly. Applied in place in U1.
  * P3-2 (`Schema-CLI-Docs Coupling Reviewer`, U3 `.autoharness/backlog-registry.yaml`, `registry_parity_test.go`, & `docs/cli-reference/`): Update `.autoharness/backlog-registry.yaml` (drift-ignored in `.autoharness/drift-ignore`) directly with `remove_from_shipment`, `move_shipment_item`, and `reorder_shipment`, and regenerate `docs/cli-reference/` via `go run ./cmd/gen-docs docs/cli-reference` inside U3 so both `TestRegistryParity_*` and CI's `CLI Reference Drift` gate pass against the authoritative registry. Applied in place in U3.
  * P3-3 (`Correctness / Go Reviewer`, U2 `ReorderShipment`): Include all non-feature manifest members (both `task` and `subtask` items) as vertices in the Kahn topological sort graph over intra-manifest `blocks` dependencies, add default `P -> S` edges for in-manifest parent tasks unless `P` transitively depends on `S` via `blocks` edges, and reject cyclic `blocks` graphs with `blerrors.ErrValidation`. Applied in place in U2.
* runtime_verification_and_closure: Targeted `-race` verification across `./internal/core/...`, `./internal/cli/...`, and `./internal/mcp/...`, CLI reference drift verification, and post-merge closure artifact `docs/closure/207-S-206-F-post-merge-closure.md` with P-020 `compact-context` invocation are explicitly specified.
