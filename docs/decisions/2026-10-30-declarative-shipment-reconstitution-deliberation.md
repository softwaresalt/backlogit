---
chunk_strategy: h1-h2-h3
description: 'Deliberation for Phase 1B of the Dark Factory C1+C3 directive (stash 40CC5C6B Slice B; feature 209-F / shipment 210-S): implement declarative multi-shipment manifest reconstitution (backlogit shipment reconstitute --plan <yaml> [--dry-run] [--force --reason <text>] [--json] and MCP backlogit_reconstitute_shipment) composing feature creation, AdoptItemWithOptions re-parenting, topological manifest ordering, blocks dependency rewiring, and all-or-nothing snapshot rollback.'
doc_type: decision
docline:
  decision_status: decided
  depth: standard
  linked_artifacts:
    - docs/design-docs/2026-09-29-backlogit-shipment-manifest-reshuffle-feature-request.md
    - docs/decisions/2026-10-30-shipment-manifest-reshuffle-deliberation.md
    - docs/decisions/2026-10-30-adopt-propagation-id-safety-doctor-manifest-deliberation.md
    - docs/exec-plans/2026-10-30-declarative-shipment-reconstitution-plan.md
    - .backlogit/queue/209-F.md
    - .backlogit/queue/210-S.md
  promoted_to: plan
  tags:
    - shipment
    - manifest
    - reconstitute
    - adopt
    - 40CC5C6B
    - 209-F
    - 210-S
  topic: 'Declarative multi-shipment manifest reconstitution (shipment reconstitute --plan)'
schema_version: "1.0"
source: docs/decisions/2026-10-30-declarative-shipment-reconstitution-deliberation.md
title: 'Deliberation: Declarative multi-shipment manifest reconstitution (40CC5C6B / 209-F)'
---

## Problem Frame

* Active stash entry `40CC5C6B` (*Phase 2b follow-up from DE3E7B67*) originally tracked four capabilities from `docs/design-docs/2026-09-29-backlogit-shipment-manifest-reshuffle-feature-request.md` §§2–5. In Phase 1A (`208-F` / `209-S`, merged on `main` at `34e656a0b85681cca408d18b9c8cddf068db17c3`), items (2), (3), and (4) shipped:
  * `AdoptItemWithOptions` (`KeepID`, atomic rewrite of shipment `custom_fields.items`, `.backlogit/archive/stash.jsonl` `harvested_artifact_id`, and `.backlogit/archive/provenance_corrections.jsonl`),
  * History-safe ID allocation (`NextTypedHierarchicalIDForWorkspace` / `NextIDForWorkspace` in `internal/core/hierarchy.go` and `internal/core/naming.go`),
  * `CorrectStashProvenance` (`--force`) with CLI/MCP parity, and
  * `Doctor` shipment manifest integrity checks (`dangling_shipment_member`, `duplicate_shipment_membership`, `split_feature_across_shipments`, `forward_shipment_dependency`, `unsequenced_queued_shipment` in `internal/core/doctor_shipment_manifest.go`).
* **Phase 1B (this session)** delivers the remaining capability (1) of `40CC5C6B`: **declarative multi-shipment manifest reconstitution (`backlogit shipment reconstitute --plan <path.yaml> [--dry-run] [--force --reason <text>] [--json]` and MCP `backlogit_reconstitute_shipment`)** under covering feature `209-F` and shipment `210-S`.
* When a queued shipment (such as `201-S` in `docs/design-docs/2026-09-29-backlogit-shipment-manifest-reshuffle-feature-request.md` §2) is split into `N` chained shipments with new covering features and re-parented tasks, orchestrating individual `add`, `adopt`, `shipment create`, `shipment move`, `shipment reorder`, and `dep add` commands across dozens of artifacts risks partial state on a midway error and requires manual bookkeeping of renumbered task IDs (`oldID -> newID`). Declarative reconstitution validates the entire multi-shipment plan up front, supports a zero-write `--dry-run` preview returning the full diff and ID mapping, and applies the transformation atomically with byte-identical rollback on any failure.

## Research Findings

* **Primitive manifest foundation (`internal/core/shipment_ops.go`, `internal/core/shipment_manifest.go`)**:
  * `validateShipmentManifestQueueGuard`, `ensureNoPendingShipmentRecoveryReadOnly`, `readArtifactFromDiskReadOnly`, and `readShipmentFromDiskReadOnly` (`internal/core/shipment_ops.go:602-668`) already enforce the queued-only default (unconditionally refusing `blocked` shipments via `hasBlockedShipmentSeal` / `blerrors.ErrShipmentConflict` even with `Force: true`, and requiring `Force: true` + non-empty `Reason` for `active`, `shipped`, or `abandoned` shipments) and provide a zero-write read-only inspection path for `DryRun: true`.
  * `buildReorderGraph`, `autoOrder`, and `validateExplicit` (`internal/core/shipment_manifest.go:108-250`) implement the two-phase dependency graph `G` (root covering features leading, hard intra-manifest `blocks` edges first, cycle-safe parent-before-child edges second). Reusing `buildReorderGraph` inside `ReconstituteShipments` guarantees identical `--auto` and `explicit_order` semantics between `shipment reorder` and `shipment reconstitute`.
* **Adoption & ID allocation foundation (`internal/core/shipment_lifecycle.go`, `internal/core/hierarchy.go`, `internal/core/stash_lock.go`)**:
  * `AdoptItemWithOptions` (`internal/core/shipment_lifecycle.go:1202-1515`) re-parents an item under a target feature (`KeepID: false` allocates a history-safe hierarchical ID via `allocateTypedHierarchicalID` and propagates `oldID -> newID` across `dependencies`, `links`, shipment manifests, `archive/stash.jsonl`, and `archive/provenance_corrections.jsonl`; `KeepID: true` preserves `oldID == newID`).
  * `adoptItem` acquires `lockActiveStash(ws)` (`internal/core/stash_lock.go:62`) when `!opts.KeepID && ws.DB != nil`. Because `lockStashFile` acquires the non-reentrant in-process `stashMu` mutex and `.lock` sidecar, `ReconstituteShipments` must acquire `lockActiveStash(ws)` outermost and mark the context (e.g., `stashLockHeldContextKey{}`) so nested `adoptItem` calls reuse the outer stash lock without deadlocking, followed by `lockShipmentLifecycleGlobal(ctx, ws)` (which runs `recoverPendingShipmentOperations` and is re-entrant via context) and `lockShipmentMembership` on all affected existing shipments in sorted order.
* **Post-apply `Doctor` manifest integrity verification (`internal/core/doctor_shipment_manifest.go`)**:
  * `detectShipmentManifestFindings` (`internal/core/doctor_shipment_manifest.go:38-145`) checks non-terminal shipments for `FindingDanglingShipmentMember`, `FindingDuplicateShipmentMembership`, `FindingSplitFeatureAcrossShipments`, `FindingForwardShipmentDependency`, and `FindingUnsequencedQueuedShipments`. Validating these invariants both in the pre-execution planner (supporting `--dry-run`) and as a post-apply gate before committing guarantees that a reconstituted workspace always leaves `backlogit doctor` clean.

## Options Evaluated

### Option A: Single-Plan Declarative Reconstitution with Read-Only `--dry-run` Planner and Full Snapshot/Restore Rollback (`209-F` / `210-S`) — Chosen

* **Description**: Implement `ReconstitutePlan` YAML parsing and validation, a zero-write read-only `--dry-run` diff planner, and atomic `ReconstituteShipments` execution in `internal/core/shipment_reconstitute.go`, wired to CLI `backlogit shipment reconstitute --plan <path.yaml> [--dry-run] [--force --reason <text>] [--json]` and MCP `backlogit_reconstitute_shipment` across 3 sequential single-domain (`code`) tasks (`209.001-T` -> `209.002-T` -> `209.003-T`):
  * **U1 (`209.001-T`)**: `ReconstitutePlan` YAML schema (with duplicate-YAML-key and unknown-field rejection, `SafeResolve` workspace containment + `os.Lstat` symlink refusal on `plan_path`), comprehensive preflight validation (source shipment status guard, complete partition + `unassigned_items` accounting, single-membership across all non-terminal shipments, no split features across shipments, acyclic shipment `blocks` DAG, no `unsequenced` queued shipments when `>1` queued shipment exists, no `forward_shipment_dependency` across shipments, and two-phase `buildReorderGraph` manifest ordering), and zero-write read-only `DryRun: true` simulation returning `ReconstituteShipmentResult` (`CreatedFeatures`, `CreatedShipments`, `AdoptedIDMap`, `BeforeManifests`, `AfterManifests`, `DependencyChanges`, `PlanDigest`).
  * **U2 (`209.002-T`)**: Live all-or-nothing `ReconstituteShipments` execution under canonical lock ordering (`lockActiveStash` outermost with context re-entrancy marker -> `lockShipmentLifecycleGlobal` -> `lockShipmentMembership` in sorted order -> `lockArtifactMutations`), full pre-mutation byte snapshots of all touched `.md`, `.jsonl`, `archive/stash.jsonl`, and `archive/provenance_corrections.jsonl` files plus tracking of newly created `.md`/`.jsonl` files, composing feature creation (`CreateArtifact` + `related_to` link), `AdoptItemWithOptions` re-parenting with `oldID -> newID` translation across target manifests and `blocks` edges, shipment creation/update with topological ordering, `blocks_on` and `rewire` dependency edge mutations, post-apply `detectShipmentManifestFindings` verification, full byte-identical filesystem + SQLite rollback (`Rehydrate`) on any midway failure, and correlated `shipment_reconstituted` JSONL/SQLite audit events on all source and target shipments.
  * **U3 (`209.003-T`)**: CLI `backlogit shipment reconstitute` (`internal/cli/shipment_manifest.go`, `internal/cli/shipment.go`), MCP `backlogit_reconstitute_shipment` (`internal/mcp/tools.go`, `internal/mcp/shipment_manifest_handlers.go`), `.autoharness/backlog-registry.yaml` (`reconstitute_shipment`), end-to-end 1-to-6 shipment split + CLI/MCP parity tests, and `docs/cli-reference/` regeneration.
* **Pros**:
  * Directly satisfies every acceptance criterion in `docs/design-docs/2026-09-29-backlogit-shipment-manifest-reshuffle-feature-request.md` §2 and `40CC5C6B` item (1).
  * Respects the 2-Hour Rule with 3 tightly scoped `code` units.
  * Full pre-mutation byte snapshot + new-file removal + SQLite `Rehydrate` rollback guarantees byte-identical workspace state if any step fails midway.
* **Cons**:
  * Snapshot/restore rollback must track both pre-existing modified files and newly created artifact/log files; addressed explicitly in U2 with failure-injection tests.

### Option B: Best-Effort Sequential Script Without Snapshot Rollback — Rejected

* **Description**: Execute `CreateArtifact`, `AdoptItem`, and `CreateShipment` sequentially without a read-only dry-run planner or full filesystem/SQLite rollback on midway failure.
* **Cons**:
  * Violates the all-or-nothing requirement in `docs/design-docs/2026-09-29-backlogit-shipment-manifest-reshuffle-feature-request.md` §2 ("A failure injected midway leaves the workspace identical to its pre-command state") and cannot support a zero-write `--dry-run` preview.

## Decision

Adopt **Option A**: promote `40CC5C6B` item (1) to implementation plan `docs/exec-plans/2026-10-30-declarative-shipment-reconstitution-plan.md` under covering feature `209-F` (`209.001-T`..`209.003-T`) and shipment `210-S` (blocked by shipped `209-S`), and harvest `40CC5C6B` into `209-F` so `40CC5C6B` is archived upon harvest.
