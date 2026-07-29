---
chunk_strategy: h1-h2-h3
description: 'Implementation plan to extract the duplicated durable mkdir-all and directory-fsync mechanics from internal/core/durable_fs.go and internal/events/stream.go into a new internal/fsutil stdlib leaf package, migrate both call sites, and cover the leaf with table-driven unit tests while every pre-existing durable test passes unchanged. Refactor that is behavior-preserving for core and additive-hardening for events: the shared primitive adopts core''s superset semantics (U4 existing-dir retry re-fsync + Finding-2 nested-partial-create ancestor re-confirm), so events GAINS the Finding-2 re-confirm as a strict additive fsync (D2); a documented parameter-gate escape hatch preserves exact events behavior if strict preservation is required. Error classification is applied at the call sites onto blerrors.ErrWriteNotApplied.'
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-07-29-fsutil-durable-mkdir-extraction-plan.md
title: 'Extract shared durable-mkdir/fsync primitive into internal/fsutil leaf'
---

## Source

- Stash entry: `45CA9F83` (priority low, kind task). Refs: `130-F`, `111-S`
  closure deferral (recovered; was stash `345297B2`).
- Deliberation: `docs/decisions/2026-07-29-fsutil-durable-mkdir-extraction-deliberation.md`
  (decisions D1–D4 are authoritative for this plan).
- Prior art / patterns to reuse:
  - Leaf-package pattern: `internal/atomicfile/` (stdlib + `blerrors`),
    `internal/mdfront/` (stdlib-only) — each with `doc.go` + `*_test.go`.
  - Neutral-outcome sibling primitive: `internal/events/fsutil.go`
    (`syncAppendLineDetailed` → `syncAppendResult{preWrite, err}`; caller maps).
  - Durability sentinels + predicates: `internal/errors/durability_errors.go`
    (`ErrWriteNotApplied`, `IsWriteNotApplied`).

## Problem Frame

The durable mkdir-all + directory-fsync logic is maintained in two copies:

- `internal/core/durable_fs.go`
  - `mkdirAllDurable(dir string, durable bool) error` — the richer copy:
    non-durable == `os.MkdirAll`; existing-dir retry re-fsync of parent (U4);
    nested partial-create re-confirm of the first existing ancestor's parent
    (Finding-2); per-new-dir parent fsync; inline `ErrWriteNotApplied` wrapping.
    Seams: package globals `mkdirDirSyncEnabled`, `mkdirDirSyncFn`.
  - `fsyncDirCore(path string) error` — open/`Sync()`/`Close()`.
  - Also `fsyncDirIfDurable`, `durableSyncDirDetailed`, `durableSyncMovedFromDir`
    (move-side helpers) route through the `mkdirDirSyncFn` seam — **not in scope**
    to relocate, but they must keep compiling against whatever `fsyncDirCore`
    becomes.
- `internal/events/stream.go`
  - `(*EventWriter).mkdirAllDurable(dir string) error` — the simpler copy:
    existing-dir re-fsync (U4) via `w.syncDirIfEnabled`; per-new-dir parent
    fsync; **no** Finding-2 nested re-confirm. Caller `appendDurable` wraps
    `ErrWriteNotApplied`. Seams: per-writer `dirSyncEnabled`, `fsyncDirImpl`.
  - `fsyncDir(path string) error` — byte-identical to `fsyncDirCore`.

Goal: one shared implementation in a new `internal/fsutil` stdlib leaf; both call
sites delegate; semantics preserved (events gains the Finding-2 re-confirm as an
additive hardening per D2); error taxonomy applied at the call sites per D1.

**Behavior-change framing (reconciled after plan-review).** This refactor is
*behavior-preserving for `core`* and *additive-hardening for `events`*. It is not
literally "zero behavior change" for `events`: under D2, `events` gains the
Finding-2 nested-partial-create ancestor re-confirm (a pure extra `fsync` in a
rare retry edge case). If a strict no-behavior-change requirement is asserted for
`events`, take the D3 escape hatch — add a `reconfirmNestedAncestor bool`
parameter (core `true`, events `false`) — to preserve exact current events
behavior. D2 (converge) is the default; the parameter gate is the documented
fallback.

## Target Design (from deliberation)

New package `internal/fsutil` (imports: stdlib only — `fmt`, `os`, `path/filepath`):

```go
// FsyncDir opens the directory at path, fsyncs its handle, and closes it so a
// new dirent or rename within it is durable. POSIX-only; callers gate on their
// own dir-sync-enabled flag. (Body identical to fsyncDirCore / fsyncDir today.)
func FsyncDir(path string) error

// MkdirAllDurable creates dir and any missing ancestors. When durable is false
// it is exactly os.MkdirAll(dir, 0o755). When durable and dirSyncEnabled:
//   - if dir exists, re-fsync its parent (U4 retry re-confirm);
//   - else collect missing ancestors deepest-first, re-confirm the first existing
//     ancestor's parent (Finding-2), then create shallowest-first fsyncing each
//     new dir's parent.
// Every directory fsync goes through syncDir. All failures are pre-write; the
// caller maps a non-nil error onto blerrors.ErrWriteNotApplied.
func MkdirAllDurable(dir string, durable, dirSyncEnabled bool, syncDir func(string) error) error
```

`syncDir` is always invoked by the leaf only when `durable && dirSyncEnabled`;
callers pass a non-nil function.

## Task Breakdown

Three tasks. Task B and Task C each depend on Task A (the leaf must land first);
B and C are independent of each other.

### Task A — Create `internal/fsutil` leaf: `FsyncDir` + `MkdirAllDurable` + unit tests

- **Domain:** code + colocated unit tests (single leaf unit; test-first).
- **Files (3):** `internal/fsutil/doc.go`, `internal/fsutil/fsutil.go`,
  `internal/fsutil/fsutil_test.go`.
- **Approach:** test-first. Port `mkdirAllDurable`'s superset body from
  `internal/core/durable_fs.go` into `MkdirAllDurable` with the D3 parameter
  signature (drop the inline `blerrors` wrap — the leaf returns neutral errors,
  keeping the generic `"fsync parent of ..."` / `"mkdir ..."` / `"stat ..."`
  messages). Port `fsyncDirCore` verbatim into `FsyncDir`. `doc.go` states the
  leaf's stdlib-only, no-`core`/no-`events` contract.
- **Test scenarios (table-driven, testify; mirror the existing core tests):**
  1. `durable=false` → exactly `os.MkdirAll`, no fsync.
  2. durable + dirSyncEnabled POSIX → each new ancestor's parent fsynced
     (assert the recorded synced-set, like `TestMkdirAllDurable_FsyncsNewAncestorsPOSIX`).
  3. `dirSyncEnabled=false` (Windows-equivalent) → tree created, no fsync.
  4. per-ancestor fsync error propagates (error contains `"fsync"`).
  5. existing-dir happy path → parent re-fsynced, returns nil.
  6. existing-dir fsync fails → non-nil error (neutral; caller will map).
  7. retry-after-fsync-fail → second call re-syncs parent (U4).
  8. nested partial-create retry → first existing ancestor's parent re-confirmed
     (Finding-2).
- **Acceptance criteria:**
  - `internal/fsutil` imports stdlib only (no `internal/core`, `internal/events`,
    or `internal/errors`).
  - `doc.go` records the leaf-family convention (one line): general filesystem
    primitives here return **neutral** errors and callers map onto the durability
    sentinels at the boundary, whereas backlogit-specific writers (e.g.
    `atomicfile`) may self-classify against `blerrors`.
  - `go test ./internal/fsutil/...` passes; `go vet`, `golangci-lint`, `gofmt` clean.
  - All 8 scenarios present and green.

### Task B — Migrate `internal/core/durable_fs.go` to the leaf

- **Domain:** code (single file).
- **Files (1):** `internal/core/durable_fs.go`.
- **Approach:** replace `mkdirAllDurable`'s body with a thin wrapper that calls
  `fsutil.MkdirAllDurable(dir, durable, mkdirDirSyncEnabled, mkdirDirSyncFn)` and,
  on a non-nil error, wraps it with `blerrors.ErrWriteNotApplied` at the boundary
  so the external contract (`errors.Is(err, ErrWriteNotApplied)` and `"fsync"` in
  message) is preserved. Replace `fsyncDirCore` with `fsutil.FsyncDir` (either
  delete `fsyncDirCore` and point `mkdirDirSyncFn = fsutil.FsyncDir`, or keep
  `fsyncDirCore` as a one-line alias). Keep package seams `mkdirDirSyncEnabled` /
  `mkdirDirSyncFn` so existing core tests that swap them still work. Leave
  `fsyncDirIfDurable`, `durableSyncDirDetailed`, `durableSyncMovedFromDir`
  untouched (they route through the seam).
- **Error-classification note (from plan-review).** Wrapping the leaf result at
  the boundary broadens `core`'s classification: today only `core`'s *fsync*
  failures carry `ErrWriteNotApplied`, while its `stat`, `mkdir`, and
  non-durable `os.MkdirAll` errors are returned unwrapped. Boundary-wrap-all
  makes every path `NotApplied`. This is **safe** — every `MkdirAllDurable`
  failure is genuinely pre-write and retry-idempotent — and no test regresses,
  but it is `errors.Is`-*broadening*, not merely `errors.Is`-preserving. Confirm
  no `core` caller depends on those paths being unclassified (grep shows none).
- **Acceptance criteria:**
  - The duplicated ancestor-collection/mkdir/fsync loop is gone from `core`.
  - `go test ./internal/core/...` passes **unchanged** — specifically
    `durable_fs_test.go`, `archive_durable_write_test.go`,
    `artifacts_durable_test.go`, `artifact_size_durable_retry_test.go`,
    `durable_move_source_fsync_test.go`.
  - `go vet`, `golangci-lint`, `gofmt` clean.
- **Depends on:** Task A.

### Task C — Migrate `internal/events/stream.go` to the leaf

- **Domain:** code (single file).
- **Files (1):** `internal/events/stream.go`.
- **Approach:** replace `(*EventWriter).mkdirAllDurable`'s body with a call to
  `fsutil.MkdirAllDurable(dir, true, w.dirSyncEnabled, syncDir)` where `syncDir`
  is a closure resolving `w.fsyncDirImpl` (falling back to `fsutil.FsyncDir`) —
  i.e. the same resolution `w.syncDirIfEnabled` performs, minus the enabled-gate
  (the leaf gates on `dirSyncEnabled`). The caller `appendDurable` keeps wrapping
  the result with `blerrors.ErrWriteNotApplied` (drop any now-redundant
  double-wrap). Replace `fsyncDir` with `fsutil.FsyncDir` (delete the local
  `fsyncDir`, or alias it). `syncDirIfEnabled` / `syncFile` seams stay for the
  post-append logs-dir fsync path.
- **Behavior note (D2):** events gains the Finding-2 nested-partial-create
  re-confirm. This is additive and must not change append ordering, the
  double-append guard, or error mapping.
- **Load-bearing seam requirement (from plan-review).** The `syncDir` argument
  MUST be a closure that reads `w.fsyncDirImpl` **at call time** and falls back
  to `fsutil.FsyncDir` when nil. A default production `EventWriter` has
  `fsyncDirImpl == nil` and `dirSyncEnabled == true`, and the leaf invokes
  `syncDir` unconditionally when `durable && dirSyncEnabled` — passing
  `w.fsyncDirImpl` directly would nil-panic in production.
- **Acceptance criteria:**
  - The duplicated mkdir loop + local `fsyncDir` are gone from `events`.
  - The `syncDir` closure resolves `w.fsyncDirImpl` with a `fsutil.FsyncDir`
    nil-fallback (no direct-pass nil hazard).
  - `go test ./internal/events/...` passes **unchanged** — specifically
    `stream_durable_test.go`, `stream_test.go`.
  - `go test ./internal/mcp/...` passes unchanged (`append_comment_durable_test.go`
    exercises the events path via the server).
  - `go vet`, `golangci-lint`, `gofmt` clean.
- **Depends on:** Task A.

## Constitution Check

- **I. Safety-First Go:** no `unsafe`; all error paths wrapped with `%w`;
  sentinels routed via `blerrors`. Gates (`go vet`, `golangci-lint`, `gofmt`)
  are per-task acceptance criteria.
- **II. Test-First:** Task A is written test-first; Tasks B/C are validated by
  the pre-existing durable regression suite that must pass unchanged (no new
  behavior to test on the caller side beyond what already exists).
- **VI. Single Responsibility:** no new external dependency; a new internal leaf
  reduces duplication. Justified.
- **X / leaf layering:** `internal/fsutil` is a stdlib-only leaf; no `core`/
  `events` imports, preserving the leaf-primitive dependency direction.
- No destructive commands; no workspace-boundary or history-rewrite concerns.

## Risks and Mitigations

| Risk | Likelihood | Mitigation |
|---|---|---|
| Error-message / `errors.Is` drift breaks a core test asserting `ErrWriteNotApplied` or `"fsync"` | low | Task B wraps at the boundary and keeps generic fsync messages in the leaf; run the full core durable suite. |
| events behavior delta (Finding-2 re-confirm) considered a regression | low | D2 documents it as additive; escape hatch is a parameter-gated re-confirm (core `true`, events `false`). |
| Seam-swapping tests break after delegation | low | Both callers keep their own seams and forward them into the leaf; no test edits needed. |
| Hidden third caller of the duplicated helpers | low | grep confirmed only `core` and `events` define/duplicate these; move-side helpers route through the retained seam. |

## Requires plan hardening

no — pure refactor of already-hardened code; blast radius is two internal files
plus one new leaf; full pre-existing durable regression suite guards behavior;
no runtime/migration/rollout/security surface. Standard plan-review gate suffices.

## Verification

Run at the end of Task C (and per-task as scoped above):

```text
go test ./internal/fsutil/... ./internal/core/... ./internal/events/... ./internal/mcp/... ./internal/atomicfile/...
go vet ./...
golangci-lint run
gofmt -l .
```

All must be clean; every pre-existing durable test must pass unchanged.

## Plan Review

<!-- plan-review-attempt: 1 -->

- dispatch_mode: multi-agent-dispatch
- decision: PASS
- date: 2026-07-29
- reviewers: Scope Boundary Auditor, Architecture Strategist, Go Reviewer
  (three independent persona subagents, cross-lens).

### Gate outcome

All three reviewers returned **PASS** with zero P0/P1 (blocking) findings. Only
P2/P3 advisory findings were raised. The gate is satisfied (`decision == PASS`).

### Findings and dispositions

| # | Sev | Source | Finding | Disposition |
|---|---|---|---|---|
| 1 | P2 | Scope, Arch, Go | "No behavior change" framing conflicts with D2 (events gains the Finding-2 re-confirm). | **Addressed** — plan front-matter + a new "Behavior-change framing" block restate this as behavior-preserving for core, additive-hardening for events, with the D3 parameter-gate escape hatch retained. |
| 2 | P2 | Go (stream.go:227) | events `syncDir` must be a nil-fallback closure reading `w.fsyncDirImpl` at call time, else a default production writer nil-panics. | **Addressed** — Task C now carries an explicit load-bearing seam requirement + acceptance criterion. |
| 3 | P3 | Go (durable_fs.go) | Boundary-wrap-all broadens core's classification (stat/mkdir/os.MkdirAll become NotApplied); safe but not merely "errors.Is-preserving". | **Addressed** — Task B error-classification note documents this as safe and `errors.Is`-broadening; grep confirms no core caller depends on those paths being unclassified. |
| 4 | P3 | Arch | Codify the neutral-vs-self-classifying leaf convention. | **Addressed** — Task A acceptance now requires a one-line convention statement in `fsutil/doc.go`. |
| 5 | P3 | Arch | `MkdirAllDurable` keeps a non-durable passthrough inside a "Durable"-named fn. | **Accepted as-is** — preserves core's call shape and minimal blast radius; noted as possible follow-up, not this refactor. |
| 6 | P3 | Go/Scope | Finding-2 events path is covered only at the leaf (fsutil scenario 8), not by a pre-existing events test; guardrail is a human check. | **Accepted** — intentional for a minimal refactor; the leaf unit test is the automated gate for that path. |

No finding blocks decomposition. Proceed to harvest.

