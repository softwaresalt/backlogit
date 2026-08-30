---
chunk_strategy: h1-h2-h3
description: "Lifecycle incident record for 133-S / 150-F: P-001 violation — tasks archived from active status, skipping done transition"
doc_type: closure
schema_version: "1.0"
source: docs/closure/2026-08-29-133-s-lifecycle-incident.md
title: 133-S Lifecycle Incident — P-001 Task Lifecycle Gap (150.001-T, 150.002-T)
---

# 133-S Lifecycle Incident Record

**Detected**: 2026-08-29T19:06:31Z (post-Ship read-only remote check, after PR #391 merge)
**Reported by**: Ship agent (autonomous post-closure verification)
**Status**: RESOLVED — 2026-08-30. `ReconcileArchivedLifecycle` (152-F, PR #394) implements the Option B path. Applied in application PR #395. Both `150.001-T` and `150.002-T` now have `archived_status: done`. See `docs/closure/2026-08-29-133-s-cleanupcheckpoints-closure.md` Lifecycle Reconciliation Addendum.

## P-001 Contradiction

### What Was Found

| Item | `status` | `archived_status` | Expected `archived_status` |
|------|----------|-------------------|---------------------------|
| `150.001-T` | `archived` | `active` | `done` |
| `150.002-T` | `archived` | `active` | `done` |
| `150-F` | `archived` | `done` | `done` ✓ |
| `133-S` | `archived` | `shipped` | `shipped` ✓ |

The tasks were archived from `active` status in closure commit `5e1b385d` (PR #391,
2026-08-29T18:38:25Z). P-001 requires the lifecycle path `active → done → archived`. The
`done` transition was never persisted to the committed record; items went `active → archived`
directly. This is a real lifecycle gap, not a documentation error.

### Transparent Timing

- The `backlogit move --status done` operations were never staged or committed (impl worktree
  local execution only), confirmed by the archive frontmatter (`archived_status: active`).
- The tasks were archived in closure PR #391 with `archived_status: active` because that was
  their committed record state at the time.
- This incident was detected AFTER PR #391 merged. No backdating is claimed. The incident is
  documented with current timestamps.

## Why `archived_status` Cannot Be Directly Corrected

The `archived_status` field is NOT a documentation annotation. It is a functional field:
`ArchiveItem` stores the pre-archive status there (`internal/core/archive.go:229-231`) and
`UnarchiveItem` reads it to restore the task to its original state (`internal/core/archive.go:775-784`).
Directly changing `archived_status: active → done` would cause a future `UnarchiveItem` call to
restore the task as `done` when the task was never formally moved to `done`. This would fabricate
history and corrupt the restore semantics. **Direct frontmatter editing of `archived_status` is
therefore harmful, not corrective.**

## Completeness Evidence (Tasks Were Functionally Done)

Both tasks were effectively complete when archived. The lifecycle gap is procedural, not substantive:

| Evidence | Value |
|----------|-------|
| RED commit (150.001-T) | `6fdd5c58` — `TestCleanupCheckpoints_NoPreRemoveInAST` FAILS as required |
| GREEN commit (150.002-T) | `1ace3861` — pre-Remove block and runtime import removed; test PASSES |
| Final Copilot fix | `af54c6a3` — test comment precision |
| All quality gates | PASS at `af54c6a3` (see closure record `2026-08-29-133-s-cleanupcheckpoints-closure.md`) |
| PR #390 merge | `e3deede6` — code fix merged into `origin/main` |
| PR #391 merge | `1dfdd6ce` — closure merged into `origin/main` |

Note: A `pre_task_completion_gate_passed` event exists in the local `.backlogit/logs/` for
`150.001-T`, but with `"old_status":"archived"` — confirming it was emitted during the
post-archive remediation attempt (trying to move from archived to done), not during any original
`active → done` completion flow. This event does not constitute evidence that the gate was
properly evaluated at task-completion time.

## Governed Remediation Attempts

The Ship agent investigated ALL governed backlogit lifecycle operations that might correct the
`archived_status` field.

### Operations Attempted

| Operation | Command | Outcome |
|-----------|---------|---------|
| Lifecycle transition | `backlogit move 150.001-T --status done` | BLOCKED: `"status archived has no allowed transitions"` |
| Lifecycle transition | `backlogit move 150.002-T --status done` | BLOCKED: same lifecycle hook rejection |
| Field update | `backlogit update 150.001-T --status done` | BLOCKED: same lifecycle hook rejection |
| Force gate override | `backlogit move 150.001-T --status done --force-gates --force-reason "..."` | BLOCKED: `--force-gates` bypasses pre-task-completion gate only; `validate_status_transition` hook is not bypassable |
| Log annotation | `backlogit comment add 150.001-T --actor ship-agent --comment "..."` | SUCCESS (local only — gitignored, not PR-able) |
| Log annotation | `backlogit comment add 150.002-T --actor ship-agent --comment "..."` | SUCCESS (local only — gitignored, not PR-able) |

### Why Direct Frontmatter Editing Is Harmful

The `.backlogit/logs/*.jsonl` files are gitignored — log annotations do not persist to the git
record. And as described above, direct frontmatter editing of `archived_status` would corrupt
`UnarchiveItem` restore semantics. **The correct governed path is restore → transition → re-archive,
which is not currently exposed as a CLI command.**

## What Cannot Be Done (Current CLI)

The `backlogit` lifecycle state machine treats `archived` as a terminal state with NO allowed
transitions (`validate_status_transition` hook; cannot be bypassed by `--force-gates`). There is
no `unarchive` or `restore` CLI command, even though `UnarchiveItem` exists internally. Direct
frontmatter editing is harmful to provenance. No current CLI operation can safely correct
`archived_status` for these items.

## Operator Recovery

Only one safe option exists with the current tooling:

### Option A: Accept the Lifecycle Gap (Permanent Incident Record)

**Action**: No further file changes needed.

The lifecycle gap is permanently documented in:
1. This incident record (`docs/closure/2026-08-29-133-s-lifecycle-incident.md`)
2. The main closure record addendum (`docs/closure/2026-08-29-133-s-cleanupcheckpoints-closure.md`)
3. The merged remediation PR

**P-001 status**: Lifecycle gap acknowledged; the historical record (`archived_status: active`)
is preserved as a factual artifact reflecting the actual state at archiving time. Tasks were
functionally complete (see evidence above). `UnarchiveItem` restore semantics remain intact.

### Option B (Future — When Governed Restore Is Available)

When a `backlogit restore` or `unarchive` CLI command is added to the tool, the correct
remediation flow would be:

1. `backlogit restore 150.001-T` — restores item from archive to queue at `active` status,
   removes `archived_status` field (preserves correct restore semantics via `UnarchiveItem`)
2. `backlogit move 150.001-T --status done` — drives the required done transition with a
   `pre_task_completion_gate_passed` event in the audit log
3. `backlogit archive 150.001-T` — re-archives with `archived_status: done` (correct value)
4. Repeat for `150.002-T`

This path creates a genuine `done` transition event, correctly sets `archived_status: done`,
and does not corrupt `UnarchiveItem` restore semantics. This was the proposed Option B.

> **Applied 2026-08-30**: `ReconcileArchivedLifecycle` (152-F, PR #394) implements exactly this
> sequence. Applied in PR #395. See Resolution Record below.

## Tooling Gaps — Systemic Follow-Up

Two backlog items are recommended for the backlogit tool:

1. **Governed restore CLI command**: A `backlogit restore <id>` command that wraps `UnarchiveItem`,
   moving an archived item back to its pre-archive status with a full audit event trail. This is the
   prerequisite for the correct P-001 remediation path (Option B above).

2. **Pre-archive lifecycle guard for completion-claiming archival**: A pre-archive check that warns
   (or rejects) when archiving a task/subtask from a non-terminal status in a completion context.
   This guard must distinguish between explicit descope archival (where archiving from `active` or
   `queued` is intentional, per `docs/compound/2026-07-20-ship-gate-descoped-archived-member-exemption.md`)
   and completion-claiming archival (where the expectation is `done → archived`).

## P-001 Status Summary (Historical — at 2026-08-29)

| Aspect | Status |
|--------|--------|
| Substantive completeness (code merged, tests pass) | ✓ COMPLETE |
| Lifecycle protocol (active → done → archived) | ✗ GAP — tasks went active → archived |
| Governed correction attempted | ✓ ATTEMPTED — all paths blocked |
| Direct frontmatter fix | ✗ HARMFUL — corrupts UnarchiveItem restore semantics |
| Incident documented | ✓ THIS DOCUMENT |
| Operator action required | Option A (accept gap) is the only safe current path |

**Superseded**: See Resolution Record (2026-08-30) — Option B was applied via 152-F.

## 11FFF601 Final Closure Status (Historical — at 2026-08-29)

At 2026-08-29, this lifecycle incident was the only remaining open item for the 11FFF601 / 150-F / 133-S
release unit. All other closure criteria are met:

- Code fix merged: ✓ (PR #390, `e3deede6`)
- P-002 RED/GREEN evidence immutable: ✓ (commits `6fdd5c58`, `1ace3861`)
- Quality gates: ✓ (all pass at `af54c6a3`)
- Adversarial review (pre-PR): ✓ (HIGH consensus, 3/3, 0 P0/P1)
- Copilot review: ✓ (0 unresolved threads at `af54c6a3`)
- CI: ✓ (all checks pass at `af54c6a3`)
- P-014 defense-in-depth: ✓ (passed at `af54c6a3`)
- P-009 merge commit: ✓
- Closure record: ✓
- Compound learning: ✓
- `150-F archived_status: done`: ✓
- `133-S archived_status: shipped`: ✓
- `150.001-T archived_status: done`: ✓ RESOLVED — corrected to `done` on 2026-08-30 (PR #395)
- `150.002-T archived_status: done`: ✓ RESOLVED — corrected to `done` on 2026-08-30 (PR #395)

## Resolution Record (2026-08-30)

**Applied**: PR #395 (chore/152-application, shipment 134-S)
**Operator**: ship-agent
**Method**: ReconcileArchivedLifecycle (152-F, shipped in PR #394)

The governed reconciliation sequence was:

1. UnarchiveItem(150.001-T) — restored to queue at active status (from archived_status field)
2. setItemStatusAndMeta(150.001-T, done) — set status=done, wrote reconciliation metadata
3. ArchiveItem(150.001-T, WithCascade=false) — re-archived with archived_status=done
4. Repeat steps 1-3 for 150.002-T

**Post-correction state**:

| Item | archived_status | reconciled_at | original_archived_status |
|------|-----------------|---------------|--------------------------|
| 150.001-T | done | 2026-08-30T05:00:02Z | active |
| 150.002-T | done | 2026-08-30T05:00:53Z | active |

The P-001 violation is corrected. The historical direct-archive fact is preserved in
custom_fields.original_archived_status and the original commit history.

**Open item resolved**: This was the only remaining open item noted in the original incident
record. No further action is required.
