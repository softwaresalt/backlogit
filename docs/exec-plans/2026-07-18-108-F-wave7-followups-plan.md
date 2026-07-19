---
chunk_strategy: h1-h2-h3
title: "108-F wave-7 staging review follow-ups — implementation plan"
source: docs/exec-plans/2026-07-18-108-F-wave7-followups-plan.md
doc_type: plan
description: "Implementation plan for three verified follow-up findings (F5/F6/F7) from the 099-S/108-F wave-7 staging review on PR #259."
docline:
    date: 2026-07-18
    status: accepted
    linked_parent_work_item: 108-F
    tags:
        - size-estimation
        - staging
        - review-followup
        - robustness
schema_version: "1.0"
---

# 108-F Wave-7 Follow-ups — Implementation Plan

**Source**: `docs/decisions/2026-07-18-108-F-wave7-staging-review-followups.md` (operator-endorsed decision artifact)
**Covering feature**: 108-F (Size estimation for feature and shipment implementation)
**Stash origin**: B062B90A (kind=feature, priority=medium)

## Objective

Deliver three bounded, ≤2h, width-isolated tasks under 108-F addressing the verified
Copilot wave-7 findings F5, F6, and F7 deferred from PR #259 (099-S staging).

## Constitution Check

- **P-II (Test-First)**: F6 task requires a fail-then-retry test written before the rollback fix. F7 task corrects test scope — removes a test targeting nonexistent behavior.
- **P-I (Safety-First Go)**: F6 changes core error paths; all errors must be wrapped with context.
- **2-Hour Rule**: Each task is <3 files, <5 functions, <4 test scenarios.
- **Width Isolation**: F5 = backlog domain, F6 = core domain, F7 = config/test domain.

## Implementation Units

### IU-1: F5 — Reconcile 108-F backlog artifact for 099-S membership (backlog)

**Scope**: Single file edit to `.backlogit/queue/108-F.md`.

**What to do**:
1. Add an authoritative reconciliation block (similar to existing `<!-- BEGIN:restage-*` and
   `<!-- BEGIN:reconciliation-model-a -->` patterns) recording that 108-F is now a member of
   shipment 099-S, superseding the "no shipment home" and "impl-plan promotion pending"
   statements in the body.
2. Soften the title: remove "impl-plan promotion pending" — replace with a title reflecting
   current state (e.g., "Size estimation for feature and shipment implementation (active;
   member of 099-S)").
3. Keep implementation deferred; retain historical narrative marked as superseded.

**Files**: `.backlogit/queue/108-F.md` (1 file)
**Functions**: 0 (backlog artifact, not source code)
**Test scenarios**: 0 (no code change; artifact consistency verification only)
**Acceptance criteria**:
- 108-F.md title no longer says "impl-plan promotion pending"
- 108-F.md body contains authoritative reconciliation block recording 099-S membership
- Historical narrative is preserved with superseded markers
**Estimated effort**: <30 min
**Execution posture**: backlogit update mutation (NOT a raw file edit). IU-1 changes
both the title (step 2) and the body (step 1), and both fields have first-class update
paths, so route the change through the mutation surface:
`backlogit update 108-F --title "<new title>" --description "<reconciled body>"`
(or the equivalent MCP `backlogit_update_item`). This stamps `updated_at`, persists the
index, and fires a truthful `update_artifact` audit event with
`changed_fields:["title","description"]`. Do not hand-edit `108-F.md` directly — a raw
edit would leave `updated_at` stale and emit no audit event, the exact defect corrected
for 108.009-T during F5 staging review.

### IU-2: F6 — Add rollback/cleanup to CreateArtifact post-create failure boundary (core)

**Scope**: Fix the failure boundary in `core.CreateArtifact` where a post-create size/event
failure orphans an unsized artifact and prevents retry due to ID collision.

**What to do**:
1. **Test-first**: Write a fail-then-retry migration test:
   - Inject a size/event-append failure after CreateArtifact persists the file and index.
   - Assert no orphaned unsized artifact remains in the filesystem or DB index.
   - Assert retry with the same ID succeeds after cleanup.
2. **Implement rollback/cleanup**: On post-create failure, remove the just-created file,
   remove the DB index row, and invalidate the `canonicalCache` record so a retry does not
   collide on `ErrIDCollision` (:338-339).
3. All errors wrapped with context per P-I.

**Prior art (compound learnings)**:
- `atomic-multi-item-claim-rollback-and-stale-blocked-clearing-2026-06-27.md`: Pre-mutation
  snapshot + activated-set rollback pattern from shipment lifecycle.
- `crash-safe-delete-rename-rollback-go-2026-04-23.md`: Rename-back-on-failure pattern for
  crash-safe operations.

**Files**: `internal/core/artifacts.go` (rollback logic), 1 test file (migration test)
**Functions**: ~3 (cleanup helper, modify CreateArtifact error path, test function)
**Test scenarios**: 2-3 (inject size failure → verify cleanup, verify retry succeeds,
optionally verify event-append failure path)
**Acceptance criteria**:
- Fail-then-retry test exists and passes
- Post-create size/event failure does not leave orphaned artifact
- Retry with same ID succeeds after cleanup
- No regression in existing CreateArtifact tests
**Estimated effort**: ~2h
**Execution posture**: test-first (red → green)
**Dependencies**: 108.002-T (099-S) — introduces the post-create size/estimate-event operation whose failure boundary this task hardens; the orphaned-artifact / retry-collision boundary does not exist until 108.002-T lands. Added as an explicit blocking dependency (wave-7 F6 review, PR #260).

**F6/9D5BB492 ride-along decision**: F6 addresses the **in-process error-return** path (size
write fails, function returns error, caller can retry). Stash 9D5BB492 addresses the
**crash-window exactly-once** path (process dies mid-mutation; requires OpID, deterministic
multi-orphan ordering, doctor reconciliation). These are different failure classes. F6's
rollback/cleanup is necessary regardless of whether crash-window exactly-once is ever
implemented. **Decision: keep 9D5BB492 as a separate deferred stash entry.** The F6 fix is
strictly in-process and does not supersede the crash-window requirements.

### IU-3: F7 — Reconcile 108.009-T env-expansion scope (verify-only)

**Scope**: Record and verify the scope correction already applied in place to existing task
108.009-T (SE-7a): the env-var-expansion rejection requirement targeting nonexistent behavior was
dropped. This is a **verify-only reconciliation item** — it carries **no new implementation** and
does not re-implement the containment guard, which is owned by 108.009-T in shipment 099-S.

**What to do** (verify-only — no production code or tests are written under this item):
1. Confirm 108.009-T no longer requires an env-expansion escape test and its description reflects
   the corrected scope.
2. Confirm the lexical `../`/absolute containment guard remains the sole surviving SE-7a
   requirement, covering `QueueLayout.RootDir` (`schema.go:99-104`) and every configured search
   root (`loader.go:77-85`). That guard is implemented by 108.009-T in 099-S, not here.
3. Confirm 108.014-T stands as the traceable reconciliation record (`related_to` 108.009-T) with no
   separate implementation, and that 108.010-T's dependency edge on 108.009-T is unchanged.
4. Do NOT modify `internal/config/*` or add tests under this item.

**Files**: `.backlogit/queue/108.014-T.md` (reconciliation record); `.backlogit/queue/108.009-T.md`
(verify corrected scope — correction already applied). No `internal/config` changes are owned here.
**Functions**: none (verify-only).
**Test scenarios**: none in this item; the SE-7a containment test is owned by 108.009-T (099-S).
**Acceptance criteria**:
- 108.009-T description reflects the corrected scope (env-expansion requirement removed)
- 108.014-T records the reconciliation and links to 108.009-T; no duplicate implementation contract
- 108.010-T dependency on 108.009-T remains intact
**Estimated effort**: ~0.5h (verification / record only)
**Execution posture**: verify-only (no red → green; no new production code or tests in this item)
**Dependencies**: None. **Reconciliation applied (wave-7 review, PR #260)**: the env-var-expansion rejection requirement was corrected **in place** on `108.009-T` (SE-7a, shipment 099-S) during staging — the requirement and its test were dropped; only the lexical `../`/absolute containment guard for `QueueLayout.RootDir` and every configured search root remains. The containment **implementation stays in `108.009-T`** so the SE-7a/SE-7b two-layer containment invariant continues to ship as one unit with `108.010-T` in 099-S. `108.014-T` is therefore the traceable F7 reconciliation / verification record (`related_to` 108.009-T) and carries **no separate implementation** in 100-S. `108.010-T`'s dependency edge is unchanged (still depends on the surviving `108.009-T`).

## Dependency Graph

```
IU-1 (F5, 108.012-T) — independent, no deps
IU-2 (F6, 108.013-T) — depends on 108.002-T (099-S); the failure boundary does not exist until 108.002-T introduces the post-create size/estimate-event operation
IU-3 (F7, 108.014-T) — reconciliation applied in place to 108.009-T at staging (PR #260); no separate implementation. 108.010-T (SE-7b) still depends on the surviving 108.009-T (unchanged).
```

IU-1 is independent. IU-2 must follow 108.002-T (099-S). IU-3 was resolved by an in-place staging correction of 108.009-T (099-S), so it carries no new implementation.

## Requires plan hardening

No. All three tasks are low-to-moderate blast radius, bounded to ≤2h, single-domain,
and the decision document already contains verified code evidence for each.

## Risk Assessment

| Finding | Blast Radius | Risk | Mitigation |
|---------|-------------|------|------------|
| F5 | Low (single backlog file) | Low | Artifact consistency check |
| F6 | Moderate (core create path) | Moderate | Test-first; rollback is additive |
| F7 | Low (backlog reconciliation record) | Low | Verify-only; containment implementation owned by 108.009-T (099-S) |
