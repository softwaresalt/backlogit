---
chunk_strategy: h1-h2-h3
description: 'Implementation plan for feature 209-F (shipment 210-S, Phase 1B, stash 40CC5C6B Slice B): implement ReconstitutePlan YAML schema, all-or-nothing validator, zero-write read-only --dry-run diff planner, atomic ReconstituteShipments execution with AdoptItemWithOptions re-parenting and full snapshot/restore rollback, and CLI/MCP/registry parity for backlogit shipment reconstitute.'
doc_type: plan
docline:
  feature_id: 209-F
  shipment_id: 210-S
  source_decision: docs/decisions/2026-10-30-declarative-shipment-reconstitution-deliberation.md
  stash_id: 40CC5C6B
schema_version: "1.0"
source: docs/exec-plans/2026-10-30-declarative-shipment-reconstitution-plan.md
title: 'Implementation Plan: Declarative multi-shipment manifest reconstitution (209-F / 210-S)'
---

## Objective

Deliver declarative multi-shipment manifest reconstitution (`40CC5C6B` item 1; `docs/design-docs/2026-09-29-backlogit-shipment-manifest-reshuffle-feature-request.md` §2) under covering feature `209-F` and shipment `210-S`:

1. **`ReconstitutePlan` YAML Schema, All-or-Nothing Graph/Partition/Split/Forward-Edge Validator, and Read-Only `--dry-run` Diff Planner (`U1` / `209.001-T`)**: In `internal/core/shipment_reconstitute.go`, define `ReconstitutePlan`, `ReconstituteShipmentSpec`, `ReconstituteFeatureSpec`, `ReconstituteCreateFeatureSpec`, `ReconstituteRewireSpec`, `ReconstituteOptions` (`PlanPath`, `PlanYAML`, `DryRun`, `Force`, `Reason`), and `ReconstituteShipmentResult` (`Status`, `DryRun`, `Force`, `Reason`, `CorrelationID`, `PlanDigest`, `SourceShipments`, `CreatedFeatures`, `CreatedShipments`, `TargetShipments`, `LabelMap`, `AdoptedIDMap`, `BeforeManifests`, `AfterManifests`, `DependencyChanges`). Enforce workspace path confinement (`SafeResolve` / `resolveContainedArtifactPath`) and `os.Lstat` symlink refusal on `PlanPath`, strict YAML decoding (`KnownFields(true)` + duplicate YAML mapping key rejection), source shipment status guards (unconditionally refusing `blocked` shipments with `blerrors.ErrShipmentConflict` even when `Force: true`, and requiring `Force: true` + non-empty `Reason` for non-`queued` shipments), complete partition accounting across `source_shipments` (every non-feature member in `source_shipments` must land in exactly one `shipments[].items` entry unless explicitly listed in `unassigned_items`, and no item may appear in more than one non-terminal shipment manifest across the workspace), automatic re-parenting planning when a task's `parent_id` differs from its target shipment's covering feature (predicting history-safe `oldID -> newID` renumbering or preserving `oldID` when `keep_id: true`), no-split-feature verification across all non-terminal shipments (`FindingSplitFeatureAcrossShipments`), shipment-level `blocks_on` (`prev`, symbolic `label`, or shipment ID) and `rewire` cycle detection (`blerrors.ErrDependencyCycle`), no-unsequenced-queued-shipment enforcement when `>1` queued shipment exists, no `forward_shipment_dependency` across shipments, two-phase topological manifest ordering via `buildReorderGraph` (`auto_order` default `true`, or `explicit_order: true` validated against `G`), and a zero-write read-only `DryRun: true` execution path (refusing unfinished recovery journals via `ensureNoPendingShipmentRecoveryReadOnly` and performing zero writes to Markdown, SQLite, lock files, or JSONL logs).
2. **Atomic `ReconstituteShipments` Execution, `AdoptItemWithOptions` Composition, Full Snapshot/Restore Rollback, and `shipment_reconstituted` Audit Events (`U2` / `209.002-T`)**: Implement live `ReconstituteShipments(ctx, ws, opts)` in `internal/core/shipment_reconstitute.go` under canonical lock ordering (`lockActiveStash(ws)` outermost with a context re-entrancy marker `stashLockHeldContextKey{}` so nested `adoptItem` calls do not re-lock the non-reentrant `stashMu` -> `lockShipmentLifecycleGlobal(ctx, ws)` -> `lockShipmentMembership` on all existing affected shipments in sorted order -> `lockArtifactMutations`). Capture pre-mutation byte snapshots of all affected `.md`, `.jsonl`, `archive/stash.jsonl`, and `archive/provenance_corrections.jsonl` files and track all newly created `.md` and `.jsonl` paths; execute feature creation (`CreateArtifact` + optional `related_to` link), task re-parenting via `AdoptItemWithOptions` (propagating `oldID -> newID` across manifests, `dependencies`, `links`, `archive/stash.jsonl`, and `archive/provenance_corrections.jsonl`), target shipment creation/update (`custom_fields.items` ordered with covering feature first + topological `blocks` order), shipment and downstream `blocks` dependency wiring (`blocks_on` and `rewire`), unassignment of any `unassigned_items` (using `planMemberDetach` to requeue eligible unstarted claim-assigned active members if `--force` was used on an active source shipment), post-apply `detectShipmentManifestFindings` verification (failing closed if any error-severity manifest finding or unsequenced queued shipment exists), and correlated `shipment_reconstituted` JSONL + SQLite-indexed audit events on all source and target shipments (recording `plan_digest`, `reason`, `source_shipments`, `target_shipments`, `created_features`, `created_shipments`, `adopted_id_map`, and `dependency_changes`). On any midway error (including injected write failures or post-apply check failure), restore all snapshotted files byte-for-byte, remove all newly created `.md`/`.jsonl` files, and rehydrate SQLite (`SyncIndex` / `bldb.Rehydrate`) so the workspace is byte-identical to its pre-command state.
3. **CLI `backlogit shipment reconstitute`, MCP `backlogit_reconstitute_shipment`, Registry Mapping, 1-to-N Split E2E Tests, and CLI Docs (`U3` / `209.003-T`)**: Wire `newShipmentReconstituteCmd()` (`backlogit shipment reconstitute --plan <path.yaml> [--dry-run] [--force --reason <text>] [--json]`) in `internal/cli/shipment_manifest.go` and register it in `NewShipmentCmd()` (`internal/cli/shipment.go`); wire `backlogit_reconstitute_shipment` (`plan_path`, `plan_yaml`, `dry_run`, `force`, `reason`) in `internal/mcp/tools.go` and `handleReconstituteShipment` in `internal/mcp/shipment_manifest_handlers.go`; add `reconstitute_shipment` to `.autoharness/backlog-registry.yaml` (`mcp_tool: "backlogit_reconstitute_shipment"`, `cli_command: "backlogit shipment reconstitute --plan {{plan_path}}"`); add end-to-end 1-to-6 shipment split + CLI/MCP parity tests verifying `backlogit doctor` is clean; and regenerate `docs/cli-reference/` via `go run ./cmd/gen-docs docs/cli-reference`.

Source decision: `docs/decisions/2026-10-30-declarative-shipment-reconstitution-deliberation.md` (Option A, stash `40CC5C6B`).

## Activation Contract & Routing Context

* `DARK_MODE_ACTIVE`: `true` (`2026-10-30T20:30:22-07:00`)
* `operator_request_verbatim`: `'Option 1: Run the pipeline in dark factory mode'`
* `bounded_ordered_capability_scope`: `["Phase 1A: 7CE12EE6 + 40CC5C6B Slice A (208-F / 209-S, shipped)", "Phase 1B: 40CC5C6B Slice B shipment reconstitute --plan (209-F / 210-S)", "Phase 2: 21BA4886 ComplexityComposition rollup and harvest auto-stamping (210-F / 211-S)"]`
* `cursor`: `last_completed=209-S, next=Phase 1B (40CC5C6B Slice B -> 209-F / 210-S)`
* `merge_approval_pre_authorized`: `true`
* `admin_fallback_pre_authorized`: `false`
* `routing_note`: `gemini-4-argon/google/high/long_context` (`tier3`).

## Problem Frame

While `207-S` added single-item manifest operations (`shipment remove`, `shipment move`, `shipment reorder`) and `209-S` added `AdoptItemWithOptions` manifest/stash rename propagation, history-safe ID allocation, and `Doctor` manifest integrity checks, splitting an oversized queued shipment into a sequenced chain of multiple shipments with newly created covering features and re-parented tasks still requires manual multi-step orchestration. A declarative `backlogit shipment reconstitute --plan <yaml>` command and `backlogit_reconstitute_shipment` MCP tool compose these primitives into a single validated, dry-runnable, all-or-nothing transaction with full rollback and audit logging.

## Requirements Trace

| ID | Requirement | Source | Implementation Unit |
|---|---|---|---|
| R1 | Implement `ReconstitutePlan` YAML schema (with `SafeResolve` containment + `os.Lstat` symlink refusal on `PlanPath`, `KnownFields(true)`, and duplicate YAML mapping key rejection), all-or-nothing preflight validation (source shipment status guard refusing `blocked` unconditionally and requiring `Force` + `Reason` for non-`queued`; complete partition of `source_shipments` non-feature items across `shipments[].items` + optional `unassigned_items`; single-membership across all non-terminal workspace shipments; automatic re-parenting ID simulation `AdoptedIDMap` when a task's `parent_id` differs from its target shipment's covering feature; no split features across shipments; acyclic shipment `blocks` DAG across `blocks_on` and `rewire`; no `unsequenced` queued shipments when `>1` queued shipment exists; no `forward_shipment_dependency` across shipments; two-phase topological manifest ordering via `buildReorderGraph`), and zero-write read-only `--dry-run` diff planner in `internal/core/shipment_reconstitute.go` | `40CC5C6B` (item 1) & `2026-09-29-backlogit-shipment-manifest-reshuffle-feature-request.md` §2 | U1 (`209.001-T`) |
| R2 | Implement atomic live `ReconstituteShipments` execution in `internal/core/shipment_reconstitute.go` under canonical lock ordering (`lockActiveStash` outermost with context re-entrancy marker `stashLockHeldContextKey{}` -> `lockShipmentLifecycleGlobal` -> `lockShipmentMembership` in sorted order -> `lockArtifactMutations`), composing feature creation (`CreateArtifact` + optional `related_to` link), `AdoptItemWithOptions` re-parenting (`KeepID` or history-safe renumbering with `oldID -> newID` propagation across manifests, `dependencies`, `links`, `archive/stash.jsonl`, and `archive/provenance_corrections.jsonl`), target shipment creation/update with covering-feature-first topological ordering, `blocks_on` and `rewire` dependency edge updates, post-apply `detectShipmentManifestFindings` verification, full byte-identical filesystem + SQLite rollback on any midway failure, and correlated `shipment_reconstituted` JSONL + SQLite-indexed audit events (`EventShipmentReconstituted = "shipment_reconstituted"`) | `40CC5C6B` (item 1) & `2026-09-29-backlogit-shipment-manifest-reshuffle-feature-request.md` §2 | U2 (`209.002-T`) |
| R3 | Wire CLI `backlogit shipment reconstitute --plan <path.yaml> [--dry-run] [--force --reason <text>] [--json]` (`internal/cli/shipment_manifest.go`, `internal/cli/shipment.go`) and MCP `backlogit_reconstitute_shipment` (`internal/mcp/tools.go`, `internal/mcp/shipment_manifest_handlers.go`), register `reconstitute_shipment` in `.autoharness/backlog-registry.yaml`, add end-to-end 1-to-6 shipment split + CLI/MCP parity tests verifying `Doctor` clean, and regenerate `docs/cli-reference/` | `40CC5C6B` (item 1) & `2026-09-29-backlogit-shipment-manifest-reshuffle-feature-request.md` §2 | U3 (`209.003-T`) |

## Constraints

* **2-Hour Rule and Width Isolation**: Each unit stays in the `code` domain, touches 2–4 tightly coupled source/test files (plus `.autoharness/backlog-registry.yaml` and generated `docs/cli-reference/` in U3), and keeps `go test -timeout=30m ./...` green at the end of every wave.
* **Non-nil slice wire-shape & map initialization**: All slice and map fields on `ReconstituteShipmentResult` (`SourceShipments`, `CreatedFeatures`, `CreatedShipments`, `TargetShipments`, `LabelMap`, `AdoptedIDMap`, `BeforeManifests`, `AfterManifests`, `DependencyChanges`) and every shipment's `custom_fields.items` MUST be initialized non-nil (`[]string{}` / `map[string]...{}`) so JSON marshaling never emits `null`.
* **Canonical lock order & re-entrant stash lock**: Because `AdoptItemWithOptions` acquires `lockActiveStash(ws)` (`stashMu`) when renumbering an item, `ReconstituteShipments` MUST acquire `lockActiveStash(ws)` outermost and attach `stashLockHeldContextKey{}` to `ctx` (with `adoptItem` skipping `lockActiveStash(ws)` when `stashLockHeld(ctx)` is true), followed by `lockShipmentLifecycleGlobal(ctx, ws)`, sorted `lockShipmentMembership`, and `lockArtifactMutations`.
* **Genuine tests-first RED -> GREEN**: Every unit specifies the exact failing test selector (`^TestU1_`, `^TestU2_`, `^TestU3_`) that compiles cleanly and is observed RED before production implementation turns it GREEN (`closed_exempt_set: []`).

## Implementation Units

### U1: Implement `ReconstitutePlan` YAML schema, all-or-nothing graph/partition/split/forward-edge validator, and zero-write read-only `--dry-run` diff planner (`code`) — `209.001-T`

* **Domain**: `code`
* **Changes**:
  * **First (RED phase, scaffolded by `harness-architect` in `*_test.go`)**:
    1. In `internal/core/shipment_reconstitute.go`, declare exported types `ReconstitutePlan`, `ReconstituteShipmentSpec`, `ReconstituteFeatureSpec`, `ReconstituteCreateFeatureSpec`, `ReconstituteRewireSpec`, `ReconstituteOptions`, `ReconstituteDependencyChange`, `ReconstituteCreatedArtifact`, and `ReconstituteShipmentResult`, `EventShipmentReconstituted = "shipment_reconstituted"` in `internal/core/shipment_events.go`, and stub `ReconstituteShipments(ctx context.Context, ws *Workspace, opts ReconstituteOptions) (*ReconstituteShipmentResult, error)` returning `nil, blerrors.ErrNotImplemented`.
    2. In `internal/core/shipment_reconstitute_test.go`, add `TestU1_ReconstitutePlanValidationAndDryRun`:
       * `yaml_and_path_safety`: rejects `PlanPath` outside workspace or pointing at a symlink (`os.Lstat`) with `blerrors.ErrValidation`; rejects unknown YAML fields and duplicate YAML mapping keys with `blerrors.ErrValidation`; requires non-empty `source_shipments` and `shipments`.
       * `source_shipment_status_guards`: refuses `blocked` source shipments unconditionally (`errors.Is(err, blerrors.ErrShipmentConflict)`) even when `Force: true`; refuses `active`/`shipped`/`abandoned` source shipments when `Force: false` (`errors.Is(err, blerrors.ErrShipmentConflict)`), refuses `Force: true` with empty `Reason` (`errors.Is(err, blerrors.ErrValidation)`), and allows `active` source shipment with unstarted claimed members when `Force: true` and `Reason: "replan"`.
       * `partition_and_membership_validation`: rejects a plan that omits a task originally in `source_shipments` unless that task is listed in `unassigned_items`; rejects duplicate item assignment across `shipments[]` or assignment of an item that already belongs to another live non-terminal shipment outside `source_shipments`; rejects a plan whose resulting manifests split tasks/subtasks of the same covering feature across multiple non-terminal shipments (`FindingSplitFeatureAcrossShipments`).
       * `dag_and_forward_edge_validation`: resolves `blocks_on` (`prev`, symbolic `label`, or shipment ID) and `rewire` rules; rejects a shipment-level `blocks` cycle (`errors.Is(err, blerrors.ErrDependencyCycle)` or `blerrors.ErrValidation`); rejects an unsequenced queued shipment when `>1` queued shipment exists after the plan (unless it carries a `blocks` edge on another shipment or `dag-root` label); rejects a plan where an item in an earlier shipment `S1` has a `blocks` dependency on an item in a later shipment `S2` where `S2` depends on `S1` (`forward_shipment_dependency`).
       * `dry_run_zero_write_and_diff_projection`: running `ReconstituteShipments` with `DryRun: true` on a 1-to-3 shipment split (creating 2 new features and 2 new shipments, keeping `002-S`, re-parenting tasks `001.002-T` and `001.003-T` under the new features, ordering manifests via `--auto`, and rewiring downstream shipment `003-S` onto `002-S`) refuses if an unfinished shipment lifecycle journal exists, performs zero writes to Markdown, SQLite (even on a cache miss), lock files, or JSONL logs, and returns `Status: "dry_run"`, `DryRun: true`, non-empty `PlanDigest` (SHA-256 hex), predicted `CreatedFeatures`, `CreatedShipments`, `AdoptedIDMap` (`"001.002-T": "004.001-T"`, `"001.003-T": "005.001-T"`), `BeforeManifests`, `AfterManifests` (with covering feature first + topological order), and `DependencyChanges`.
    3. Observe RED: `go test -timeout=30m -run=^$ -count=1 ./...` exits 0 and `go test -timeout=30m -count=1 -run '^TestU1_ReconstitutePlanValidationAndDryRun$' ./internal/core` fails.
  * **Second (GREEN phase, implemented by `build-feature`)**:
    1. Implement YAML decoding with custom `UnmarshalYAML` on `ReconstituteFeatureSpec` (accepting either a scalar string `"195-F"` or mapping `{ id: "195-F" }` / `{ create: { title: "...", priority: "...", description: "...", related_to: "195-F" } }`) and a recursive `yaml.Node` duplicate-mapping-key check + `yaml.NewDecoder` `KnownFields(true)`.
    2. Implement `loadAndValidateReconstitutePlan` and `planReconstitution` in `internal/core/shipment_reconstitute.go` covering all validation rules and simulating history-safe feature/shipment/task ID allocations (`NextTypedHierarchicalIDForWorkspace`) and two-phase `buildReorderGraph` manifest ordering for `DryRun: true`.
* **Affected files**:
  * `internal/core/shipment_reconstitute.go`
  * `internal/core/shipment_events.go`
  * `internal/core/shipment_reconstitute_test.go`
* **Acceptance criteria**:
  * RED phase is observed failing in `go test -timeout=30m -count=1 -run '^TestU1_ReconstitutePlanValidationAndDryRun$' ./internal/core`.
  * GREEN phase passes `go test -timeout=30m -count=1 -race ./internal/core/...`.
* **Execution posture**: test-first.

### U2: Implement atomic `ReconstituteShipments` execution composing feature creation, `AdoptItemWithOptions` re-parenting, topological manifest ordering, `blocks` dependency rewiring, full snapshot/restore rollback, and `shipment_reconstituted` audit events (`code`) — `209.002-T`

* **Domain**: `code`
* **Depends on**: U1 (`209.001-T`)
* **Changes**:
  * **First (RED phase, scaffolded by `harness-architect` in `*_test.go`)**:
    1. In `internal/core/shipment_reconstitute_test.go`, add `TestU2_ReconstituteShipmentsAtomicExecutionAndRollback`:
       * `atomic_reconstitution_and_audit_events`: live `ReconstituteShipments` (`DryRun: false`) on a queued shipment with 6 tasks splits it into 3 sequenced shipments (`ship-1 <- ship-2 <- orig-S <- downstream-S`), creates 2 new covering features (with `related_to` links to the original feature), re-parents 4 tasks via `AdoptItemWithOptions` (propagating `oldID -> newID` across intra-plan item `blocks` dependencies, shipment manifests, `archive/stash.jsonl` `harvested_artifact_id`, and `archive/provenance_corrections.jsonl`), orders each shipment's `custom_fields.items` with its covering feature first followed by topological `blocks` order, rewires `downstream-S` `blocks` dependencies, appends correlated `shipment_reconstituted` events (carrying `correlation_id`, `plan_digest`, `reason`, `source_shipments`, `target_shipments`, `created_features`, `created_shipments`, `adopted_id_map`, and `dependency_changes`) to each source and target shipment log, and leaves `Doctor` completely clean (`len(report.Findings) == 0`). Also verifies `keep_id: true` re-parenting preserves task IDs when requested.
       * `midway_failure_restores_byte_identical_workspace`: injects failures at three points during `ReconstituteShipments` — (a) after creating new features and adopting tasks (during target shipment write), (b) during `rewire` dependency persistence, and (c) during `shipment_reconstituted` event append — and verifies that in every case `ReconstituteShipments` returns an error, removes all newly created `.md` and `.jsonl` files, restores all pre-existing `.md`, `.jsonl`, `archive/stash.jsonl`, and `archive/provenance_corrections.jsonl` files byte-for-byte, and rehydrates SQLite so both the on-disk workspace and SQLite index are identical to their pre-command state.
    2. Observe RED: `go test -timeout=30m -run=^$ -count=1 ./...` exits 0 and `go test -timeout=30m -count=1 -run '^TestU2_ReconstituteShipmentsAtomicExecutionAndRollback$' ./internal/core` fails.
  * **Second (GREEN phase, implemented by `build-feature`)**:
    1. In `internal/core/stash_lock.go` and `internal/core/shipment_lifecycle.go`, add `stashLockHeldContextKey{}` and `withStashLockHeld(ctx)` / `stashLockHeld(ctx)` so `adoptItem` skips calling `lockActiveStash(ws)` when the outer `ReconstituteShipments` caller already holds the active stash lock.
    2. In `internal/core/shipment_reconstitute.go`, implement live `ReconstituteShipments` execution:
       * Acquire `lockActiveStash(ws)` outermost (attaching `withStashLockHeld(ctx)`), then `lockShipmentLifecycleGlobal(ctx, ws)`, then `lockShipmentMembership(ctx, ws, id)` for all existing shipments in sorted order, then `lockArtifactMutations` for all existing source/rewire shipments, features, and member items.
       * Re-validate the plan under lock and snapshot the entire `.backlogit/` workspace state (`queue/`, `archive/`, `logs/`) so any modified file has its exact pre-command bytes recorded and any newly created file path can be removed on rollback.
       * Create requested features (`CreateArtifact` + `AddLink` when `related_to` is specified), adopt re-parented tasks via `AdoptItemWithOptions` (recording `oldID -> newID` in `AdoptedIDMap`), create new shipments or update existing source shipments with topologically ordered `custom_fields.items` (and detach any `unassigned_items` via `planMemberDetach`), apply `blocks_on` and `rewire` dependency edges (`AddDependency` / `RemoveDependency`), run `detectShipmentManifestFindings` to verify zero manifest errors, and append correlated `shipment_reconstituted` events via `ws.appendShipmentEvent`.
       * If any step returns an error, restore all snapshotted files, delete all newly created files in `queue/` and `logs/`, run `bldb.Rehydrate` to restore SQLite to match the restored Markdown/JSONL files, and return the wrapped error.
* **Affected files**:
  * `internal/core/shipment_reconstitute.go`
  * `internal/core/stash_lock.go`
  * `internal/core/shipment_lifecycle.go`
  * `internal/core/shipment_reconstitute_test.go`
* **Acceptance criteria**:
  * RED phase is observed failing in `go test -timeout=30m -count=1 -run '^TestU2_ReconstituteShipmentsAtomicExecutionAndRollback$' ./internal/core`.
  * GREEN phase passes `go test -timeout=30m -count=1 -race ./internal/core/...`.
* **Execution posture**: test-first.

### U3: Wire CLI `backlogit shipment reconstitute` and MCP `backlogit_reconstitute_shipment`, update `.autoharness/backlog-registry.yaml`, add end-to-end 1-to-N split + parity tests, and regenerate `docs/cli-reference/` (`code`) — `209.003-T`

* **Domain**: `code`
* **Depends on**: U2 (`209.002-T`)
* **Changes**:
  * **First (RED phase, scaffolded by `harness-architect` in `*_test.go`)**:
    1. In `internal/cli/shipment_reconstitute_test.go`, add `TestU3_ShipmentReconstituteCLIMCPParityAnd201SScenario`:
       * End-to-end `201-S` scenario (`docs/design-docs/2026-09-29-backlogit-shipment-manifest-reshuffle-feature-request.md` Acceptance Criterion 1): 1 queued shipment (`201-S` with feature `195-F` and 12 tasks, preceded by `190-S` and followed by `198-S`) is split into 6 chained shipments (`190-S <- S2 <- S3 <- S4 <- S5 <- S6 <- 201-S <- 198-S`) with 5 newly created covering features and 9 re-parented tasks via one `backlogit shipment reconstitute --plan <path.yaml> --dry-run --json` call (verifying zero workspace mutations and matching predicted IDs) followed by one live `backlogit shipment reconstitute --plan <path.yaml> --json` call, with zero hand edits, and ends with `backlogit doctor` reporting 0 findings.
       * MCP `backlogit_reconstitute_shipment` parity test exercising both `plan_path` and `plan_yaml` (inline YAML) in `dry_run: true` and live modes, plus `.autoharness/backlog-registry.yaml` `reconstitute_shipment` verification under `TestRegistryParity_*`.
    2. Observe RED: `go test -timeout=30m -run=^$ -count=1 ./...` exits 0 and `go test -timeout=30m -count=1 -run '^TestU3_ShipmentReconstituteCLIMCPParityAnd201SScenario$' ./internal/cli` fails.
  * **Second (GREEN phase, implemented by `build-feature`)**:
    1. In `internal/cli/shipment_manifest.go`, add `newShipmentReconstituteCmd()` (`Use: "reconstitute"`, `--plan` required string flag, `--dry-run`, `--force`, `--reason`, and `--json` bool flag) calling `core.ReconstituteShipments`, and register it in `NewShipmentCmd()` in `internal/cli/shipment.go`.
    2. In `internal/mcp/tools.go` and `internal/mcp/shipment_manifest_handlers.go`, register `backlogit_reconstitute_shipment` (`plan_path`, `plan_yaml`, `dry_run`, `force`, `reason`) and implement `handleReconstituteShipment` using `s.manifestWorkspace(ctx, opts.DryRun)`.
    3. In `.autoharness/backlog-registry.yaml`, add `reconstitute_shipment` (`mcp_tool: "backlogit_reconstitute_shipment"`, `cli_command: "backlogit shipment reconstitute --plan {{plan_path}}"`, `params: { plan_path: "plan_path", plan_yaml: "plan_yaml", dry_run: "dry_run", force: "force", reason: "reason" }`).
    4. Run `go run ./cmd/gen-docs docs/cli-reference` to generate `docs/cli-reference/backlogit_shipment_reconstitute.md` and update `docs/cli-reference/backlogit_shipment.md`.
* **Affected files**:
  * `internal/cli/shipment_manifest.go`
  * `internal/cli/shipment.go`
  * `internal/cli/shipment_reconstitute_test.go`
  * `internal/mcp/tools.go`
  * `internal/mcp/shipment_manifest_handlers.go`
  * `.autoharness/backlog-registry.yaml`
  * `docs/cli-reference/backlogit_shipment.md`
  * `docs/cli-reference/backlogit_shipment_reconstitute.md`
* **Acceptance criteria**:
  * RED phase is observed failing in `go test -timeout=30m -count=1 -run '^TestU3_ShipmentReconstituteCLIMCPParityAnd201SScenario$' ./internal/cli`.
  * GREEN phase passes `go test -timeout=30m -count=1 -race ./internal/cli/... ./internal/mcp/...`.
  * `go run ./cmd/gen-docs docs/cli-reference` followed by `git diff --exit-code docs/cli-reference/` exits 0, and `backlogit docs lint --path docs/cli-reference` reports 0 violations.
* **Execution posture**: test-first.

## Dependency Graph

* **Shipment sequencing**: `209-S` (shipped) -> `210-S` (`210-S` carries an explicit `blocks` dependency on `209-S`).
* **Wave 1**: `209.001-T` (U1 — `ReconstitutePlan` YAML schema, all-or-nothing validator, and read-only `--dry-run` diff planner)
* **Wave 2**: `209.002-T` (U2 — blocked by `209.001-T`; atomic `ReconstituteShipments` execution, `AdoptItemWithOptions` composition, snapshot/restore rollback, and `shipment_reconstituted` events)
* **Wave 3**: `209.003-T` (U3 — blocked by `209.002-T`; CLI `backlogit shipment reconstitute` and MCP `backlogit_reconstitute_shipment` wiring, `.autoharness/backlog-registry.yaml`, 1-to-6 split E2E tests, and `docs/cli-reference/` regeneration)

## Constitution Check

* `Constitution Check`: `pass`
* **CQRS & Durable Markdown/JSONL Authority**: All reconstitution writes update Markdown and JSONL atomically and synchronize SQLite (`items`, `item_deps`, `item_links`, `stash_links`, `item_log_entries`), with full byte-snapshot restoration + SQLite `Rehydrate` rollback on any midway error.
* **CLI/MCP Parity**: `backlogit shipment reconstitute` and `backlogit_reconstitute_shipment` share `core.ReconstituteShipments`, return identical JSON structures, and are paired in `.autoharness/backlog-registry.yaml`.

## Plan Hardening Signals

* `Requires plan hardening`: `yes`

## Plan Hardening

### System-Wide Invariants

* **I1 (Canonical lock order & re-entrant stash lock across multi-shipment reconstitution)**: `ReconstituteShipments` MUST acquire locks in the canonical order `lockActiveStash(ws)` (attaching `stashLockHeldContextKey{}` to `ctx` so nested `adoptItem` calls do not re-lock the non-reentrant `stashMu`) -> `lockShipmentLifecycleGlobal(ctx, ws)` (running `recoverPendingShipmentOperations` before any artifact lock) -> `lockShipmentMembership(ctx, ws, shipmentID)` for all existing affected shipments in sorted order -> `lockArtifactMutations`.
* **I2 (Zero-write read-only `--dry-run` preview)**: When `opts.DryRun == true`, `ReconstituteShipments` MUST verify `ensureNoPendingShipmentRecoveryReadOnly(ws)` and inspect artifacts via `readArtifactFromDiskReadOnly` / `scanCanonicalArtifacts` without opening SQLite for writes, creating lock sidecars, or mutating any `.md` or `.jsonl` file, while returning the exact predicted `CreatedFeatures`, `CreatedShipments`, `AdoptedIDMap`, `BeforeManifests`, `AfterManifests`, and `DependencyChanges`.
* **I3 (Complete partition, single-membership, no split features, and no forward `blocks` edges)**: Every non-feature member in `source_shipments` MUST land in at most one `shipments[].items` entry (and must be accounted for in `shipments[].items` or explicit `unassigned_items` so no task is silently dropped); no item may belong to more than one non-terminal shipment across the workspace; no feature's tasks/subtasks may span multiple non-terminal shipments; the resulting shipment `blocks` DAG must be acyclic with no unsequenced queued shipments (when `>1` queued shipment exists) and no item-level `forward_shipment_dependency`.
* **I4 (Byte-identical all-or-nothing rollback on midway failure)**: Before applying any mutation in live mode, `ReconstituteShipments` MUST snapshot all pre-existing files under `.backlogit/` (`queue/`, `archive/`, `logs/`) that may be modified and track all newly created `.md` and `.jsonl` paths. If any step or the post-apply `detectShipmentManifestFindings` check fails, `ReconstituteShipments` MUST restore all snapshotted files byte-for-byte, remove all newly created files, and run `bldb.Rehydrate` so both disk and SQLite are identical to their pre-command state.
* **I5 (`-timeout=30m` on all race/package test invocations & non-nil JSON slices/maps)**: All `go test` verification commands in `internal/core`, `internal/cli`, and `internal/mcp` MUST include `-timeout=30m` to prevent default 10m timeout expirations under `-race`, and all slice/map fields on `ReconstituteShipmentResult` must be initialized non-nil.

### Learnings Consulted

* `docs/compound/2026-10-19-lock-held-side-effect-ordering.md` — Prevent same-goroutine deadlock on non-reentrant `stashMu` when `ReconstituteShipments` invokes `AdoptItemWithOptions` by acquiring `lockActiveStash` outermost and passing `stashLockHeldContextKey{}` on `ctx`.
* `docs/compound/2026-08-31-duplicate-yaml-keys-survive-safe-load-validation.md` — Reject duplicate YAML mapping keys on `yaml.Node` in addition to `KnownFields(true)` when parsing `ReconstitutePlan`.
* `docs/compound/2026-07-21-omitempty-defeats-arrays-always-json-contract.md` — Initialize all result slices and maps non-nil so JSON output never emits `null`.
* `docs/compound/workflow-issues/cli-reference-drift-check-manual-edits-bypass-gen-docs-2026-04-25.md` — Always run `go run ./cmd/gen-docs docs/cli-reference` when adding a CLI subcommand.

### Strict-Safety Action Classifications (`strict-safety`)

| Unit | ProposedAction | ActionRisk | Safeguard |
|---|---|---|---|
| U1 (`209.001-T`) | Create `internal/core/shipment_reconstitute.go` and `internal/core/shipment_reconstitute_test.go`; update `internal/core/shipment_events.go` | `safe_write` | Path confinement (`SafeResolve`), `os.Lstat` symlink refusal, strict YAML validation, and zero-write `DryRun` tested in `t.TempDir()` |
| U2 (`209.002-T`) | Update `internal/core/shipment_reconstitute.go`, `internal/core/stash_lock.go`, `internal/core/shipment_lifecycle.go`, and `internal/core/shipment_reconstitute_test.go` | `safe_write` | Canonical lock order + full pre-mutation byte-snapshot restoration + SQLite `Rehydrate` rollback tested with failure injection in `t.TempDir()` |
| U3 (`209.003-T`) | Update `internal/cli/shipment_manifest.go`, `internal/cli/shipment.go`, `internal/mcp/tools.go`, `internal/mcp/shipment_manifest_handlers.go`, `.autoharness/backlog-registry.yaml`, add `internal/cli/shipment_reconstitute_test.go`, and regenerate `docs/cli-reference/` | `safe_write` | Verified by `TestRegistryParity_*`, 1-to-6 split E2E test, and `git diff --exit-code docs/cli-reference/` |

## Runtime Verification and Closure

* **Changed runtime surfaces**:
  * Core: `core.ReconstitutePlan`, `core.ReconstituteOptions`, `core.ReconstituteShipmentResult`, `core.ReconstituteShipments`, `core.EventShipmentReconstituted`.
  * CLI & MCP: `backlogit shipment reconstitute --plan <path.yaml> [--dry-run] [--force --reason <text>] [--json]` and `backlogit_reconstitute_shipment`.
* **Runtime verification expectations**:
  * Run `go test -timeout=30m -count=1 -race ./internal/core/... ./internal/cli/... ./internal/mcp/...` and `go vet ./...`.
  * Verify `go run ./cmd/gen-docs docs/cli-reference` produces zero `git diff` on `docs/cli-reference/` and `backlogit docs lint --path docs/cli-reference` reports 0 violations.
* **Operational closure expectations**:
  * Produce `docs/closure/210-S-209-F-post-merge-closure.md` with top-level `closure_status` and `compaction_status`.
  * **Rollback trigger and procedure**: If `shipment reconstitute` regresses in CI, revert the `210-S` merge commit via `git revert -m 1 <merge_sha>` on a reviewed PR.

## Plan Review

* dispatch_mode: single-agent-declared-degradation
* TOOL_DEGRADED: reviewer-subagent-dispatch — single-agent persona pass
* decision: PASS
* attempt: 1
* operator_authorization: approved
* reviewers: Constitution Reviewer, Go Reviewer, Scope Boundary Auditor, Learnings Researcher, Architecture Strategist, Agent-Native Parity Reviewer, Concurrency Reviewer, Security Lens Reviewer, SQLite Reviewer, Schema-CLI-Docs Coupling Reviewer.
* plan_hardening: `Requires plan hardening: yes` satisfied by `## Plan Hardening` (Invariants I1–I5, learnings consulted, and `strict-safety` classifications).
* constitution_check: `Constitution Check: pass` confirmed with zero violations.
* findings:
  * P0: 0, P1: 0, P2: 0, P3: 2 (applied in place).
  * P3-1 (`Concurrency Reviewer`, U2 `internal/core/stash_lock.go` & `internal/core/shipment_lifecycle.go`): Guard against same-goroutine deadlock on `stashMu` when `ReconstituteShipments` holds `lockActiveStash(ws)` outermost and calls `AdoptItemWithOptions` by propagating `stashLockHeldContextKey{}` on `ctx`. Applied in I1 and U2.
  * P3-2 (`Schema-CLI-Docs Coupling Reviewer`, U3 `internal/cli/shipment_manifest.go` & `.autoharness/backlog-registry.yaml`): Mark `--plan` required on `newShipmentReconstituteCmd` and expose `--json` so `TestRegistryParity_FlagAndPositionalParity` and CLI `--json` callers both succeed. Applied in U3.
