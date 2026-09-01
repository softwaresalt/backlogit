---
chunk_strategy: h1-h2-h3
description: "Adversarial plan review consensus for 152-F lifecycle reconciliation"
doc_type: decision
schema_version: "1.0"
source: docs/decisions/2026-08-29-152-adversarial-review.md
title: "152-F Adversarial Plan Review — Consensus Report"
---

# 152-F Adversarial Plan Review — Consensus Report

**Date**: 2026-08-29
**Reviewers**: 3 independent model instances (Tier 1: claude-sonnet-5, Tier 2: gpt-5.6-terra, Tier 3: claude-opus-4.8)
**Method**: Parallel independent review, no cross-contamination

## Consensus Findings

### HIGH Confidence (All 3 reviewers agree) — GATE-BLOCKING

#### HC-1: Stash provenance correction is non-durable and non-effective

**Severity**: P0/P1 (consensus P1 — gate-blocking)
**All reviewers identified**: Event-only correction lands in gitignored `.backlogit/logs/` and does not modify any tracked surface. Additionally, `stash_links` DB table is built from artifact `custom_fields.source_stash_id` during rehydration, not from stash.jsonl's `harvested_artifact_id`. Both 150-F and 151-F carry `source_stash_id: 11FFF601`, creating a PK collision in `stash_links`.

**Remediation**: The correction must:
1. Write to a tracked, durable surface (not gitignored logs)
2. Address the `source_stash_id` duplication on 151-F's frontmatter (or record canonical delivery in a queryable way that rehydration respects)
3. Ensure `stash_links` correctly resolves the canonical delivery after sync

**Plan revision**: Add a `provenance_corrections.jsonl` tracked file in `.backlogit/archive/` alongside `stash.jsonl`. Rehydration reads this file to resolve `stash_links` canonical delivery. The correction event is ALSO appended to item logs for non-durable audit trail. The 151-F frontmatter's `source_stash_id` is NOT removed (that would be history falsification) — instead, the correction record takes precedence during rehydration.

#### HC-2: Non-atomic composite operation with unsafe partial failure

**Severity**: P1 (consensus — gate-blocking)
**All reviewers identified**: UnarchiveItem → MoveItem → ArchiveItem each acquire/release locks independently. Partial failure after UnarchiveItem leaves item worse than before (unarchived, active, recovery evidence lost). No rollback or forward-recovery defined.

**Remediation**: 
1. Acquire `lockArtifactMutations` once for all target IDs at the start of `ReconcileArchivedLifecycle`, hold through entire sequence
2. On MoveItem failure: re-archive the item with original archived_status to restore pre-reconcile state
3. On ArchiveItem failure: item is left at `done` in queue (better than `active` — it passed done transition)
4. Never proceed to ArchiveItem unless MoveItem confirmed status=done

#### HC-3: Non-idempotent on retry for already-reconciled items

**Severity**: P1 (consensus — gate-blocking)
**All reviewers identified**: Re-running on an item with `archived_status: done` → UnarchiveItem restores to `done` → MoveItem(done→done) fails because `done→done` is not in DefaultTransitions.

**Remediation**: Add precondition check before sequence: if `archived_status == target_status`, return NoOp immediately without mutation.

### MEDIUM Confidence (2/3 reviewers agree)

#### MC-1: CheckChildrenTerminal interaction

**Severity**: P2
**Reviewers T1, T3**: MoveItem to `done` triggers CheckChildrenTerminal. If 150.001-T/150.002-T have subtasks, this may block. UnarchiveItem does not cascade-restore children.

**Remediation**: Pre-check for children; for this specific case (150.001-T and 150.002-T are tasks with no subtasks), document the precondition. In the general case, validate no non-terminal children exist or provide cascade-unarchive option.

#### MC-2: ArchiveItem re-archive cascade interaction

**Severity**: P2
**Reviewers T1, T3**: Re-archiving may trigger cascade on already-archived children.

**Remediation**: Pass `WithCascade(false)` to ArchiveItem in the reconcile sequence.

#### MC-3: Audit events in gitignored logs insufficient for governance

**Severity**: P2
**Reviewers T2, T3**: Lifecycle reconciliation events go to gitignored logs. Governance trail should be durable.

**Remediation**: Record reconciliation metadata in the artifact frontmatter (e.g., `reconciled_at`, `reconciled_by`, `reconciled_reason` fields in custom_fields). This travels with the committed artifact.

### LOW Confidence (1/3 reviewers)

#### LC-1: Root cause unaddressed (T3 only) — P2

ArchiveItem archives from any status. Consider adding a pre-archive guard.

**Disposition**: Out of scope for this repair feature. Document as follow-up backlog item. The incident record already calls this out as "Tooling Gap #2."

#### LC-2: Hardcoded active→done assumption limits generality (T1 only) — P2

**Disposition**: Accept. The plan already specifies `target_status` parameter; the implementation should validate the restored status can reach target_status via DefaultTransitions.

#### LC-3: Event schema not specified (T1 only) — P3

**Disposition**: Accept advisory. Specify event types: `lifecycle_reconciliation` and `stash_provenance_correction`.

## Gate Decision

**BLOCKED** until HC-1, HC-2, HC-3 remediations are incorporated into the plan.

## Revised Plan Decisions

All three HIGH-confidence findings have clear remediations. Incorporating:

1. **HC-1 fix**: Add `provenance_corrections.jsonl` tracked surface; rehydration resolves canonical delivery; 151-F frontmatter preserved
2. **HC-2 fix**: Single lock acquisition for full sequence; rollback-to-archive on MoveItem failure; gate ArchiveItem on verified done status
3. **HC-3 fix**: Precondition check for already-reconciled items returns NoOp
4. **MC-1 fix**: Pre-check children status; pass WithCascade(false) to final ArchiveItem
5. **MC-2 fix**: Explicit WithCascade(false) on re-archive
6. **MC-3 fix**: Record reconciliation metadata in artifact custom_fields (tracked)
7. **LC-2 fix**: Validate restored status can reach target via transitions

After incorporating these remediations, the gate passes.
