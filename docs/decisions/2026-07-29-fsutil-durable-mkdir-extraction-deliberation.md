---
chunk_strategy: h1-h2-h3
description: 'Deliberation for extracting the duplicated durable mkdir-all + directory-fsync mechanics from internal/core/durable_fs.go and internal/events/stream.go into a new internal/fsutil stdlib leaf package. Records the error-classification (sentinel-placement) decision, the semantics-convergence decision for the events call site, the seam-parameterization decision, and the scope boundary. Corrects the intake premise that the durability sentinels live in internal/core (they already live in the internal/errors leaf).'
doc_type: decision
schema_version: "1.0"
source: docs/decisions/2026-07-29-fsutil-durable-mkdir-extraction-deliberation.md
title: 'fsutil durable-mkdir/fsync extraction — design deliberation'
---

## Context

- Stash intake: `45CA9F83` (priority low, kind task) — "Extract a shared
  durable-mkdir/fsync primitive into a new internal/fsutil leaf package so
  internal/events (stream.go) and internal/core (durable_fs.go) stop
  maintaining two copies of durable mkdir/fsync." Refs: `130-F`, `111-S`
  closure deferral (recovered; was stash `345297B2`).
- This is a **behavior-preserving-for-core, additive-hardening-for-events**
  refactor of durability code hardened in shipment `111-S` / feature `130-F`
  (merged `d1be5117`). It is **not** literally "no behavior change": under D2 the
  `events` call site **gains** an extra ancestor re-confirm `fsync` (the
  Finding-2 nested-partial-create re-confirm it does not perform today), and
  under D1/D2 the `core` call site **broadens** its `ErrWriteNotApplied`
  classification to also cover its `stat`/`mkdir`/non-durable `os.MkdirAll`
  failure paths (all genuinely pre-write and retry-safe). Both deltas are
  intentional and enumerated in D1/D2 below; Ship MUST NOT treat this as a
  strict byte-for-byte behavior-preservation task.

The refactor targets two duplicated mechanics:

1. **Directory fsync.** `internal/core/durable_fs.go:fsyncDirCore` and
   `internal/events/stream.go:fsyncDir` are byte-identical (open dir → `Sync()`
   → `Close()`, POSIX-only).
2. **Durable mkdir-all.** `internal/core/durable_fs.go:mkdirAllDurable(dir, durable)`
   (function) and `internal/events/stream.go:(*EventWriter).mkdirAllDurable(dir)`
   (method) both create missing ancestors shallowest-first and fsync each new
   directory's parent, but they differ (see D2/D3 below).

## Corrected grounding (intake premise was wrong)

The intake note stated: *"the durable error sentinels (`ErrWriteIndeterminate`,
`ErrWriteNotApplied`) currently live in internal/core."* **This is incorrect.**
The sentinels live in the `internal/errors` leaf (`blerrors`):

- `internal/errors/durability_errors.go` defines both sentinels and the
  `IsWriteNotApplied` / `IsWriteIndeterminate` predicates. Its doc comment: they
  live in the errors leaf "so atomicfile, events, core, cli, and mcp classify
  via `errors.Is`/`As` without importing internal/core."
- Precedent: `internal/atomicfile/atomicfile.go:10` imports
  `blerrors "github.com/softwaresalt/backlogit/internal/errors"` and wraps the
  sentinels itself. A leaf importing `internal/errors` is already sanctioned.

Consequence: **no error-package move is required.** The "pure leaf" constraint
for `internal/fsutil` means *no imports of `internal/core` or `internal/events`*;
importing the `internal/errors` leaf (like `atomicfile` does) would still be leaf-clean.
This reframes — but does not eliminate — the sentinel-placement decision below.

## Decision D1 — Error classification lives at the call sites (neutral leaf)

**Options considered:**

- **D1-A (chosen) — neutral outcome, callers map.** `fsutil.MkdirAllDurable`
  returns a plain `error`; each caller (`core`, `events`) wraps it with
  `blerrors.ErrWriteNotApplied` at its call boundary. `fsutil` imports **stdlib
  only** (no `blerrors`).
- **D1-B — self-classifying leaf.** `fsutil` imports `blerrors` and wraps
  `ErrWriteNotApplied` itself (mirrors `atomicfile`).

**Decision: D1-A.** Rationale:

1. **Sibling-primitive consistency.** The append primitive that already lives in
   `internal/events/fsutil.go` (`syncAppendLineDetailed`) returns a *neutral*
   `syncAppendResult{preWrite bool, err error}` and lets the caller
   (`appendDurable`) map onto `blerrors`. The new mkdir primitive should follow
   the same neutral-outcome-then-caller-maps shape for a coherent fsutil surface.
2. **Narrowest leaf dependency.** Keeping `fsutil` pure stdlib matches the
   `mdfront`/`atomicfile`-family intent of minimal, well-layered leaves and keeps
   `fsutil` reusable outside backlogit's error vocabulary.
3. **Trivial for mkdir.** Every `MkdirAllDurable` failure is **pre-write** by
   construction (stat/mkdir/dir-fsync all happen before any canonical file write),
   so the classification the callers apply is unconditional
   `ErrWriteNotApplied` — there is no indeterminate case to encode. `events`
   already wraps at the call site today; `core` moves from **inline** wrapping
   (only its `fsync` failures carry `ErrWriteNotApplied`) to **call-boundary**
   wrapping (all paths). This is `errors.Is`-**broadening** for `core`'s
   `stat`/`mkdir`/non-durable `os.MkdirAll` paths — an intentional, safe delta
   (every path is genuinely pre-write and retry-idempotent), not a purely
   mechanical no-op. grep confirms no `core` caller depends on those paths being
   unclassified.

D1-B remains a valid alternative (atomicfile precedent) and is noted for the
reviewer, but D1-A is preferred and also matches the operator's stated intake
preference ("fsutil returns a neutral typed outcome; callers keep mapping").

## Decision D2 — Shared primitive implements core's superset semantics; events converges

The two `mkdirAllDurable` implementations differ in ONE behavior:

| Behavior | core | events |
|---|---|---|
| Per-new-dir parent fsync (POSIX) | yes | yes |
| Existing-dir retry re-fsync of parent (U4) | yes | yes |
| Non-durable path == `os.MkdirAll` | yes (`durable=false`) | via separate `appendFast` |
| **Nested partial-create: re-confirm first existing ancestor's parent (Finding-2)** | **yes** | **no** |

**Decision:** the shared `fsutil.MkdirAllDurable` implements **core's superset**
(including the Finding-2 nested-partial-create re-confirm). `events` migrates onto
it and thereby **gains** the nested-ancestor re-confirm.

Rationale: consolidation means keeping the *more correct* copy. The events delta
is a **pure additive `fsync`** of an already-existing ancestor's parent, only in
the rare nested-partial-create *retry* edge case. It:

- cannot duplicate an append (the append still happens once, after mkdir),
- cannot lose or corrupt data (it is a durability re-confirm, identical in kind
  to the U4 top-level re-fsync events already performs),
- introduces no new error class and no retry-loop change.

**Guardrail for Ship:** verify (a) every existing durable test across
`internal/core`, `internal/events`, `internal/mcp`, `internal/atomicfile` passes
**unchanged**, and (b) the events change is *only* the added re-confirm `fsync`
(no change to append ordering, error mapping, or the double-append guard).

If a reviewer judges the events behavior delta unacceptable for a "no behavior
change" refactor, the fallback is to gate the nested re-confirm behind a
parameter (core passes `true`, events `false`) to preserve each caller's exact
current behavior. D2 (converge) is preferred for a single correct implementation;
the parameter fallback is the documented escape hatch.

## Decision D3 — Seams are parameters, not package globals

`core` uses package-global seams (`mkdirDirSyncEnabled`, `mkdirDirSyncFn`);
`events` uses per-writer injectable fields (`dirSyncEnabled`, `fsyncDirImpl`).

**Decision:** the leaf takes the gate + fsync function as parameters:

```go
package fsutil

// FsyncDir opens the directory at path and fsyncs its handle (POSIX). Callers
// gate on their own dir-sync-enabled flag.
func FsyncDir(path string) error

// MkdirAllDurable creates dir and missing ancestors. durable=false is exactly
// os.MkdirAll. When durable and dirSyncEnabled, it fsyncs each new dir's parent,
// re-fsyncs an existing dir's parent on retry (U4), and re-confirms the first
// existing ancestor's parent on nested partial-create retry (Finding-2), calling
// syncDir for every directory fsync. Any failure is pre-write; the caller maps
// it onto blerrors.ErrWriteNotApplied.
func MkdirAllDurable(dir string, durable, dirSyncEnabled bool, syncDir func(string) error) error
```

- `core` passes its package globals: `MkdirAllDurable(dir, durable, mkdirDirSyncEnabled, mkdirDirSyncFn)`.
- `events` passes its per-writer seam: `dirSyncEnabled=w.dirSyncEnabled` and a
  `syncDir` closure that resolves `w.fsyncDirImpl` (falling back to `fsutil.FsyncDir`).

Both callers' existing seam-swapping tests remain valid because each keeps its own
seam and simply forwards it into the leaf.

## Decision D4 — Scope boundary (minimal blast radius)

**In scope:** move ONLY the duplicated durable mkdir-all + directory-fsync into
`internal/fsutil`; migrate `internal/core/durable_fs.go` and
`internal/events/stream.go` to it; add `internal/fsutil` unit tests.

**Out of scope:** the append primitive `syncAppendLineDetailed` /
`syncAppendResult` in `internal/events/fsutil.go` is **not** duplicated in `core`
(core writes via `atomicfile`, not append), so it stays in package `events`.
Graduating it into the leaf is a possible future follow-up, not this refactor.
`atomicfile`, `blerrors`, and the two-class error contract are untouched.

## Done looks like

- New `internal/fsutil` stdlib-only leaf: `doc.go`, `fsutil.go` (`FsyncDir`,
  `MkdirAllDurable`), `fsutil_test.go`.
- `internal/core/durable_fs.go` and `internal/events/stream.go` delegate to the
  leaf; the duplicated mkdir loop + dir-fsync helper are gone from both.
- `go build ./...`, `go vet ./...`, `golangci-lint run`, `gofmt -l .` clean.
- All pre-existing durable tests pass **unchanged**; new fsutil unit tests cover
  the superset semantics (non-durable, POSIX per-ancestor fsync, Windows skip,
  fsync-error propagation, existing-dir happy path, existing-dir fsync fail,
  retry-after-fail re-sync, nested partial-create first-ancestor re-confirm).

## Open questions for plan-review

1. D1-A (neutral) vs D1-B (self-classifying like atomicfile) — is neutral the
   right call given atomicfile is self-classifying? (Deliberation recommends
   neutral for sibling-primitive consistency.)
2. D2 convergence — is the additive events re-confirm acceptable under a
   "no behavior change" refactor, or should it be parameter-gated?
