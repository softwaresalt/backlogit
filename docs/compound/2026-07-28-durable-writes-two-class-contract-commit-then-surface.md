---
chunk_strategy: h1-h2-h3
description: 'durable_writes two-class error contract: commit-then-surface ErrWriteIndeterminate for post-mutation fsync failures (never roll back an indeterminate write), fsync the SOURCE parent on cross-directory durable moves, and level-by-level POSIX parent fsync for durable mkdir. Includes the opt-in-default severity heuristic that keeps caller/retry gaps as P2 follow-ups.'
doc_type: learning
docline:
    date: 2026-07-28T00:00:00Z
    severity: high
    tags:
        - durable-writes
        - fsync
        - posix
        - error-contract
        - err-write-indeterminate
        - commit-then-surface
        - atomic-write
        - archive
        - adopt
        - opt-in-default
        - review-disposition
        - gofmt-windows
ingested_at: "2026-07-28T16:00:00Z"
schema_version: "1.0"
source: docs/compound/2026-07-28-durable-writes-two-class-contract-commit-then-surface.md
title: 'durable_writes: the two-class fsync contract and the commit-then-surface pattern for post-mutation durability'
---

## Problem

Making Markdown/JSONL writes, cross-directory moves, archive/restore, and
directory creation power-loss durable is not just "add an fsync." Each fsync sits
at a different point relative to the DB/state mutation it protects, and getting
the error semantics wrong either loses durability silently or corrupts FS/DB
consistency. Feature 123-F (shipment 109-S, PR #308) landed the primitives; four
Copilot review cycles hardened them.

## The two-class write-error contract

A durable write has exactly two failure classes, distinguished by whether the
on-disk mutation already happened:

- **`ErrWriteNotApplied`** — the failure occurred **before** the rename/commit,
  so the target file is untouched. The caller MAY safely retry; nothing was
  applied.
- **`ErrWriteIndeterminate`** — the rename/commit **already happened** but the
  subsequent durability flush (fsync of the file's or directory's parent) failed.
  The write may or may not survive power loss. The caller MUST NOT retry blindly
  and MUST NOT roll back — either action risks duplicating or destroying an
  applied write.

Both classes live in `internal/errors/durability_errors.go`
(`blerrors.ErrWriteIndeterminate`, `blerrors.IsWriteIndeterminate`).

## Commit-then-surface (the key pattern)

When a post-mutation directory fsync fails **after** an operation has already
mutated the filesystem and staged DB changes, do **not** roll back. Rolling back
an indeterminate write is the primary hazard: the rename likely persisted, so a
rollback diverges FS from DB or destroys an applied change. Instead:

1. Accumulate the fsync failure (`errors.Join`) without returning.
2. Let the operation finish normally — commit the DB transaction, append the
   event, build the result. FS and DB stay in agreement.
3. At the very end, return the fully-built result **wrapped with**
   `ErrWriteIndeterminate` so an error-only caller and `IsWriteIndeterminate`
   both see the honest durability signal.

`AdoptItem` (`internal/core/shipment_lifecycle.go`) is the reference: its md- and
log-rename dir fsyncs run before `tx.Commit()`, accumulate into a
function-scoped `durSyncErr`, and surface via `adoptDurabilityErr(newID, err)`
after commit. `persistArtifact` Site-1 (`internal/core/shipment.go`) is the
before-commit variant: it fsyncs the source parent before the sole `UpsertItem`,
so it surfaces indeterminate cleanly.

## Source-parent fsync on cross-directory durable moves

A durable move that fsyncs only the **destination** parent can, after power loss,
resurrect the removed **source** dirent alongside the durable new one — leaving a
duplicate canonical artifact. `durableSyncMovedFromDir` fsyncs the source parent
after the old entry is removed/renamed, gated on dirs-differ and POSIX
(`fsyncDirIfDurable`). Archive/unarchive move sites use the best-effort
`slog.Warn` variant (they run after a completed move whose rollback is already
wired); same-directory adopt renames use the returning
`durableSyncDirDetailed` so the failure can be surfaced per commit-then-surface.

## Directory-creation durability

`mkdirAllDurable(dir, durable)` creates missing ancestors shallowest-first and,
on POSIX, fsyncs each new directory's **parent** so the new dirent survives power
loss. It is exactly `os.MkdirAll` when `durable` is false. Any code that creates
a directory it then writes into durably (e.g. a fresh `.backlogit/archive` on
first archive) must use `mkdirAllDurable`, not plain `os.MkdirAll` — otherwise the
directory's own dirent in its parent is never flushed and the whole directory can
vanish after a reported success.

## Windows/POSIX seam

Windows exposes no directory-handle flush, so dirent durability is best-effort
there. `mkdirDirSyncEnabled` (defaults to `runtime.GOOS != "windows"`) and the
`mkdirDirSyncFn` seam let tests exercise and fail the per-ancestor flush
in-process. Tests that swap these seams MUST NOT use `t.Parallel` (the seam is a
package global read on the production write path).

## Severity heuristic: opt-in default gates edge-case severity

`durable_writes` is opt-in (default false). Findings that require **durable ON +
an actual fsync failure + a retry** are triple-gated and inert in the default
configuration. Combined with the fact that Markdown is the source of truth and
the SQLite index self-heals from it on `sync`, such caller/retry-completeness
gaps are legitimately **P2 follow-ups**, not merge blockers — even when a
reviewer frames them as consistency bugs. Cycle-4 of PR #308 dispositioned five
such findings to stash `50471E28` rather than opening a fourth fix cycle past the
§1.8 limit.

## gofmt-on-Windows gotcha (verification)

With `.gitattributes * text=auto` + `core.autocrlf=true`, `gofmt -l .` flags ~96
files on CRLF noise alone. Verify formatting on **LF-normalized, BOM-free** copies
of the committed blobs: `(git show HEAD:file) -join "\n"`, write with
`System.Text.UTF8Encoding($false)` (no BOM), then `gofmt -l`. Do **not** use
`Set-Content -Encoding utf8` — it injects a BOM and gofmt reports a false
"expected ';', found '('" parse error at line 1.
