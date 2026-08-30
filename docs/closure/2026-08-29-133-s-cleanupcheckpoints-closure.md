---
chunk_strategy: h1-h2-h3
description: "Post-merge closure record for 133-S / 150-F: CleanupCheckpoints pre-Remove data-loss fix"
doc_type: closure
schema_version: "1.0"
source: ship-agent
title: 133-S Closure — CleanupCheckpoints pre-Remove fix (150-F)
---

# 133-S Closure Record

## Shipment Metadata

| Field | Value |
|-------|-------|
| Shipment ID | 133-S |
| Feature | 150-F |
| Tasks | 150.001-T (RED harness), 150.002-T (GREEN fix) |
| Stash source | 11FFF601 |
| PR | #390 |
| Feature branch | fix/150-cleanup-checkpoints-preremove |
| Closure branch | chore/133-s-closure |
| Merge commit | e3deede67e04d1d60a29c4feddde8e0e3379bacf |
| Reviewed HEAD | af54c6a386f396d3f9a6c1881de791b2cbef6eb9 |
| Merged at | 2026-08-29T18:24:40Z |

## P-002 Evidence

| Phase | Commit | Output |
|-------|--------|--------|
| FC-2 preflight | — | Confirmed os.Remove(dst) at L435-437, runtime import at L10 |
| RED commit | 6fdd5c58 | TestCleanupCheckpoints_NoPreRemoveInAST FAILS — PREREMOVE_FOUND_IN_CLEANUPCHECKPOINTS |
| GREEN fix | 1ace3861 | os.Remove(dst) block + runtime import removed; test PASSES |
| Copilot fix | af54c6a3 | Comment precision improved per Copilot review thread PRRT_kwDORzozKM6dbri1 |

## Quality Gates

| Gate | Status |
|------|--------|
| go test ./... | PASS |
| go vet ./... | PASS |
| golangci-lint run | PASS |
| gofmt -l . | PASS (changed files formatted) |

## Adversarial Review (pre-PR)

| Reviewer | Model | P0/P1 | Verdict |
|----------|-------|-------|---------|
| T1 | claude-sonnet-5 | 0 | No findings |
| T2 | gpt-5.6-terra | 0 | No findings |
| T3 | gemini-3.1-pro-preview | 0 | No findings |

Consensus: HIGH (3/3) — fix is correct and safe.

## Copilot Review

| Round | State | Threads | Outcome |
|-------|-------|---------|---------|
| Round 1 (1ace3861) | COMMENTED | 1 unresolved | Fixed: corrected data-loss scope and rename property in comment |
| Round 2 (af54c6a3) | COMMENTED | 0 unresolved | 0 unresolved threads; review body flagged tasks-not-done (see note) |

**Round 2 review body finding (not a thread)**: Copilot flagged 🔵 Needs a closer look — both tasks
remained ctive (not done) at merge. The move done CLI ops executed locally in the impl
worktree were not staged/committed before merge; tasks went ctive → archived in this closure
PR rather than ctive → done → archived. Functionally complete: code merged, tests pass.

## P-014 Gate (pre-merge, defense-in-depth)

- headRefOid: af54c6a386f396d3f9a6c1881de791b2cbef6eb9
- Check 1 (no pending review): PASS — reviewRequests.nodes: []
- Check 2 (review freshness): PASS — Copilot review at 2026-08-29T18:18:07Z covers headRefOid
- Check 3 (thread resolution): PASS — 1 thread, isResolved: true
- reviewDecision: null (no branch protection requiring review approval)

## P-009 Verification

- allow_merge_commit: true
- allow_squash_merge: false
- allow_rebase_merge: false
- Merge strategy: MERGE COMMIT ONLY ✓

## CI Evidence

| Check | Run 1 (1ace3861) | Run 2 (af54c6a3) |
|-------|-----------------|-----------------|
| test | pass (4m30s) | pass (5m21s) |
| CLI Reference Drift | pass | pass |
| Docline frontmatter gate | pass | pass |
| Markdown lint (P-008) | pass | pass |
| Detect code changes | pass | pass |

## Release Observability

**Change class**: Pure deletion fix (3 lines removed, 1 import removed). No new API surface, no schema change, no data migration, no configuration change.

**SLI / monitoring**: CheckpointCleanup archive operations. Healthy signal: no increase in cleanup errors; no increase in missing checkpoint archive entries. Baseline: zero errors in checkpoint cleanup log entries post-merge.

**Pre-deploy audit**: No feature flags, no rollout gates, no migration — pure code deletion.

**Observation window**: 24 hours post-merge. Owner: operator. Monitor via `backlogit checkpoint cleanup` logs.

**Rollback trigger**: If checkpoint archive operations begin emitting unexpected errors on Windows (os.Rename failure rate > baseline), revert the fix by reverting e3deede6.

**Rollback procedure**: `git revert e3deede6 --no-edit && git push origin main` (requires operator approval as destructive history change).

## Runtime Verification

The fix removes a Windows-specific pre-Remove call. On Linux/macOS, behavior is identical (the removed block was GOOS-gated). On Windows:
- Go 1.24.0 os.Rename → MoveFileExW(MOVEFILE_REPLACE_EXISTING) — handles existing dst without pre-Remove
- Verified by existing TestCleanupCheckpoints tests which exercise archive paths
- No regression: all 30 events package tests pass

## Scope Compliance

| Item | In scope | Disposition |
|------|----------|-------------|
| 11FFF601 stash (CleanupCheckpoints pre-Remove) | YES | Fixed and merged |
| 302EFF07 (symlink rejection) | NO | Excluded per operator directive |
| Other dark-run stash entries | NO | Not touched |

## Archived Items

Archived post-merge:
- 133-S (shipment) → archived
- 150-F (feature) → archived
- 150.001-T (RED harness task) → archived
- 150.002-T (GREEN fix task) → archived
- Stash 11FFF601 → archived

## Durable Knowledge

Compound learning: same pre-Remove data-loss pattern applied to a third location (checkpoint_lifecycle.go CleanupCheckpoints). Pattern: Windows os.Rename pre-Remove blocks are universally unnecessary in Go 1.24.0 — MoveFileExW(MOVEFILE_REPLACE_EXISTING) handles existing destination. See docs/compound/ for the pattern entry.

## Addendum — P-001 Lifecycle Incident (2026-08-29)

A post-Ship read-only remote check detected a P-001 lifecycle contradiction: `150.001-T` and
`150.002-T` have `archived_status: active` in their archive frontmatter. The required lifecycle
path is `active → done → archived`; both tasks skipped the `done` transition and went
`active → archived` directly in closure commit `5e1b385d`.

All governed backlogit lifecycle operations to correct `archived_status` were attempted and blocked
by the `validate_status_transition` hook. Direct frontmatter editing is harmful: `archived_status`
is used by `UnarchiveItem` to restore the pre-archive state, so overwriting it would corrupt
restore semantics. Only Option A (accept the lifecycle gap as documented) is safe with current
tooling. A `backlogit restore` command is recommended as a tooling follow-up.

**Full incident record**: `docs/closure/2026-08-29-133-s-lifecycle-incident.md`

**Round 2 Copilot review body note** (not a thread): The Copilot review body in PR #390 flagged
that both tasks remained active (not done) at merge. This was disclosed in the original closure
record but not corrected at that time. This addendum closes the disclosure gap.

## Lifecycle Reconciliation Addendum (2026-08-30)

Added as part of 152-F post-merge application (PR #395, shipment 134-S).

### P-001 Incident Correction

Tasks 150.001-T and 150.002-T were archived directly from ctive status, bypassing the
done transition — a P-001 violation noted in the lifecycle incident record
docs/closure/2026-08-29-133-s-lifecycle-incident.md.

The governed ReconcileArchivedLifecycle operation (152-F) was applied to correct this:

| Task | Original archived_status | Reconciled to | Reconciled at | Actor |
|------|--------------------------|---------------|---------------|-------|
| 150.001-T | active | done | 2026-08-30T05:00:02Z | ship-agent |
| 150.002-T | active | done | 2026-08-30T05:00:53Z | ship-agent |

Each item was unarchived → updated to done → re-archived with a durable
lifecycle_reconciliation event. The original direct-archive history is preserved;
the reconciliation records are additive (see custom_fields.reconciled_at,
custom_fields.original_archived_status).

### Stash Provenance Correction

Stash entry 11FFF601 was auto-harvested as 151-F (duplicate artifact) but the canonical
actual delivery was 150-F/133-S.

The governed CorrectStashProvenance operation (152-F) was applied:

| Field | Value |
|-------|-------|
| Stash ID | 11FFF601 |
| Historical artifact (stash archive) | 151-F |
| Canonical delivery | 150-F |
| Corrected at | 2026-08-30T05:02:13Z |
| Actor | ship-agent |
| Reason | Stash auto-harvested as 151-F but actual delivery was 150-F/133-S; 151-F was a duplicate artifact |

The correction is recorded in .backlogit/archive/provenance_corrections.jsonl.
The original harvested_artifact_id: 151-F in the stash archive is preserved (never mutated).
After acklogit sync, stash_links resolves 11FFF601 → 150-F.

### Verification Evidence

- 150.001-T archive: rchived_status: done, econciled_at: 2026-08-30T05:00:02Z ✅
- 150.002-T archive: rchived_status: done, econciled_at: 2026-08-30T05:00:53Z ✅
- provenance_corrections.jsonl: entry with stash_id: 11FFF601, canonical_delivery_artifact_id: 150-F ✅
- stash_links DB: 11FFF601 → 150-F (after sync) ✅
- Item log events: lifecycle_reconciliation on 150.001-T and 150.002-T, stash_provenance_corrected on 150-F ✅