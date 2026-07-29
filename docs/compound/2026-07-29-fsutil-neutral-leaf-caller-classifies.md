---
chunk_strategy: h1-h2-h3
description: 'Context-dependent error classification from 112-S: when the same filesystem op can mean pre-write (NotApplied) or post-write (Indeterminate), use a neutral stdlib-only leaf and let callers classify. FsyncDir must validate IsDir at boundary. Nil-safe closure seam for injectable fsync with nil production default.'
doc_type: learning
docline:
    date: 2026-07-29T00:00:00Z
    severity: medium
    tags:
        - refactor
        - leaf-package
        - neutral-errors
        - fsync
        - fsutil
        - test-seam
        - nil-safe
        - tdd
        - durable-writes
ingested_at: "2026-07-29T09:45:00Z"
schema_version: "1.0"
source: docs/compound/2026-07-29-fsutil-neutral-leaf-caller-classifies.md
title: 'internal/fsutil neutral-leaf: stdlib-only leaf returns plain errors; callers classify into domain sentinels (112-S)'
---

## Problem

`internal/core/durable_fs.go` and `internal/events/stream.go` each carried a
private copy of durable mkdir-all and parent-fsync logic. Consolidating both into
a shared package requires resolving an error-classification tension: the two call
sites use different domain sentinels (`ErrWriteNotApplied` in core,
`ErrWriteIndeterminate` in events), and the same directory-fsync failure maps
to a different class depending on whether it occurred before or after the write.

## Rule 1 — Context-dependent errors require caller classification; stdlib-only leaves make this mandatory

`internal/fsutil` imports only the standard library — no `internal/errors` or
other internal package. This is a stricter constraint than `internal/atomicfile`,
which imports `internal/errors` (itself a stdlib-only leaf) and therefore can
self-classify failures into `ErrWriteNotApplied` / `ErrWriteIndeterminate`.

The key distinction is not "leaf vs. non-leaf based on blerrors import" but
**whether the correct error class depends on context that only the caller has**.
`atomicfile` always knows: failure before the rename commit is `ErrWriteNotApplied`;
post-rename fsync failure is `ErrWriteIndeterminate`. `fsutil.FsyncDir` does not
know — the same fsync failure is `ErrWriteNotApplied` when called before a write
(core mkdir path) and `ErrWriteIndeterminate` when called after a write
(events append path). The leaf must therefore return neutral errors.

- **The leaf returns plain `fmt.Errorf` wraps** — e.g.
  `fmt.Errorf("fsync dir %s: %w", path, err)`.
- **Each caller wraps at its own boundary** into the appropriate sentinel:
  - `internal/core` wraps with `fmt.Errorf("%w: %w", blerrors.ErrWriteNotApplied, err)` for pre-write failures.
  - `internal/events` wraps with `blerrors.ErrWriteIndeterminate` for post-write failures; pre-write failures propagate as `ErrWriteNotApplied` from `mkdirAllDurable`'s own return to `appendDurable`.

> Rule of thumb: when the same filesystem operation can mean either pre-write
> (retriable, NotApplied) or post-write (indeterminate), it belongs in a neutral
> leaf that returns plain OS errors. Self-classifying is only appropriate when the
> caller context is always unambiguous — as it is for atomicfile's rename boundary.

## Rule 2 — Exported fsync helpers MUST validate the path is a directory

`os.Open` and `f.Sync` succeed on a regular file handle. A function named
`FsyncDir(path string) error` that silently succeeds on a file is a
latent correctness bug — callers assume they are fsyncing a directory entry.

**Pattern:** stat the path before opening, and reject non-directories explicitly:

```go
func FsyncDir(path string) error {
    info, err := os.Stat(path)
    if err != nil {
        return fmt.Errorf("stat %s: %w", path, err)
    }
    if !info.IsDir() {
        return fmt.Errorf("fsync dir %s: not a directory", path)
    }
    f, err := os.Open(path)
    if err != nil {
        return fmt.Errorf("open dir %s: %w", path, err)
    }
    defer f.Close()
    if err := f.Sync(); err != nil {
        return fmt.Errorf("sync dir %s: %w", path, err)
    }
    return nil
}
```

This was caught by Copilot review in 112-S (thread PRRT_kwDORzozKM6Ur2zF). The
test `TestFsyncDir_ErrorOnNonExistentPath` checks the error contains the path
string (not a fixed prefix), so the error-string change from `"open dir %s: ..."` to
`"stat %s: ..."` does not break the test.

## Rule 3 — Nil-safe closure seam when production default is nil

`internal/events.EventWriter` exposes `fsyncDirImpl func(string) error` for test
injection. The production default is `nil` — callers expect the live `fsutil.FsyncDir`
to be used when `fsyncDirImpl == nil`. Passing `w.fsyncDirImpl` directly to
`fsutil.MkdirAllDurable`'s `syncDir` parameter nil-panics on the production path.

**Pattern:** always build a nil-safe closure that reads the field at call time:

```go
syncDir := func(path string) error {
    if w.fsyncDirImpl != nil {
        return w.fsyncDirImpl(path)
    }
    return fsutil.FsyncDir(path)
}
// now pass syncDir to fsutil.MkdirAllDurable
```

Key properties:
- The closure captures `w` (the receiver), not `w.fsyncDirImpl`. This means if the
  field is set between closure creation and invocation, the closure sees the latest
  value.
- The fallback `fsutil.FsyncDir` is the live implementation; no package-level
  fallback variable is needed.
- Tests that override `w.fsyncDirImpl` before calling the function under test work
  correctly — the closure delegates to the injected function.

Contrast with `internal/core`'s `mkdirDirSyncFn` (package-level var, always set to
a non-nil function): that pattern is simpler but ties the seam to a single package.
The closure pattern is necessary when the seam is per-instance rather than
per-package.

## Rule 4 — Superset convergence: migrate both call sites to the richer semantics

When two private copies have diverged and one is richer (U4 retry re-fsync;
Finding-2 ancestor re-confirm on nested partial creates), migrate both to the
superset rather than:
- parameterizing away the new behavior (adds complexity, defers the hardening), or
- maintaining two versions (defeats the purpose of extraction).

The caller that gained behavior (events: gets Finding-2 ancestor re-confirm) must
document this as "additive hardening" in the closure, not a behavioral regression.
The caller that preserved behavior (core: `ErrWriteNotApplied` broadening to
stat/mkdir paths) must document this as "intentional, conservative broadening" and
verify no caller depended on those paths being unclassified.

## Evidence

- `internal/fsutil/fsutil.go` — `FsyncDir` with `stat+IsDir` guard;
  `MkdirAllDurable` with Finding-2 and U4 re-confirm.
- `internal/fsutil/fsutil_test.go` — 9 TDD tests covering non-directory path,
  non-durable passthrough, U4 re-fsync, Finding-2 nested partial, dir-sync-disabled.
- `internal/core/durable_fs.go` — thin wrapper with `ErrWriteNotApplied` at boundary.
- `internal/events/stream.go` — nil-safe `syncDir` closure, fallback to `fsutil.FsyncDir`.
- PR #318, merge commit `899b70e2`, feature 131-F, shipment 112-S.
- Closure: `docs/closure/2026-07-29-112-S-fsutil-leaf-extraction-closure.md`.
- Deliberation: `docs/decisions/2026-07-29-fsutil-durable-mkdir-extraction-deliberation.md`.
