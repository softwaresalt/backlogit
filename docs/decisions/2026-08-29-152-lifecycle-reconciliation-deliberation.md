---
chunk_strategy: h1-h2-h3
description: "Deliberation: governed lifecycle reconciliation and stash provenance correction for 150.001-T, 150.002-T, and 11FFF601"
doc_type: decision
schema_version: "1.0"
source: docs/decisions/2026-08-29-152-lifecycle-reconciliation-deliberation.md
title: "152-F Deliberation — Governed Lifecycle Reconciliation and Stash Provenance Correction"
---

# 152-F Deliberation — Governed Lifecycle Reconciliation and Stash Provenance Correction

**Date**: 2026-08-29
**Status**: Decided
**Trigger**: P-001 lifecycle violation in 150.001-T and 150.002-T (archived from active, skipping done); stash 11FFF601 harvested_artifact_id points to 151-F but actual delivery was 150-F/133-S.

## Problem Statement

Two related but distinct integrity gaps exist in the 11FFF601/150-F/133-S release unit:

### Gap 1: P-001 Lifecycle Violation (150.001-T, 150.002-T)

Items were archived directly from `active` status. P-001 requires `active → done → archived`. The `archived_status: active` field is functionally used by `UnarchiveItem` to restore items, so direct frontmatter editing would corrupt restore semantics. No CLI or MCP command exists to unarchive or reconcile archived items.

### Gap 2: Stash Provenance Mismatch (11FFF601)

Stash entry 11FFF601 was archived with `harvested_artifact_id: 151-F` (the auto-harvest artifact), but the actual delivery was through manually-created 150-F/133-S. Both 150-F and 151-F carry `source_stash_id: 11FFF601`. 151-F was archived from `queued` (never started). No mechanism exists to record provenance corrections or canonical delivery pointers.

## Decision

### Capability 1: ReconcileArchivedLifecycle

Add a governed `ReconcileArchivedLifecycle` operation to `internal/core/archive.go` that:

1. **Accepts**: explicit archived item IDs (slice), non-empty reason string, non-empty operator/actor string
2. **Validates**: each ID exists, is currently `archived`, has `archived_status` != target status (not already reconciled)
3. **Executes atomically per-item**: unarchive → transition to `done` → re-archive, using existing `UnarchiveItem`, `MoveItem`/`UpdateArtifact`, and `ArchiveItem` primitives
4. **Records**: `lifecycle_reconciliation` event with real timestamps, original `archived_status`, target status, reason, actor, and idempotency key
5. **Returns**: structured `ReconciliationResult` with `Completed`, `NoOp`, `Partial`, `Indeterminate` outcome per item
6. **Idempotency**: items already at target `archived_status` return `NoOp`; repeat calls with same key are safe

This reuses existing governed primitives rather than inventing a new state machine path. The sequence `unarchive → done → archive` creates genuine lifecycle events at each step.

### Capability 2: StashProvenanceCorrection

Add a governed `CorrectStashProvenance` operation that:

1. **Accepts**: stash ID, canonical delivery artifact ID, correction reason, operator/actor
2. **Validates**: stash archive entry exists, canonical delivery artifact exists, source_stash_id matches, artifact class consistency
3. **Records**: `stash_provenance_correction` event preserving both the original `harvested_artifact_id` pointer and the corrected canonical delivery pointer
4. **Idempotency**: same correction repeated is a no-op; conflicting correction (different canonical ID) is rejected
5. **Never mutates**: the archived stash entry or the original `harvested_artifact_id` — correction is additive only

### Surface Parity

Both capabilities MUST have CLI and MCP tool surfaces with contract parity, consistent with the governed-operation-parity design doc.

## Alternatives Considered

1. **Direct frontmatter editing**: Rejected — corrupts `UnarchiveItem` restore semantics and falsifies history.
2. **Adding `archived` to transition map**: Rejected — opens a dangerous general path out of terminal state.
3. **Accept the gap permanently**: Rejected — operator explicitly authorized the causal repair.
4. **New `unarchive` CLI only**: Insufficient — does not handle the atomic reconcile-and-rearchive sequence or the stash provenance correction.

## Scope Guard

This feature is strictly limited to:
- ReconcileArchivedLifecycle operation + CLI + MCP
- CorrectStashProvenance operation + CLI + MCP
- Tests for both capabilities
- NO stash harvesting (11FFF601 is already archived)
- NO modifications to existing lifecycle transitions
- NO changes to ArchiveItem or UnarchiveItem behavior
- Application of these capabilities to 150.001-T/150.002-T/11FFF601 happens AFTER the capability is shipped to main

## Release Observability

- **Rollback trigger**: If reconciliation produces incorrect `archived_status` values in any test, revert the feature branch
- **Monitoring**: Event log entries for `lifecycle_reconciliation` and `stash_provenance_correction` are the primary signals
- **Post-deploy validation**: Apply to 150.001-T/150.002-T in a dedicated closure PR after capability merges
