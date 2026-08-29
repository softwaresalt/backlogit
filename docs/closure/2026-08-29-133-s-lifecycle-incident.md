---
chunk_strategy: h1-h2-h3
description: "Lifecycle incident record for 133-S / 150-F: P-001 violation — tasks archived from active status, skipping done transition"
doc_type: closure
schema_version: "1.0"
source: ship-agent
title: 133-S Lifecycle Incident — P-001 Task Lifecycle Gap (150.001-T, 150.002-T)
---

# 133-S Lifecycle Incident Record

**Detected**: 2026-08-29T19:06:31Z (post-Ship read-only remote check, after PR #391 merge)
**Reported by**: Ship agent (autonomous post-closure verification)
**Status**: All governed remediation paths blocked; no durable correction applied. `archived_status` field requires operator action.

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
properly evaluated at task-completion time. The RED/GREEN commit SHAs and test results above
are the authoritative evidence of functional completeness.

## Governed Remediation Attempts

The Ship agent investigated ALL governed backlogit lifecycle operations that might correct the
`archived_status` field without ad-hoc frontmatter editing.

### Operations Attempted

| Operation | Command | Outcome |
|-----------|---------|---------|
| Lifecycle transition | `backlogit move 150.001-T --status done` | BLOCKED: `"status archived has no allowed transitions"` |
| Lifecycle transition | `backlogit move 150.002-T --status done` | BLOCKED: same lifecycle hook rejection |
| Field update | `backlogit update 150.001-T --status done` | BLOCKED: same lifecycle hook rejection |
| Force gate override | `backlogit move 150.001-T --status done --force-gates --force-reason "..."` | BLOCKED: `--force-gates` bypasses pre-task-completion gate only; `validate_status_transition` hook is not bypassable |
| Log annotation | `backlogit comment add 150.001-T --actor ship-agent --comment "..."` | SUCCESS (local only — gitignored) |
| Log annotation | `backlogit comment add 150.002-T --actor ship-agent --comment "..."` | SUCCESS (local only — gitignored) |

### Why Log Annotations Are Not a Governed Durable Correction

The `.backlogit/logs/*.jsonl` files are gitignored. The lifecycle incident comments appended
locally to `150.001-T.jsonl` and `150.002-T.jsonl` exist only in the local worktree filesystem.
They are NOT persisted to `origin/main` and are NOT part of the permanent git record.

## What Cannot Be Done (Governed Operations)

The `backlogit` lifecycle state machine treats `archived` as a terminal state with NO allowed
transitions. This is enforced by the `validate_status_transition` pre-update hook, which cannot
be bypassed by any current flag (including `--force-gates`). There is no `unarchive`, `restore`,
or `--archived-status` operation in the current backlogit API surface.

**Conclusion**: The `archived_status: active` field in `.backlogit/archive/150.001-T.md` and
`.backlogit/archive/150.002-T.md` CANNOT be corrected to `archived_status: done` via any
governed backlogit lifecycle operation.

## Operator Recovery Options

Two options are available. The operator must choose one:

### Option A: Accept the Lifecycle Gap (Permanent Incident Record)

**Action**: No further file changes needed.

The lifecycle gap is permanently documented in:
1. This incident record (`docs/closure/2026-08-29-133-s-lifecycle-incident.md`)
2. The main closure record addendum (`docs/closure/2026-08-29-133-s-cleanupcheckpoints-closure.md`)
3. The merged remediation PR

**P-001 status**: Lifecycle gap acknowledged; the historical record (`archived_status: active`)
remains as a factual artifact. Tasks were functionally complete (see evidence above).

### Option B: Manual Frontmatter Correction

**Action**: Directly edit the two archive markdown files to correct `archived_status`.

Step-by-step:

1. **Edit both files**:
   - `.backlogit/archive/150.001-T.md`: change `archived_status: active` to `archived_status: done`
   - `.backlogit/archive/150.002-T.md`: change `archived_status: active` to `archived_status: done`
   - Do NOT add inline YAML comments in the frontmatter (backlogit may strip them on round-trips)

2. **Rebuild the index**:
   ```
   backlogit sync
   ```
   Without this step the SQLite query cache retains `archived_status: active`.

3. **Verify**:
   ```
   backlogit get 150.001-T
   backlogit get 150.002-T
   ```
   Confirm both report `archived_status: done`.

4. **Commit** with a transparent provenance note:
   ```
   fix(harness): correct archived_status to done for 150.001-T and 150.002-T

   - Archived from active in PR #391 (1dfdd6ce); governed ops blocked.
   - Incident: docs/closure/2026-08-29-133-s-lifecycle-incident.md

   ⚠️ - Generated by Copilot
   ```

**P-001 status**: Option B corrects the `archived_status` metadata field to reflect the
intended terminal value. It does NOT retroactively create a governed `active → done` transition
event. The historical lifecycle gap (no done event was ever emitted in the original flow)
remains a documented fact. The P-001 violation is partially addressed by correcting the recorded
field value and this incident record.

**Note**: The Ship agent does NOT execute Option B because the operator instruction states to
halt with precise operator recovery rather than editing archived headers by hand.

## Tooling Gap — Systemic Follow-Up

Two backlog items are recommended for the backlogit tool itself:

1. **Administrative `archived_status` correction command**: A governed `backlogit archive --set-archived-status done <id>` or similar operation that allows operators to correct `archived_status` with an explicit audit event, bypassing the lifecycle state machine for administrative corrections.

2. **Pre-archive lifecycle guard**: A pre-archive check that rejects archiving an item whose `status` is not `done` (for tasks/subtasks) or `shipped` (for shipments), forcing the correct terminal transition before archival.

These gaps mean the current backlogit API cannot recover from this class of lifecycle protocol error.

## P-001 Status Summary

| Aspect | Status |
|--------|--------|
| Substantive completeness (code merged, tests pass) | ✓ COMPLETE |
| Lifecycle protocol (active → done → archived) | ✗ GAP — tasks went active → archived |
| Governed correction attempted | ✓ ATTEMPTED — all paths blocked |
| Incident documented | ✓ THIS DOCUMENT |
| Operator action required | Pending (Option A or B above) |

## 11FFF601 Final Closure Status

This lifecycle incident is the only remaining open item for the 11FFF601 / 150-F / 133-S
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
- `150.001-T archived_status: done`: ✗ OPEN — `active` in archive (requires operator action)
- `150.002-T archived_status: done`: ✗ OPEN — `active` in archive (requires operator action)
