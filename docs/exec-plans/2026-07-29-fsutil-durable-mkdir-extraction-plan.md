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

Five tasks. The leaf is split so each exported function has direct atomic
test coverage. Backlog ID mapping: `131.001-T` (leaf + `FsyncDir`),
`131.004-T` (`MkdirAllDurable` creation path), `131.005-T` (`MkdirAllDurable`
retry/re-confirm), then the two migrations `131.002-T` (core) and `131.003-T`
(events). Dependency chain: `131.001-T → 131.004-T → 131.005-T`; **both
migrations depend on `131.005-T`** (the complete primitive), and are independent
of each other.

### Task A1 — Create `internal/fsutil` leaf + `FsyncDir` with direct tests (131.001-T)

- **Domain:** code + colocated unit tests (test-first).
- **Files (3):** `internal/fsutil/doc.go`, `internal/fsutil/fsutil.go`,
  `internal/fsutil/fsutil_test.go`.
- **Approach:** create the stdlib-only leaf; port `fsyncDirCore` verbatim into
  `FsyncDir(path string) error`. `doc.go` states the leaf's stdlib-only,
  no-`core`/no-`events` contract and the neutral-error convention.
- **Test scenarios (direct `FsyncDir` coverage, <5):**
  1. `FsyncDir` on a real directory returns nil (handle opened, synced, closed).
     Guard this success case with a `runtime.GOOS == "windows"` skip — production
     disables directory fsync on Windows (`dirSyncEnabled = runtime.GOOS != "windows"`),
     where a directory-handle `Sync()` is unsupported and would spuriously fail
     the test on a Windows host.
  2. `FsyncDir` on a non-existent / non-openable path returns an error naming the
     path (open-error case; reliable cross-platform).
- **Acceptance criteria:**
  - `internal/fsutil` imports stdlib only (no `internal/core`, `internal/events`,
    or `internal/errors`).
  - `doc.go` records the leaf-family convention (one line): general filesystem
    primitives here return **neutral** errors and callers map onto the durability
    sentinels at the boundary, whereas backlogit-specific writers (e.g.
    `atomicfile`) may self-classify against `blerrors`.
  - `FsyncDir` has **direct** success + open-error tests (not merely exercised via
    `MkdirAllDurable`).
  - `go test ./internal/fsutil/...` passes; `go vet`, `golangci-lint`, `gofmt` clean.

### Task A2 — Add `fsutil.MkdirAllDurable` creation-path semantics + tests (131.004-T)

- **Domain:** code + colocated unit tests (test-first). **Depends on 131.001-T.**
- **Files (2):** `internal/fsutil/fsutil.go`, `internal/fsutil/fsutil_test.go` (extend).
- **Approach:** add `MkdirAllDurable(dir string, durable, dirSyncEnabled bool, syncDir func(string) error) error`
  implementing the fresh-creation path — non-durable == `os.MkdirAll(dir, 0o755)`;
  durable collects missing ancestors and creates them shallowest-first, fsyncing
  each new directory's parent via `syncDir`. Neutral errors (generic
  `"mkdir ..."` / `"stat ..."` / `"fsync parent of ..."` messages). Callers pass
  `fsutil.FsyncDir` (or their seam) as `syncDir`.
- **Test scenarios (table-driven, testify; <5):**
  1. `durable=false` → exactly `os.MkdirAll`, no fsync.
  2. durable + `dirSyncEnabled` POSIX → each new ancestor's parent fsynced
     (assert the recorded synced-set).
  3. `dirSyncEnabled=false` (Windows-equivalent) → tree created, no fsync.
  4. per-ancestor fsync error propagates (error contains `"fsync"`).
- **Acceptance criteria:**
  - 4 scenarios present and green; `go test ./internal/fsutil/...` passes;
    `go vet`, `golangci-lint`, `gofmt` clean.

### Task A3 — Add `fsutil.MkdirAllDurable` retry/re-confirm semantics + tests (131.005-T)

- **Domain:** code + colocated unit tests (test-first). **Depends on 131.004-T.**
- **Files (2):** `internal/fsutil/fsutil.go`, `internal/fsutil/fsutil_test.go` (extend).
- **Approach:** add the existing-dir parent re-fsync (U4) and the nested
  partial-create first-existing-ancestor re-confirm (Finding-2) to
  `MkdirAllDurable`; any fsync failure returns a neutral error. This completes
  core's superset semantics.
- **Test scenarios (table-driven, testify; <5):**
  5. existing-dir happy path → parent re-fsynced, returns nil.
  6. existing-dir fsync fails → non-nil neutral error (caller will map).
  7. retry-after-fsync-fail → second call re-syncs parent (U4).
  8. nested partial-create retry → first existing ancestor's parent re-confirmed
     (Finding-2).
- **Acceptance criteria:**
  - 4 scenarios present and green; `MkdirAllDurable` now implements core's full
    superset (U4 + Finding-2); `go test ./internal/fsutil/...` passes;
    `go vet`, `golangci-lint`, `gofmt` clean.

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
- **Depends on:** Task A3 (`131.005-T`) — needs the complete `MkdirAllDurable`
  (U4 + Finding-2) since core's tests assert those semantics.

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
- **Depends on:** Task A3 (`131.005-T`) — needs the complete `MkdirAllDurable`
  including the Finding-2 re-confirm that events gains.

## Constitution Check

- **I. Safety-First Go:** no `unsafe`; all error paths wrapped with `%w`;
  sentinels routed via `blerrors`. Gates (`go vet`, `golangci-lint`, `gofmt`)
  are per-task acceptance criteria.
- **II. Test-First:** Tasks A1–A3 are written test-first, each exported function
  carrying direct atomic coverage (`FsyncDir` in A1; `MkdirAllDurable` creation
  path in A2; retry/re-confirm in A3). Tasks B/C are validated by the
  pre-existing durable regression suite that must pass unchanged, plus the
  runtime verification in the Runtime Impact section below.
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

no — the refactor targets already-hardened code with a small blast radius (two
internal files plus one new leaf) and the full pre-existing durable regression
suite guards behavior. This is **not** a no-runtime-surface change, however (see
the Runtime Impact section below): Task C adds an `fsync` on the durable
event-append hot path and Task B changes observable error classification. Because
those runtime deltas are bounded and their verification, monitoring, and rollback
evidence are supplied inline below, a separate plan-harden pass is not required;
the standard plan-review gate plus the Runtime Impact evidence suffices.

## Runtime Impact and Release Observability

This change **is runtime-affecting**. Correcting the earlier "no runtime surface"
framing (plan-review finding):

- **Task C (events) — new latency + failure point.** Routing
  `EventWriter.appendDurable`'s mkdir through the shared primitive means the
  durable event-append path performs the Finding-2 ancestor re-confirm `fsync`
  it did not before. That re-confirm is **not** gated on a retry flag: it fires
  on **any** fresh creation where the target dir is missing and the first
  existing ancestor's parent differs from itself (per
  `internal/core/durable_fs.go`), i.e. on normal first-time nested creation, not
  only on a retry. On POSIX with `durable_writes` enabled this adds one directory
  `fsync` — extra I/O latency and a new failure point on the durable audit/log
  append path. Real-world frequency for events is low because its `logsDir`
  usually pre-exists (the existing-dir branch, which already re-fsyncs the parent
  today), but the delta is real and must be treated as runtime-affecting.
- **Task B (core) — observable error-class change.** Boundary-wrap-all broadens
  `core`'s `stat`/`mkdir`/non-durable `os.MkdirAll` failures from unclassified to
  `ErrWriteNotApplied`. Callers that branch on `IsWriteNotApplied` (retry vs
  surface-indeterminate) will now see those paths classified as safe-to-retry.

### Runtime verification (Ship must perform post-build, pre-merge)

- Run the full durable suites with `durable_writes` enabled:
  `go test ./internal/fsutil/... ./internal/core/... ./internal/events/... ./internal/mcp/... ./internal/atomicfile/...`.
- Exercise a real durable event append against a fresh (uncreated) logs tree and
  confirm the log line is written exactly once and the dirents are fsynced
  (no double-append across a simulated parent-fsync-fail retry) —
  `stream_durable_test.go` `TestAppendDurable_ParentFsyncFail_RetryResyncesParent`
  is the automated proxy; confirm it stays green.
- Confirm `append_comment` via the MCP server path still succeeds under durable
  mode (`internal/mcp/append_comment_durable_test.go`).

### Monitoring signals (durable_writes deployments)

- **SLI:** durable event-append error rate and `ErrWriteIndeterminate` incidence
  on the item-log / hook-queue path. **Baseline:** unchanged from `130-F`/`111-S`
  (the added fsync is a re-confirm, not a new indeterminate source). **Alert
  threshold:** any sustained rise in `ErrWriteIndeterminate` on the append path,
  or a new `ErrWriteNotApplied` spike from `core` mkdir paths, after rollout.
- Where no metrics backend exists, this is a manual observation item: watch for
  new durable-write warnings in logs during the first post-merge runs.

### Rollback

- **Trigger:** a regression in durable append correctness (double-append,
  data-loss, or a new indeterminate/not-applied misclassification surfaced by the
  suites or field logs).
- **Procedure:** the change is behind the existing `durable_writes` gate and is a
  contained refactor — revert the shipment's commits (merge-commit revert) to
  restore the two in-place `mkdirAllDurable` copies; no data migration or state
  change is involved, so revert is clean.

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

<!-- plan-review-attempt: 2 -->

- dispatch_mode: multi-agent-dispatch
- decision: PASS
- date: 2026-07-29
- reviewers: Scope Boundary Auditor, Go Reviewer (re-review after the PR #317
  task-structure change: leaf split into 131.001-T/131.004-T/131.005-T for
  direct per-function coverage, plus the added Runtime Impact section).

### Gate outcome (attempt 2)

Both re-reviewers returned **PASS** with zero P0/P1. The prior blockers
(FsyncDir lacked direct coverage; single task bundled two exported functions and
8 scenarios) are resolved. Only P2/P3 advisories remained; the accuracy-relevant
ones are addressed below.

| # | Sev | Source | Finding | Disposition |
|---|---|---|---|---|
| 1 | P2 | Go | FsyncDir direct success test needs a `runtime.GOOS == "windows"` skip (production disables dir fsync on Windows; unguarded it fails on a Windows host). | **Addressed** — Task A1 scenario 1 now specifies the Windows skip guard. |
| 2 | P2 | Go | Runtime Impact wording ("nested partial-create *retry* edge case") understated when the events re-confirm fsync fires — it is not retry-gated; it fires on any fresh missing-dir creation. | **Addressed** — the Task C runtime bullet now states the re-confirm fires on any fresh nested creation (events `logsDir` usually pre-exists, so real frequency is low). |
| 3 | P3 | Scope | 131.004-T's atomic milestone is internal-only (no consumer uses the intermediate MkdirAllDurable). | **Accepted** — the split is forced by the <5-scenario rule; the semantic seam (creation vs U4/Finding-2) is real. |
| 4 | P3 | Scope | Release-observability apparatus is heavyweight for a small refactor. | **Accepted** — required by the PR review finding (#5); kept as evidence, not code scope. |
| 5 | P3 | Go | FsyncDir Sync()/Close()-error branches lack direct coverage. | **Accepted** — optional fault-injection; the success + open-error pair is the agreed minimum. |

No finding blocks decomposition. Revised structure stands.

