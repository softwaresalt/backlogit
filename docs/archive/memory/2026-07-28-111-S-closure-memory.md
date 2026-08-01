---
chunk_strategy: h1-h2-h3
doc_type: memory
docline:
    date: 2026-07-29T00:00:00Z
    tags:
        - session-memory
        - shipment-111-S
        - 130-F
        - durable-writes
ingested_at: "2026-07-29T01:40:00Z"
schema_version: "1.0"
source: docs/memory/2026-07-28/111-S-closure-memory.md
title: 'Session Memory — Shipment 111-S Closure (durable_writes second-layer hardening)'
---

# Session Memory — Shipment 111-S Closure

**Session:** Post-merge closure for shipment 111-S (feature 130-F)
**Date:** 2026-07-28 / 2026-07-29
**Branch:** `chore/close-111-S` (off `main@d1be5117`)

## Task IDs Completed

| ID | Title | Status |
|---|---|---|
| 130-F | durable_writes second-layer hardening | done → archived |
| 130.001-T (U1) | ErrWriteIndeterminate reconciliation in UnarchiveItem non-git restore | done → archived |
| 130.002-T (U2) | Explicit indeterminate reconciliation for dependency callers | done → archived |
| 130.003-T (U3) | Re-attempt parent flush on durable append retry | done → archived |
| 130.004-T (U4) | Re-fsync existing dir in core mkdirAllDurable retry | done → archived |
| 130.005-T (U5) | Map durability classes to explicit MCP append_comment outcomes | done → archived |
| 111-S | Shipment manifest | shipped → archived |

## Merge Commit

`d1be5117` — feature PR #315 merged to main (operator-approved, P-009 merge commit)

## Files Changed in PR #315 (source code only)

- `internal/core/durable_fs.go` — U4 existing-dir re-fsync; Finding-2 nested ancestor pre-confirm; Finding-3 fsyncErr preservation; `blerrors` import
- `internal/core/archive.go` — U1 indeterminate continue, commit-then-surface; Finding-5 git branch indeterminate tracking + rollback skip; upsert priority fix
- `internal/core/dependencies.go` — U2 IsWriteIndeterminate guard + explicit UpsertItem reconciliation; `blerrors` import
- `internal/core/shipment.go` — `persistArtifactWriteFn` seam added
- `internal/events/stream.go` — U3 parent re-fsync moved to mkdirAllDurable pre-write; post-write re-fsync removed; doc comment updated
- `internal/mcp/gate_errors.go` — `durabilityOutcomeResult` helper; `fmt` import
- `internal/mcp/tools.go` — `appendCommentFn` seam; `handleAppendComment` durability routing
- `internal/core/durable_fs_test.go` — U4 tests; nested-retry test; path-selective mock fix
- `internal/core/archive_durable_write_test.go` — U1 tests including combined-failure path
- `internal/events/stream_durable_test.go` — U3 test; selective mock fix for DirFsyncFailureIsIndeterminate
- `internal/core/dependencies_indeterminate_test.go` — NEW: U2 tests + items row reconciliation tests
- `internal/mcp/append_comment_durable_test.go` — NEW: U5 tests

## Copilot Review Findings (PR #315 — addressed in commit 9860f7ef)

1. Post-write parent fsync in `appendDurable` wrongly produces `ErrWriteIndeterminate` on retry → moved to mkdirAllDurable pre-write
2. Nested partial-create retry never re-confirms first ancestor's parent → added pre-creation `mkdirDirSyncFn(filepath.Dir(cur))` before creation loop
3. `fsyncErr` dropped from existing-dir `ErrWriteNotApplied` wrap → added `%w: %w` format
4. `items.dependencies` column stale on indeterminate (UpsertItem never reached) → added explicit `db.UpsertItem` in AddDependency/RemoveDependency indeterminate branch
5. Git branch in archive.go never tracked `writeWasIndeterminate`, still called rollback → tracked flag, skip rollback + durableSyncMovedFromDir when indeterminate; reordered DB upsert block

## Decisions

- **Combined-failure contract (U1)**: When restore write is indeterminate AND DB upsert also fails → `errors.Join(ErrWriteIndeterminate, upsertErr)` without calling `restoreArchiveAfterUnarchiveFailure`. This preserves the never-roll-back-indeterminate invariant.
- **Pre-write vs post-write placement (U3 + Finding-1)**: Parent re-fsync must be pre-write (in mkdirAllDurable) to remain `ErrWriteNotApplied`. Post-write placement produces `ErrWriteIndeterminate`, breaking safe-retry semantics.
- **Test seams over OS-level mocking**: `persistArtifactWriteFn` and `appendCommentFn` inject at the call site rather than mocking unexported OS functions. Path-selective mocks required for `mkdirDirSyncFn` to avoid breaking creation assertions.

## Deferred Follow-up

Stash entry `345297B2`: "Extract a shared durable-mkdir primitive into an `fsutil` leaf so `internal/events` and `internal/core` stop maintaining two copies of durable mkdir/fsync." (kind=task, priority=low). DO NOT implement in 111-S scope.

## Closure Branch

Branch: `chore/close-111-S`
Closure PR: TBD (pending push + PR creation)

## Reconcile Reports

- PRE-mode: `.backlogit/reconcile/111-S-pre-20260728T182713.md` → PROCEED (all 6 members pre-archived)
- POST-mode: `.backlogit/reconcile/111-S-post-20260728T182900.md` → PROCEED (all 7 archive files present)

## Next Steps

- Quality gates on closure branch
- `backlogit doctor`
- Commit + push closure artifacts
- Open closure PR, request Copilot review, run §1.9 gate
- HALT for operator merge approval (P-014/§1.10)
