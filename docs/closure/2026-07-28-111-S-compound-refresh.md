---
chunk_strategy: h1-h2-h3
doc_type: closure
docline:
    date: 2026-07-29T00:00:00Z
    tags:
        - compound-refresh
        - durable-writes
        - 111-S
        - 130-F
ingested_at: "2026-07-29T01:35:00Z"
schema_version: "1.0"
source: docs/closure/2026-07-28-111-S-compound-refresh.md
title: 'Compound Refresh — Shipment 111-S (durable_writes second-layer hardening)'
---

# Compound Refresh — Shipment 111-S

**Context:** Feature 130-F / Shipment 111-S — durable_writes second-layer hardening (PR #315, merge commit d1be5117)
**Mode:** apply
**Date:** 2026-07-29

## Entries Reviewed

| File | Classification | Action |
|---|---|---|
| `2026-07-28-durable-writes-two-class-contract-commit-then-surface.md` | update | Extended with U1–U5 second-layer findings |
| All other compound entries | keep | No overlap with durable-writes or test seam topics |

## New Entry Created

| File | Reason |
|---|---|
| `2026-07-29-durable-writes-test-seam-patterns.md` | New reusable learning: injectable seam patterns (persistArtifactWriteFn, appendCommentFn, path-selective mkdirDirSyncFn mocks); t.Parallel prohibition for package-global seams |

## Evidence Used

- PR #315 source diffs: `internal/core/durable_fs.go`, `archive.go`, `dependencies.go`, `shipment.go`; `internal/events/stream.go`; `internal/mcp/tools.go`, `gate_errors.go`
- Test files: `durable_fs_test.go`, `archive_durable_write_test.go`, `dependencies_indeterminate_test.go`, `stream_durable_test.go`, `append_comment_durable_test.go`
- Five Copilot review findings from PR #315 (addressed in commit `9860f7ef`)

## Update Detail: durable-writes-two-class-contract-commit-then-surface.md

Added "Second-layer hardening" section covering:

1. **U4 + Finding-2**: `mkdirAllDurable` existing-dir re-fsync (pre-write, ErrWriteNotApplied); nested ancestor pre-confirm before creation loop; fsyncErr preservation in error wrap
2. **U3**: Pre-write vs post-write parent fsync placement — post-write → ErrWriteIndeterminate (unsafe); pre-write (in mkdirAllDurable) → ErrWriteNotApplied (safe retry)
3. **U1**: Combined-failure `errors.Join(ErrWriteIndeterminate, upsertErr)` contract — skip `restoreArchiveAfterUnarchiveFailure` when write was indeterminate; git-move branch tracks `writeWasIndeterminate` and skips rollback
4. **U2**: `items.dependencies` stale-column reconciliation via explicit `db.UpsertItem` on `IsWriteIndeterminate` in AddDependency/RemoveDependency
5. **U5**: `durabilityOutcomeResult` maps both durability classes to machine-readable MCP outcomes for exactly-once retry semantics

## Follow-up Items

- Stash `345297B2`: "Extract a shared durable-mkdir primitive into an `fsutil` leaf so `internal/events` and `internal/core` stop maintaining two copies of durable mkdir/fsync." (kind=task, priority=low) — when implemented, update the `mkdirAllDurable` sections in both compound entries.
