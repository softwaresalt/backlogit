---
chunk_strategy: h1-h2-h3
description: "Execution plan for 152-F: governed lifecycle reconciliation and stash provenance correction (final revision)"
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-08-29-152-lifecycle-reconciliation-plan.md
title: "152-F Execution Plan — Governed Lifecycle Reconciliation and Stash Provenance Correction"
---

# 152-F Execution Plan (Final — Post-Adversarial + Post-Copilot Review)

**Feature**: 152-F — Governed Lifecycle Reconciliation and Stash Provenance Correction
**Deliberation**: `docs/decisions/2026-08-29-152-lifecycle-reconciliation-deliberation.md`
**Adversarial Review**: `docs/decisions/2026-08-29-152-adversarial-review.md`
**Shipment**: 134-S (11 tasks, 3 waves)

## Constitution Check

| Principle | Compliance |
|-----------|-----------|
| I. Safety-First Go | All new code is Go 1.24.0; errors wrapped with context |
| II. Test-First (P-002) | Each wave: declaration, RED harness, GREEN impl; surfaces split RED/GREEN; integration verification-only with exemption |
| III. Workspace Isolation | All paths resolve within workspace root via WorkspaceStorageRoot; validated |
| IV. CLI Containment | No files outside cwd tree |
| V. Observability | Durable events in tracked custom_fields + tracked provenance_corrections.jsonl |
| VI. Single Responsibility | Uses existing primitives; minimal new surface |
| VII. Destructive Approval | No destructive operations |
| VIII. Safety Modes | Fail-closed design; invalid IDs rejected; rollback on partial failure; ErrWriteIndeterminate honored |
| IX. Git-Friendly | Markdown artifacts with YAML frontmatter |
| X. Context Efficiency | Structured query results |
| XI. Merge Commits | P-009 enforced |

Constitution Check: pass

## Adversarial Review Remediations

- **HC-1**: Provenance correction writes to tracked provenance_corrections.jsonl; rehydration AND merge-sync resolve canonical delivery
- **HC-2**: Single lockArtifactMutations held for full sequence; rollback-to-archive on MoveItem failure
- **HC-3**: Precondition check: if archived_status == target_status, return NoOp
- **MC-1**: Pre-check children status before MoveItem
- **MC-2**: WithCascade(false) on re-archive
- **MC-3**: Reconciliation metadata in custom_fields written atomically with re-archive
- **LC-2**: Validate restored status -> target_status path exists in DefaultTransitions

## Copilot Review Remediations

- **Source stash ID**: 152-F does not carry source_stash_id to avoid stash_links PK collision
- **Surface task splits**: CLI/MCP surfaces split into separate RED harness and GREEN implementation tasks
- **Integration exemption**: 152.009-T declared verification-only with named harness owner
- **ErrWriteIndeterminate**: Handled at every step of 3-step sequence; never-roll-back-indeterminate invariant
- **Post-application verification**: Expanded to check archived_status, reconciled_at, stash_links resolution, and correction record
- **Artifact class consistency**: Defined rule: kind != artifact_type is valid; validation checks only stash existence, artifact existence, and source_stash_id match
- **WorkspaceStorageRoot**: All archive paths resolved via WorkspaceStorageRoot, not hardcoded
- **Merge-sync**: provenance_corrections.jsonl classified in manifest; merge-sync refreshes stash projections

## Artifact Class Consistency Rule

Stash kind (feature/task/bug/epic/unknown) != artifact_type (feature/task/subtask) is valid workflow. A stash kind:task harvested into artifact_type:feature is normal operator scoping. Validation checks only: (1) stash exists, (2) artifact exists, (3) artifact source_stash_id matches stash ID.

## Task Decomposition (11 tasks, 3 waves)

### Wave 1: Lifecycle Reconciliation (152.001-T -> 152.003-T -> 152.004-T -> 152.010-T)

#### 152.001-T — Declarations: ReconcileArchivedLifecycle types and stubs

Create internal/core/archive_reconcile.go with type declarations and function stub returning ErrNotImplemented.

**Files**: internal/core/archive_reconcile.go (new)
**Dependencies**: none
**Effort**: ~15 min

#### 152.002-T — RED harness: ReconcileArchivedLifecycle unit tests

P-002 FC-1: tests compile against stubs, all FAIL (RED). Committed separately.

Tests: HappyPath, AlreadyDone_NoOp, NotArchived_Error, NotFound_Error, EmptyReason_Error, EmptyActor_Error, IdempotencyKey_Repeat, MultipleItems, PartialFailure_Rollback, PathTraversal_Rejected, NoCascadeOnReArchive, InvalidTransitionPath_Error, MoveItemFails_RollbackToArchive, DurableCustomFieldsMetadata, UnarchiveIndeterminate, MoveIndeterminate, ArchiveIndeterminate.

**Files**: internal/core/archive_reconcile_test.go (new)
**Dependencies**: 152.001-T
**Effort**: ~1 hr

#### 152.003-T — GREEN implementation: ReconcileArchivedLifecycle core

Algorithm:
1. Validate request
2. Acquire lockArtifactMutations(ctx, ws, req.ItemIDs) — SINGLE lock
3. Per item: verify archived + archived_status exists; NoOp if already at target; validate transition path; check idempotency; pre-check children; UnarchiveItem; MoveItem (rollback to archive on failure); verify status; ArchiveItem with WithCascade(false) + atomic custom_fields metadata; append event
4. ErrWriteIndeterminate at ANY step: do NOT roll back; record indeterminate
5. Return structured result

**Files**: internal/core/archive_reconcile.go (modify)
**Dependencies**: 152.002-T
**Effort**: ~1.5 hr

#### 152.004-T — RED harness: CLI and MCP reconcile surface tests

P-002 FC-1: surface tests compile and FAIL. Committed separately.

**Files**: internal/cli/reconcile_test.go, internal/mcp/tools_reconcile_test.go (new)
**Dependencies**: 152.003-T
**Effort**: ~30 min

#### 152.010-T — GREEN: CLI reconcile and MCP backlogit_reconcile_archived_lifecycle

Implementation making 152.004-T tests pass.

CLI: backlogit reconcile <ids...> --reason --actor [--target-status done] [--idempotency-key]
MCP: backlogit_reconcile_archived_lifecycle (item_ids, reason, actor, target_status, idempotency_key)

**Files**: internal/cli/reconcile.go, internal/mcp/tools_reconcile.go (new), registration
**Dependencies**: 152.004-T
**Effort**: ~45 min

### Wave 2: Stash Provenance Correction (152.005-T -> 152.007-T -> 152.008-T -> 152.011-T)

#### 152.005-T — Declarations: CorrectStashProvenance types and stubs

Create internal/core/stash_provenance.go with type declarations and stub.

**Files**: internal/core/stash_provenance.go (new)
**Dependencies**: none (parallel with Wave 1)
**Effort**: ~15 min

#### 152.006-T — RED harness: CorrectStashProvenance unit tests

P-002 FC-1: tests compile against stubs, all FAIL. Committed separately.

Tests: HappyPath, AlreadyCorrected_NoOp, ConflictingCorrection_Error, StashNotFound_Error, ArtifactNotFound_Error, SourceStashMismatch_Error, EmptyReason_Error, EmptyActor_Error, EventDurability, RehydrationResolvesCanonical, ConcurrentConflict_Serialized.

**Files**: internal/core/stash_provenance_test.go (new)
**Dependencies**: 152.005-T
**Effort**: ~45 min

#### 152.007-T — GREEN: CorrectStashProvenance core + rehydration + merge-sync

Algorithm: validate, acquire cross-process stash lock, resolve paths via WorkspaceStorageRoot, read stash archive, find artifact + verify source_stash_id, check existing corrections (idempotency/conflict), append to tracked provenance_corrections.jsonl, append event.

Rehydration: read corrections, resolve canonical stash_links.item_id.
Manifest: classify provenance_corrections.jsonl.
Merge-sync: refresh stash projections on correction file changes.

**Files**: internal/core/stash_provenance.go, internal/db/rehydration.go, manifest.go, merge_sync.go
**Dependencies**: 152.006-T
**Effort**: ~2 hr

#### 152.008-T — RED harness: CLI and MCP stash provenance surface tests

P-002 FC-1: surface tests compile and FAIL. Committed separately.

**Files**: internal/cli/stash_correct_test.go, internal/mcp/tools_stash_correct_test.go (new)
**Dependencies**: 152.007-T
**Effort**: ~30 min

#### 152.011-T — GREEN: CLI stash correct and MCP backlogit_correct_stash_provenance

Implementation making 152.008-T tests pass.

CLI: backlogit stash correct --stash-id --canonical-delivery --reason --actor
MCP: backlogit_correct_stash_provenance (stash_id, canonical_delivery_artifact_id, reason, actor)

**Files**: internal/cli/stash_correct.go, internal/mcp/tools_stash_correct.go (new), registration
**Dependencies**: 152.008-T
**Effort**: ~45 min

### Wave 3: Integration (152.009-T)

#### 152.009-T — Integration tests (verification-only, P-002 exemption)

P-002 exemption: verification-only. RED evidence from 152.002-T and 152.006-T.

Tests: end-to-end lifecycle reconciliation, MoveItem failure rollback, ArchiveItem forward-recovery, ErrWriteIndeterminate handling, stash provenance correction, rehydration resolution, merge-sync resolution, cross-workspace containment.

**Files**: tests/reconcile_integration_test.go (new)
**Dependencies**: 152.010-T, 152.011-T
**Effort**: ~1 hr

## Security and Safety Analysis

### Failure Semantics (Explicit)

| Step fails | State | Recovery |
|-----------|-------|---------|
| MoveItem error | Rollback: re-archive with original archived_status | Re-run reconcile |
| ArchiveItem error | Forward-recovery: item at done in queue | Operator archives manually |
| ErrWriteIndeterminate (any step) | Indeterminate: do NOT roll back | Operator inspects and reconciles |
| Provenance file write error | No state change | Retry safe |

### Post-Application Irreversibility

After application to 150.001-T/150.002-T: UnarchiveItem would restore to done (new archived_status), not active. This is correct — reconciliation IS the correction. Provenance corrections are append-only; conflicting corrections rejected. Application is an explicit operator-approved checkpoint. Feature code revert does not undo persisted records.

## Plan Hardening

### Protected Invariants
1. UnarchiveItem restore semantics never bypassed
2. archived_status reflects actual pre-archive status after reconciliation
3. Stash archive harvested_artifact_id never mutated; corrections additive
4. Incident record docs/closure/2026-08-29-133-s-lifecycle-incident.md never modified

### ProposedAction / ActionRisk

| Action | Risk | Approval |
|--------|------|----------|
| Add ReconcileArchivedLifecycle | moderate | Standard review |
| Add CorrectStashProvenance | moderate | Standard review |
| Modify rehydration.go | moderate | Standard review + integration test |
| Modify merge_sync.go | moderate | Standard review + integration test |
| Apply to 150.001-T/150.002-T | high | Explicit operator checkpoint |
| Apply provenance correction | high | Explicit operator checkpoint |

## Plan Review

**dispatch_mode**: adversarial (3-model parallel, independent — claude-sonnet-5, gpt-5.6-terra, claude-opus-4.8)
**decision**: PASS with remediations. All HIGH-confidence findings remediated, MEDIUM addressed, LOW dispositioned.

## Release Observability

- **SLIs**: lifecycle_reconciliation events, stash_provenance_correction events, custom_fields reconciled_at, provenance_corrections.jsonl entries
- **Rollback trigger**: Incorrect archived_status after reconciliation → revert feature (before application only)
- **Post-deploy validation**: Expanded verification in closure PR (see Ship Sequence step 8d-8f)

## Ship Sequence

1. Claim 134-S
2. Wave 1: 152.001-T -> 152.002-T -> 152.003-T -> 152.004-T -> 152.010-T
3. Wave 2: 152.005-T -> 152.006-T -> 152.007-T -> 152.008-T -> 152.011-T
4. Wave 3: 152.009-T
5. Quality gates: go test ./..., go vet ./..., golangci-lint run, gofmt -l .
6. Review + Copilot review
7. PR to main, CI green, merge commit
8. Post-merge application PR (explicit operator checkpoint):
   a. backlogit reconcile 150.001-T 150.002-T --reason "P-001 lifecycle reconciliation" --actor "ship-agent" --target-status done
   b. backlogit stash correct --stash-id 11FFF601 --canonical-delivery 150-F --reason "Stash auto-harvested as 151-F but actual delivery was 150-F/133-S" --actor "ship-agent"
   c. backlogit sync
   d. Verify lifecycle: backlogit query checking archived_status:done and reconciled_at present
   e. Verify provenance: backlogit query checking stash_links resolves 11FFF601 to 150-F
   f. Verify correction record: read provenance_corrections.jsonl
   g. Commit, push, Copilot review, CI, merge
9. 133-S reconciliation/closure update
