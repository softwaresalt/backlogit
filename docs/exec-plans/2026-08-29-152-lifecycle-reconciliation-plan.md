---
chunk_strategy: h1-h2-h3
description: "Execution plan for 152-F: governed lifecycle reconciliation and stash provenance correction (post-adversarial-review revision)"
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-08-29-152-lifecycle-reconciliation-plan.md
title: "152-F Execution Plan — Governed Lifecycle Reconciliation and Stash Provenance Correction"
---

# 152-F Execution Plan (Revised — Post-Adversarial Review)

**Feature**: 152-F — Governed Lifecycle Reconciliation and Stash Provenance Correction
**Deliberation**: `docs/decisions/2026-08-29-152-lifecycle-reconciliation-deliberation.md`
**Adversarial Review**: `docs/decisions/2026-08-29-152-adversarial-review.md`
**Shipment**: 134-S

## Constitution Check

| Principle | Compliance |
|-----------|-----------|
| I. Safety-First Go | All new code is Go 1.24.0; errors wrapped with context |
| II. Test-First (P-002) | Each task has RED commit before GREEN; FC-1..FC-3 enforced |
| III. Workspace Isolation | All paths resolve within workspace root; validated |
| IV. CLI Containment | No files outside cwd tree |
| V. Observability | Durable events in tracked custom_fields + tracked provenance_corrections.jsonl |
| VI. Single Responsibility | Uses existing primitives; minimal new surface |
| VII. Destructive Approval | No destructive operations |
| VIII. Safety Modes | Fail-closed design; invalid IDs rejected; rollback on partial failure |
| IX. Git-Friendly | Markdown artifacts with YAML frontmatter |
| X. Context Efficiency | Structured query results |
| XI. Merge Commits | P-009 enforced |

## Adversarial Review Remediations Incorporated

- **HC-1**: Provenance correction writes to tracked `provenance_corrections.jsonl` in `.backlogit/archive/`; rehydration resolves canonical delivery; 151-F frontmatter preserved (no history falsification)
- **HC-2**: Single `lockArtifactMutations` held for full sequence; rollback-to-archive on MoveItem failure; ArchiveItem gated on verified done status
- **HC-3**: Precondition check — if `archived_status == target_status`, return NoOp immediately
- **MC-1**: Pre-check children status before MoveItem
- **MC-2**: `WithCascade(false)` on re-archive step
- **MC-3**: Reconciliation metadata recorded in artifact `custom_fields` (tracked, durable)
- **LC-2**: Validate restored status → target_status path exists in DefaultTransitions

## Task Decomposition

### Wave 1: Core Lifecycle Reconciliation (152.001-T through 152.004-T)

#### 152.001-T — RED harness: ReconcileArchivedLifecycle unit tests

**P-002 Contract**: Separate RED commit with compiling tests that FAIL against current code.

Tests (table-driven with `t.Run` in `internal/core/archive_reconcile_test.go`):
- `TestReconcileArchivedLifecycle_HappyPath`: archived item with `archived_status: active` → reconciled to `done`, custom_fields has reconciliation metadata, result is `Completed`
- `TestReconcileArchivedLifecycle_AlreadyDone_NoOp`: archived item with `archived_status: done` → `NoOp` result (HC-3 fix)
- `TestReconcileArchivedLifecycle_NotArchived_Error`: item in `active` status → error returned
- `TestReconcileArchivedLifecycle_NotFound_Error`: nonexistent ID → error returned
- `TestReconcileArchivedLifecycle_EmptyReason_Error`: empty reason → validation error
- `TestReconcileArchivedLifecycle_EmptyActor_Error`: empty actor → validation error
- `TestReconcileArchivedLifecycle_IdempotencyKey_Repeat`: same key repeated → `NoOp`
- `TestReconcileArchivedLifecycle_MultipleItems`: batch of 2 items → both reconciled
- `TestReconcileArchivedLifecycle_PartialFailure_Rollback`: one valid, one with MoveItem failure → valid item rolls back to archive, partial result (HC-2 fix)
- `TestReconcileArchivedLifecycle_PathTraversal_Rejected`: ID with path traversal → rejected
- `TestReconcileArchivedLifecycle_EventDurability`: reconciliation metadata in custom_fields is durable (MC-3 fix)
- `TestReconcileArchivedLifecycle_NoCascadeOnReArchive`: re-archive uses WithCascade(false) (MC-2 fix)
- `TestReconcileArchivedLifecycle_InvalidTransitionPath_Error`: archived_status: queued, target: done — queued→done not in transitions → error (LC-2 fix)
- `TestReconcileArchivedLifecycle_MoveItemFails_RollbackToArchive`: MoveItem error → item re-archived with original archived_status (HC-2 fix)

**Files**: `internal/core/archive_reconcile_test.go` (new)
**Dependencies**: none
**Effort**: ~1 hr

#### 152.002-T — GREEN implementation: ReconcileArchivedLifecycle core

**P-002 Contract**: Implementation that makes 152.001-T tests pass.

Implementation in `internal/core/archive_reconcile.go` (new file):

```go
// ReconciliationRequest defines the input for ReconcileArchivedLifecycle.
type ReconciliationRequest struct {
    ItemIDs        []string // explicit archived item IDs
    TargetStatus   string   // target status (default: "done")
    Reason         string   // non-empty reason for reconciliation
    Actor          string   // non-empty operator/actor identifier
    IdempotencyKey string   // optional repeat detection key
}

// ReconciliationItemResult records the outcome per item.
type ReconciliationItemResult struct {
    ID      string // item ID
    Outcome string // "completed", "no_op", "error"
    Error   string // error detail if Outcome == "error"
}

// ReconciliationResult is the aggregate outcome.
type ReconciliationResult struct {
    Items   []ReconciliationItemResult
    Outcome string // "completed", "no_op", "partial", "indeterminate"
}
```

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
   g. `MoveItem` to target_status — if fails, rollback: re-archive with original archived_status, record error (HC-2)
   h. Verify status == target_status before proceeding
   i. `ArchiveItem` with `WithCascade(false)` (MC-2) — stores `archived_status: done`
   j. Update custom_fields with reconciliation metadata: `reconciled_at`, `reconciled_by`, `reconciled_reason` (MC-3)
   k. Append `lifecycle_reconciliation` event to item log
4. Return structured result

**Files**: `internal/core/archive_reconcile.go` (new)
**Dependencies**: 152.001-T
**Effort**: ~1.5 hr

#### 152.003-T — RED harness: CLI and MCP reconcile surfaces

**P-002 Contract**: Separate RED commit with compiling tests that FAIL.

Tests:
- CLI: `TestReconcileCLI_HappyPath`, `TestReconcileCLI_MissingArgs`, `TestReconcileCLI_InvalidID`
- MCP: `TestHandleReconcileArchivedLifecycle_HappyPath`, `TestHandleReconcileArchivedLifecycle_ValidationError`

**Files**: `internal/cli/reconcile_test.go` (new), `internal/mcp/tools_reconcile_test.go` (new)
**Dependencies**: 152.002-T
**Effort**: ~30 min

#### 152.004-T — GREEN implementation: CLI reconcile and MCP backlogit_reconcile_archived_lifecycle

CLI: `backlogit reconcile <id1> [id2...] --reason <reason> --actor <actor> [--target-status done] [--idempotency-key <key>]`
MCP: `backlogit_reconcile_archived_lifecycle` tool with params: `item_ids` (required array), `reason` (required), `actor` (required), `target_status` (optional, default "done"), `idempotency_key` (optional)

**Files**: `internal/cli/reconcile.go` (new), `internal/mcp/tools_reconcile.go` (new), registration in `internal/cli/root.go` and `internal/mcp/tools.go`
**Dependencies**: 152.003-T
**Effort**: ~1 hr

### Wave 2: Stash Provenance Correction (152.005-T through 152.008-T)

#### 152.005-T — RED harness: CorrectStashProvenance unit tests

**P-002 Contract**: Separate RED commit with compiling tests that FAIL.

Tests (table-driven in `internal/core/stash_provenance_test.go`):
- `TestCorrectStashProvenance_HappyPath`: correction recorded in tracked provenance_corrections.jsonl, original harvested_artifact_id preserved
- `TestCorrectStashProvenance_AlreadyCorrected_NoOp`: same correction repeated → `NoOp`
- `TestCorrectStashProvenance_ConflictingCorrection_Error`: different canonical ID for same stash → rejected
- `TestCorrectStashProvenance_StashNotFound_Error`: nonexistent stash ID → error
- `TestCorrectStashProvenance_ArtifactNotFound_Error`: nonexistent artifact ID → error
- `TestCorrectStashProvenance_SourceStashMismatch_Error`: artifact's source_stash_id doesn't match → error
- `TestCorrectStashProvenance_EmptyReason_Error`: empty reason → validation error
- `TestCorrectStashProvenance_EmptyActor_Error`: empty actor → validation error
- `TestCorrectStashProvenance_EventDurability`: correction event written to TRACKED file (HC-1 fix)
- `TestCorrectStashProvenance_RehydrationResolvesCanonical`: after correction + sync, stash_links.item_id points to canonical delivery (HC-1 fix)

**Files**: `internal/core/stash_provenance_test.go` (new)
**Dependencies**: none (can parallel Wave 1)
**Effort**: ~45 min

#### 152.006-T — GREEN implementation: CorrectStashProvenance core

Implementation in `internal/core/stash_provenance.go` (new file):

Data structures:
```go
// StashProvenanceCorrectionRequest defines the input for CorrectStashProvenance.
type StashProvenanceCorrectionRequest struct {
    StashID                      string
    CanonicalDeliveryArtifactID  string
    Reason                       string
    Actor                        string
}

// StashProvenanceCorrectionResult records the outcome.
type StashProvenanceCorrectionResult struct {
    StashID              string
    OriginalHarvestID    string
    CanonicalDeliveryID  string
    Outcome              string // "corrected", "no_op", "error"
}

// ProvenanceCorrectionRecord is one entry in provenance_corrections.jsonl.
type ProvenanceCorrectionRecord struct {
    StashID                     string    `json:"stash_id"`
    OriginalHarvestedArtifactID string    `json:"original_harvested_artifact_id"`
    CanonicalDeliveryArtifactID string    `json:"canonical_delivery_artifact_id"`
    Reason                      string    `json:"reason"`
    Actor                       string    `json:"actor"`
    CorrectedAt                 time.Time `json:"corrected_at"`
}
```

Algorithm:
1. Validate request (non-empty stash_id, canonical_delivery_artifact_id, reason, actor)
2. Read `.backlogit/archive/stash.jsonl`, find entry for stash_id
3. Find canonical delivery artifact, verify its `source_stash_id` matches
4. Read existing `provenance_corrections.jsonl` — check for existing correction:
   - Same stash_id + same canonical_delivery → NoOp
   - Same stash_id + different canonical_delivery → ConflictingCorrection error
5. Append `ProvenanceCorrectionRecord` to `.backlogit/archive/provenance_corrections.jsonl` (TRACKED)
6. Also append `stash_provenance_correction` event to item log (non-durable supplement)
7. Return result

Rehydration integration:
- In `internal/db/rehydration.go`, during `stash_links` population, check `provenance_corrections.jsonl` for corrections. If a correction exists for a stash_id, use `canonical_delivery_artifact_id` instead of the artifact iteration order.

**Files**: `internal/core/stash_provenance.go` (new), `internal/db/rehydration.go` (modification)
**Dependencies**: 152.005-T
**Effort**: ~1.5 hr

#### 152.007-T — RED harness: CLI and MCP stash provenance surface tests

**P-002 Contract**: Separate RED commit with compiling tests that FAIL.

Tests:
- CLI: `TestStashCorrectCLI_HappyPath`, `TestStashCorrectCLI_MissingArgs`
- MCP: `TestHandleCorrectStashProvenance_HappyPath`, `TestHandleCorrectStashProvenance_ValidationError`

**Files**: `internal/cli/stash_correct_test.go` (new), `internal/mcp/tools_stash_correct_test.go` (new)
**Dependencies**: 152.006-T
**Effort**: ~30 min

#### 152.008-T — GREEN implementation: CLI stash correct and MCP backlogit_correct_stash_provenance

CLI: `backlogit stash correct --stash-id <id> --canonical-delivery <artifact_id> --reason <reason> --actor <actor>`
MCP: `backlogit_correct_stash_provenance` tool with params: `stash_id` (required), `canonical_delivery_artifact_id` (required), `reason` (required), `actor` (required)

**Files**: `internal/cli/stash_correct.go` (new), `internal/mcp/tools_stash_correct.go` (new), registration updates
**Dependencies**: 152.007-T
**Effort**: ~45 min

### Wave 3: Integration Tests (152.009-T)

#### 152.009-T — Integration test: end-to-end reconciliation and provenance correction

Tests in `tests/reconcile_integration_test.go`:
- `TestReconcileArchivedLifecycle_Integration`: create item → archive from active → reconcile → verify archived_status: done, custom_fields has reconciliation metadata, event trail
- `TestReconcileArchivedLifecycle_Integration_MoveFailsRollback`: archive from active → inject MoveItem failure → verify item restored to archive with original archived_status
- `TestCorrectStashProvenance_Integration`: create stash → harvest → archive with wrong pointer → correct → verify provenance_corrections.jsonl has entry
- `TestCorrectStashProvenance_Integration_RehydrationResolves`: after correction + sync, verify stash_links resolves canonical delivery
- `TestReconcile_CrossWorkspaceContainment`: verify path traversal rejected

**Files**: `tests/reconcile_integration_test.go` (new)
**Dependencies**: 152.004-T, 152.008-T
**Effort**: ~1 hr

## Security and Safety Analysis

### Attack Surface
- **Path traversal**: Item IDs validated against format; paths resolved within workspace root
- **Injection**: All inputs validated; parameterized SQLite queries
- **State corruption**: Fail-closed; rollback-to-archive on partial failure; single lock held for full sequence
- **History falsification**: Original archived_status and harvested_artifact_id are NEVER mutated; corrections are additive only

### Safety Properties
- **Idempotency**: NoOp for already-reconciled items; NoOp for duplicate corrections
- **Event durability**: Reconciliation metadata in tracked custom_fields; corrections in tracked provenance_corrections.jsonl
- **Atomicity**: Single lock acquisition; rollback on failure; no partial-done state
- **No cascade**: Re-archive uses WithCascade(false) to avoid double-archiving children

### Rollback
- Feature is purely additive (new files, new commands, one rehydration enhancement)
- Reverting the feature branch removes all new capability with zero impact on existing functionality
- If applied to 150.001-T/150.002-T and result is wrong, UnarchiveItem can restore them

## Release Observability

- **SLIs**: Event log entries for `lifecycle_reconciliation` and `stash_provenance_correction`; custom_fields `reconciled_at` timestamp
- **Monitoring**: Verify reconciled items have correct `archived_status` after application
- **Rollback trigger**: If any reconciled item has incorrect `archived_status`, revert
- **Post-deploy validation**: Apply to 150.001-T/150.002-T in dedicated closure PR after capability merges; verify with `backlogit query`

## Ship Sequence (Post-Stage)

1. Ship claims 134-S
2. Wave 1 tasks (152.001-T → 152.004-T) executed with P-002 RED/GREEN discipline
3. Wave 2 tasks (152.005-T → 152.008-T) executed with P-002 RED/GREEN discipline
4. Wave 3 (152.009-T) integration tests
5. Quality gates: `go test ./...`, `go vet ./...`, `golangci-lint run`, `gofmt -l .`
6. Review skill + Copilot review
7. PR to main, CI green, merge commit
8. **Post-merge application PR** (separate closure branch):
   a. `backlogit reconcile 150.001-T 150.002-T --reason "P-001 lifecycle reconciliation: tasks archived from active status without done transition. Evidence: closure/2026-08-29-133-s-lifecycle-incident.md" --actor "ship-agent" --target-status done`
   b. `backlogit stash correct --stash-id 11FFF601 --canonical-delivery 150-F --reason "Stash auto-harvested as 151-F (archived from queued, unused) but actual delivery was 150-F/133-S" --actor "ship-agent"`
   c. `backlogit sync` to refresh index
   d. Verify: `backlogit query "SELECT id, json_extract(custom_fields, '$.reconciled_at') FROM items WHERE id IN ('150.001-T','150.002-T')"`
   e. Commit, push, Copilot review, CI, merge
9. 133-S reconciliation/closure update referencing this repair
