---
chunk_strategy: h1-h2-h3
description: "Execution plan for 152-F: governed lifecycle reconciliation and stash provenance correction (post-adversarial-review, post-Copilot-review revision)"
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-08-29-152-lifecycle-reconciliation-plan.md
title: "152-F Execution Plan — Governed Lifecycle Reconciliation and Stash Provenance Correction"
---

# 152-F Execution Plan (Revised — Post-Adversarial + Post-Copilot Review)

**Feature**: 152-F — Governed Lifecycle Reconciliation and Stash Provenance Correction
**Deliberation**: `docs/decisions/2026-08-29-152-lifecycle-reconciliation-deliberation.md`
**Adversarial Review**: `docs/decisions/2026-08-29-152-adversarial-review.md`
**Shipment**: 134-S

## Constitution Check

| Principle | Compliance |
|-----------|-----------|
| I. Safety-First Go | All new code is Go 1.24.0; errors wrapped with context |
| II. Test-First (P-002) | Each wave follows declaration → RED harness → GREEN impl; FC-1..FC-3 enforced |
| III. Workspace Isolation | All paths resolve within workspace root via `WorkspaceStorageRoot`; validated |
| IV. CLI Containment | No files outside cwd tree |
| V. Observability | Durable events in tracked custom_fields + tracked provenance_corrections.jsonl |
| VI. Single Responsibility | Uses existing primitives; minimal new surface |
| VII. Destructive Approval | No destructive operations |
| VIII. Safety Modes | Fail-closed design; invalid IDs rejected; rollback on partial failure |
| IX. Git-Friendly | Markdown artifacts with YAML frontmatter |
| X. Context Efficiency | Structured query results |
| XI. Merge Commits | P-009 enforced |

Constitution Check: pass

## Adversarial Review Remediations Incorporated

- **HC-1**: Provenance correction writes to tracked `provenance_corrections.jsonl` in workspace archive dir; rehydration and merge-sync resolve canonical delivery; 151-F frontmatter preserved
- **HC-2**: Single `lockArtifactMutations` held for full sequence; rollback-to-archive on MoveItem failure; ArchiveItem gated on verified done status
- **HC-3**: Precondition check — if `archived_status == target_status`, return NoOp immediately
- **MC-1**: Pre-check children status before MoveItem
- **MC-2**: `WithCascade(false)` on re-archive step
- **MC-3**: Reconciliation metadata recorded in artifact `custom_fields` atomically with re-archive
- **LC-2**: Validate restored status → target_status path exists in DefaultTransitions

## Artifact Class Consistency Rule

Stash entries carry a `kind` field (feature, task, bug, epic, unknown). Artifacts carry `artifact_type` (feature, task, subtask, deliberation, shipment). The canonical-delivery validation does NOT require `kind == artifact_type` because stash kind reflects the operator's initial classification at intake, while artifact_type reflects the structural position chosen at harvest time. A stash with `kind: task` harvested into a feature (`artifact_type: feature`) is normal workflow — the operator decided the fix needed a feature-level scope. Validation checks only: (1) stash archive entry exists, (2) canonical delivery artifact exists, (3) canonical delivery artifact's `source_stash_id` matches the stash ID.

## Task Decomposition

### Wave 1: Core Lifecycle Reconciliation (152.001-T through 152.004-T)

#### 152.001-T — Declarations and stubs: ReconcileArchivedLifecycle types

Create `internal/core/archive_reconcile.go` with:
- `ReconciliationRequest` struct with fields: `ItemIDs []string`, `TargetStatus string`, `Reason string`, `Actor string`, `IdempotencyKey string`
- `ReconciliationItemResult` struct with fields: `ID string`, `Outcome string`, `Error string`
- `ReconciliationResult` struct with fields: `Items []ReconciliationItemResult`, `Outcome string`
- `ReconcileArchivedLifecycle(ctx, db, ws, req)` function stub returning `ErrNotImplemented`

This task provides the compilation surface that enables the RED harness in 152.002-T.

**Files**: `internal/core/archive_reconcile.go` (new)
**Dependencies**: none
**Effort**: ~15 min

#### 152.002-T — RED harness: ReconcileArchivedLifecycle unit tests

**P-002 Contract**: Separate RED commit with compiling tests that FAIL against stubs from 152.001-T.

Tests (table-driven with `t.Run` in `internal/core/archive_reconcile_test.go`):
- `TestReconcileArchivedLifecycle_HappyPath`: archived item with `archived_status: active` → reconciled to `done`, custom_fields has reconciliation metadata, result is `Completed`
- `TestReconcileArchivedLifecycle_AlreadyDone_NoOp`: archived item with `archived_status: done` → `NoOp` result (HC-3)
- `TestReconcileArchivedLifecycle_NotArchived_Error`: item in `active` status → error returned
- `TestReconcileArchivedLifecycle_NotFound_Error`: nonexistent ID → error returned
- `TestReconcileArchivedLifecycle_EmptyReason_Error`: empty reason → validation error
- `TestReconcileArchivedLifecycle_EmptyActor_Error`: empty actor → validation error
- `TestReconcileArchivedLifecycle_IdempotencyKey_Repeat`: same key repeated → `NoOp`
- `TestReconcileArchivedLifecycle_MultipleItems`: batch of 2 items → both reconciled
- `TestReconcileArchivedLifecycle_PartialFailure_Rollback`: one valid, one with MoveItem failure → valid item rolls back to archive, partial result (HC-2)
- `TestReconcileArchivedLifecycle_PathTraversal_Rejected`: ID with path traversal → rejected
- `TestReconcileArchivedLifecycle_NoCascadeOnReArchive`: re-archive uses WithCascade(false) (MC-2)
- `TestReconcileArchivedLifecycle_InvalidTransitionPath_Error`: archived_status: queued, target: done — queued→done not in transitions → error (LC-2)
- `TestReconcileArchivedLifecycle_MoveItemFails_RollbackToArchive`: MoveItem error → item re-archived with original archived_status (HC-2)
- `TestReconcileArchivedLifecycle_DurableCustomFieldsMetadata`: reconciliation metadata in custom_fields is persisted atomically with re-archive (MC-3)

**Files**: `internal/core/archive_reconcile_test.go` (new)
**Dependencies**: 152.001-T
**Effort**: ~1 hr

#### 152.003-T — GREEN implementation: ReconcileArchivedLifecycle core

**P-002 Contract**: Implementation that makes 152.002-T tests pass.

Algorithm:
1. Validate request (non-empty reason, actor, at least one ID, valid target_status)
2. Acquire `lockArtifactMutations(ctx, ws, req.ItemIDs)` — SINGLE lock for all IDs (HC-2)
3. For each item:
   a. Find artifact, verify `status == "archived"` and `archived_status` exists
   b. If `archived_status == target_status` → NoOp (HC-3)
   c. Validate transition path: check `archived_status → target_status` in DefaultTransitions (LC-2)
   d. Check idempotency key if provided
   e. Pre-check children: verify no non-terminal children (MC-1)
   f. `UnarchiveItem` — restores to archived_status
   g. `MoveItem` to target_status — if fails: re-archive with original archived_status, record error result (HC-2)
   h. Verify status == target_status before proceeding
   i. Prepare reconciliation custom_fields metadata: `reconciled_at`, `reconciled_by`, `reconciled_reason`
   j. `ArchiveItem` with `WithCascade(false)` (MC-2) — the ArchiveItem call stores `archived_status: done` and the reconciliation metadata is written atomically in the same frontmatter serialization
   k. Append `lifecycle_reconciliation` event to item log (supplementary non-durable audit)
4. Return structured result

Failure semantics:
- MoveItem failure → rollback: re-archive with original archived_status → item result is "error"
- ArchiveItem failure → item is left at target_status in queue (forward-recoverable: re-run reconcile, which will find item not archived and return error; operator can archive manually)
- This is NOT "no partial-done state" — ArchiveItem failure explicitly leaves item at done in queue as a defined forward-recovery state

**Files**: `internal/core/archive_reconcile.go` (modify from stubs)
**Dependencies**: 152.002-T
**Effort**: ~1.5 hr

#### 152.004-T — CLI reconcile and MCP backlogit_reconcile_archived_lifecycle surfaces

CLI: `backlogit reconcile <id1> [id2...] --reason <reason> --actor <actor> [--target-status done] [--idempotency-key <key>]`
MCP: `backlogit_reconcile_archived_lifecycle` tool with params: `item_ids` (required array), `reason` (required), `actor` (required), `target_status` (optional, default "done"), `idempotency_key` (optional)

Includes surface tests (RED+GREEN combined since thin wrappers over core):
- CLI: `TestReconcileCLI_HappyPath`, `TestReconcileCLI_MissingArgs`, `TestReconcileCLI_InvalidID`
- MCP: `TestHandleReconcileArchivedLifecycle_HappyPath`, `TestHandleReconcileArchivedLifecycle_ValidationError`

**Files**: `internal/cli/reconcile.go`, `internal/cli/reconcile_test.go`, `internal/mcp/tools_reconcile.go`, `internal/mcp/tools_reconcile_test.go` (all new), registration in `internal/cli/root.go` and `internal/mcp/tools.go`
**Dependencies**: 152.003-T
**Effort**: ~1 hr

### Wave 2: Stash Provenance Correction (152.005-T through 152.008-T)

#### 152.005-T — Declarations and stubs: CorrectStashProvenance types

Create `internal/core/stash_provenance.go` with:
- `StashProvenanceCorrectionRequest` struct: `StashID`, `CanonicalDeliveryArtifactID`, `Reason`, `Actor`
- `StashProvenanceCorrectionResult` struct: `StashID`, `OriginalHarvestID`, `CanonicalDeliveryID`, `Outcome`
- `ProvenanceCorrectionRecord` struct for the tracked JSONL entries
- `CorrectStashProvenance(ctx, ws, req)` function stub returning `ErrNotImplemented`

**Files**: `internal/core/stash_provenance.go` (new)
**Dependencies**: none (can parallel Wave 1)
**Effort**: ~15 min

#### 152.006-T — RED harness: CorrectStashProvenance unit tests

**P-002 Contract**: Separate RED commit with compiling tests that FAIL against stubs.

Tests (table-driven in `internal/core/stash_provenance_test.go`):
- `TestCorrectStashProvenance_HappyPath`: correction recorded in tracked `provenance_corrections.jsonl` resolved via `WorkspaceStorageRoot`, original harvested_artifact_id preserved
- `TestCorrectStashProvenance_AlreadyCorrected_NoOp`: same correction repeated → `NoOp`
- `TestCorrectStashProvenance_ConflictingCorrection_Error`: different canonical ID for same stash → rejected
- `TestCorrectStashProvenance_StashNotFound_Error`: nonexistent stash ID → error
- `TestCorrectStashProvenance_ArtifactNotFound_Error`: nonexistent artifact ID → error
- `TestCorrectStashProvenance_SourceStashMismatch_Error`: artifact's source_stash_id doesn't match → error
- `TestCorrectStashProvenance_EmptyReason_Error`: empty reason → validation error
- `TestCorrectStashProvenance_EmptyActor_Error`: empty actor → validation error
- `TestCorrectStashProvenance_EventDurability`: correction written to TRACKED provenance_corrections.jsonl (HC-1)
- `TestCorrectStashProvenance_RehydrationResolvesCanonical`: after correction + sync, `stash_links.item_id` resolves to canonical delivery artifact
- `TestCorrectStashProvenance_ConcurrentConflict_Serialized`: concurrent corrections serialized by stash-file lock

**Files**: `internal/core/stash_provenance_test.go` (new)
**Dependencies**: 152.005-T
**Effort**: ~45 min

#### 152.007-T — GREEN implementation: CorrectStashProvenance core + rehydration + merge-sync

**P-002 Contract**: Implementation that makes 152.006-T tests pass.

Algorithm:
1. Validate request (non-empty stash_id, canonical_delivery_artifact_id, reason, actor)
2. Acquire cross-process stash-file lock (serialize concurrent corrections)
3. Resolve `provenance_corrections.jsonl` path via `WorkspaceStorageRoot(ws.RootPath)` (not hardcoded `.backlogit`)
4. Read stash archive (resolved via `WorkspaceStorageRoot`), find entry for stash_id
5. Find canonical delivery artifact, verify its `source_stash_id` matches stash_id
6. Read existing `provenance_corrections.jsonl` — check for existing correction:
   - Same stash_id + same canonical_delivery → NoOp
   - Same stash_id + different canonical_delivery → ConflictingCorrection error
7. Append `ProvenanceCorrectionRecord` to tracked `provenance_corrections.jsonl`
8. Also append `stash_provenance_correction` event to item log (non-durable supplement)
9. Return result

Rehydration integration (`internal/db/rehydration.go`):
- During `stash_links` population, read `provenance_corrections.jsonl` from workspace archive dir
- If a correction exists for a stash_id, use `canonical_delivery_artifact_id` as the authoritative `stash_links.item_id` instead of artifact iteration order

Merge-sync integration:
- `internal/db/manifest.go`: classify `provenance_corrections.jsonl` appropriately (new `FileKindProvenanceCorrection` or extend `FileKindStash`)
- `internal/db/merge_sync.go`: refresh stash projections when provenance corrections file changes

**Files**: `internal/core/stash_provenance.go` (modify from stubs), `internal/db/rehydration.go` (modify), `internal/db/manifest.go` (modify), `internal/db/merge_sync.go` (modify)
**Dependencies**: 152.006-T
**Effort**: ~2 hr

#### 152.008-T — CLI stash correct and MCP backlogit_correct_stash_provenance surfaces

CLI: `backlogit stash correct --stash-id <id> --canonical-delivery <artifact_id> --reason <reason> --actor <actor>`
MCP: `backlogit_correct_stash_provenance` tool with params: `stash_id` (required), `canonical_delivery_artifact_id` (required), `reason` (required), `actor` (required)

Includes surface tests (RED+GREEN combined):
- CLI: `TestStashCorrectCLI_HappyPath`, `TestStashCorrectCLI_MissingArgs`
- MCP: `TestHandleCorrectStashProvenance_HappyPath`, `TestHandleCorrectStashProvenance_ValidationError`

**Files**: `internal/cli/stash_correct.go`, `internal/cli/stash_correct_test.go`, `internal/mcp/tools_stash_correct.go`, `internal/mcp/tools_stash_correct_test.go` (all new), registration updates
**Dependencies**: 152.007-T
**Effort**: ~45 min

### Wave 3: Integration Tests (152.009-T)

#### 152.009-T — Integration test: end-to-end reconciliation and provenance correction

Tests in `tests/reconcile_integration_test.go`:
- `TestReconcileArchivedLifecycle_Integration`: create item → archive from active → reconcile → verify archived_status: done, custom_fields has reconciliation metadata
- `TestReconcileArchivedLifecycle_Integration_MoveFailsRollback`: archive from active → inject MoveItem failure → verify item restored to archive with original archived_status
- `TestReconcileArchivedLifecycle_Integration_ArchiveFailsForwardRecovery`: verify item left at done in queue when re-archive fails
- `TestCorrectStashProvenance_Integration`: create stash → harvest → archive with wrong pointer → correct → verify provenance_corrections.jsonl has entry
- `TestCorrectStashProvenance_Integration_RehydrationResolves`: after correction + sync, verify stash_links resolves canonical delivery
- `TestCorrectStashProvenance_Integration_MergeSyncResolves`: after correction + merge-sync, verify stash_links updated
- `TestReconcile_CrossWorkspaceContainment`: verify path traversal rejected

**Files**: `tests/reconcile_integration_test.go` (new)
**Dependencies**: 152.004-T, 152.008-T
**Effort**: ~1 hr

## Security and Safety Analysis

### Attack Surface
- **Path traversal**: Item IDs validated against format; paths resolved within workspace root via `WorkspaceStorageRoot`
- **Injection**: All inputs validated; parameterized SQLite queries
- **State corruption**: Fail-closed; rollback-to-archive on MoveItem failure; single lock held for full sequence; stash correction serialized by cross-process lock
- **History falsification**: Original archived_status and harvested_artifact_id are NEVER mutated; corrections are additive only

### Safety Properties
- **Idempotency**: NoOp for already-reconciled items; NoOp for duplicate corrections; conflicting corrections rejected
- **Event durability**: Reconciliation metadata in tracked custom_fields; corrections in tracked provenance_corrections.jsonl
- **Atomicity**: Single lock acquisition for lifecycle reconciliation; cross-process lock for provenance correction
- **No cascade**: Re-archive uses WithCascade(false) to avoid double-archiving children

### Failure Semantics (Explicit)
- **MoveItem failure**: Item is rolled back to archive with original archived_status. Recovery: re-run reconcile.
- **ArchiveItem failure**: Item is left at target_status (done) in queue. This is a defined forward-recovery state, NOT a claim of full rollback. Recovery: operator archives manually or re-runs reconcile (which will detect non-archived item and return error).
- **Provenance correction file write failure**: No correction recorded; operation returns error. Recovery: retry.

### Post-Application Irreversibility
After the capability is applied to 150.001-T/150.002-T:
- Lifecycle reconciliation is practically irreversible: `UnarchiveItem` would restore to `done` (the new archived_status), not `active` (the original). This is correct behavior — the reconciliation IS the correction.
- Provenance correction is append-only and conflicting corrections are rejected. Supersession requires a new mechanism not in scope.
- Reverting the feature code does NOT undo persisted repair records. The application step is an explicit operator-approved checkpoint.

### Rollback (Feature Code Only)
- Feature is purely additive (new files, new commands, rehydration/merge-sync enhancements)
- Reverting the feature branch removes all new capability with zero impact on existing functionality
- Reverting BEFORE application has no data impact

## Plan Hardening

### Protected Invariants
1. **UnarchiveItem restore semantics**: Never bypassed; reconciliation uses the standard unarchive → transition → archive path
2. **archived_status field accuracy**: After reconciliation, archived_status reflects the actual pre-archive status (done), not a fabricated one
3. **Stash archive immutability**: harvested_artifact_id in stash.jsonl is never mutated; corrections are additive via separate tracked file
4. **History integrity**: docs/closure/2026-08-29-133-s-lifecycle-incident.md is never modified; original archived_status: active is preserved in event logs

### ProposedAction / ActionRisk

| Action | Risk | Approval |
|--------|------|----------|
| Add ReconcileArchivedLifecycle operation | moderate | Standard review |
| Add CorrectStashProvenance operation | moderate | Standard review |
| Modify rehydration.go for correction resolution | moderate | Standard review + integration test |
| Modify merge_sync.go for correction propagation | moderate | Standard review + integration test |
| Apply reconciliation to 150.001-T/150.002-T (post-merge) | high | Explicit operator checkpoint in closure PR |
| Apply provenance correction to 11FFF601 (post-merge) | high | Explicit operator checkpoint in closure PR |

### Recovery Behavior
- MoveItem failure: rollback to archive with original archived_status
- ArchiveItem failure: forward-recovery (item at done in queue)
- Provenance file write failure: no state change, retry safe
- Full feature revert: safe before application; after application, data corrections persist

## Plan Review

**dispatch_mode**: adversarial (3-model parallel, independent)
**decision**: PASS with remediations — all HIGH-confidence findings remediated, MEDIUM addressed, LOW dispositioned. See `docs/decisions/2026-08-29-152-adversarial-review.md`.

## Release Observability

- **SLIs**: Event log entries for `lifecycle_reconciliation` and `stash_provenance_correction`; custom_fields `reconciled_at` timestamp; `provenance_corrections.jsonl` entry count
- **Monitoring**: Verify reconciled items have correct `archived_status` after application via `backlogit query`
- **Rollback trigger**: If any reconciled item has incorrect `archived_status`, revert feature (before application only)
- **Post-deploy validation**: Apply to 150.001-T/150.002-T in dedicated closure PR after capability merges; verify with `backlogit query`

## Ship Sequence (Post-Stage)

1. Ship claims 134-S
2. Wave 1 tasks (152.001-T → 152.004-T) executed with P-002 declaration → RED → GREEN discipline
3. Wave 2 tasks (152.005-T → 152.008-T) executed with P-002 declaration → RED → GREEN discipline
4. Wave 3 (152.009-T) integration tests
5. Quality gates: `go test ./...`, `go vet ./...`, `golangci-lint run`, `gofmt -l .`
6. Review skill + Copilot review
7. PR to main, CI green, merge commit
8. **Post-merge application PR** (separate closure branch, explicit operator checkpoint):
   a. `backlogit reconcile 150.001-T 150.002-T --reason "P-001 lifecycle reconciliation: tasks archived from active status without done transition. Evidence: closure/2026-08-29-133-s-lifecycle-incident.md" --actor "ship-agent" --target-status done`
   b. `backlogit stash correct --stash-id 11FFF601 --canonical-delivery 150-F --reason "Stash auto-harvested as 151-F (archived from queued, unused) but actual delivery was 150-F/133-S" --actor "ship-agent"`
   c. `backlogit sync` to refresh index
   d. Verify: `backlogit query "SELECT id, json_extract(custom_fields, '$.reconciled_at') FROM items WHERE id IN ('150.001-T','150.002-T')"`
   e. Commit, push, Copilot review, CI, merge
9. 133-S reconciliation/closure update referencing this repair
