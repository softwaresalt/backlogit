---
description: "Post-merge closure for shipment 112-S (fsutil durable-mkdir/fsync leaf extraction, feature 131-F, PR #318 merged at 899b70e2). Records runtime verification, operational readiness, Copilot review findings, and the neutral-leaf design decisions D1-D4."
doc_type: closure
chunk_strategy: h1-h2-h3
schema_version: "1.0"
docline:
  ms.date: 2026-07-29T00:00:00Z
  ms.topic: reference
source: docs/closure/2026-07-29-112-S-fsutil-leaf-extraction-closure.md
title: "112-S fsutil leaf extraction post-merge closure"
---

## Scope

Shipment **112-S** — extract duplicated durable-mkdir/fsync mechanics into a new
`internal/fsutil` stdlib-only leaf package (feature **131-F**, tasks 131.001-T
through 131.005-T). Delivered via PR **#318** on branch `feat/112-S-fsutil`,
merged to `main` as merge commit **`899b70e2`** (P-009 merge-commit strategy).
Shipped in dark factory mode (P-017) for the declared 112-S scope.

The work consolidates two independent implementations:

- `internal/core/durable_fs.go` — `mkdirAllDurable` (durable_writes
  triple-gating: mkdir, fsync new dirs, re-fsync existing dir on retry).
- `internal/events/stream.go` — `appendDurable` path's parent-dir fsync logic
  (re-fsync parent on durable append retry).

Both call sites now delegate to `fsutil.MkdirAllDurable`, which carries a strict
superset of the former `mkdirAllDurable` semantics (D2: convergence — U4 retry
re-fsync and Finding-2 nested-partial-create ancestor re-confirm). The
`internal/fsutil` package imports only the Go standard library (D1: neutral leaf).

## Design Decisions Recorded

| ID | Decision | Rationale |
|---|---|---|
| D1 | `internal/fsutil` is a pure stdlib leaf — no `internal/errors` or other internal imports | Needed because callers map the same error to different classes; importing blerrors would force premature classification. Mirrors `internal/mdfront` (pure stdlib); stricter than `internal/atomicfile` which also imports the `internal/errors` stdlib-only leaf |
| D2 | `MkdirAllDurable` is a superset (core behavior preserved; events gains additive Finding-2 hardening) | Unifying two divergent impls on the richer semantics is safer than introducing a parameter gate |
| D3 | Seam parameterized: `(dir string, durable, dirSyncEnabled bool, syncDir func(string) error)` | Enables injection of per-package fsync mock without exposing package-level state |
| D4 | Scope boundary: only `fsyncDirCore`/`fsyncDir` and `mkdirAllDurable` moved; `syncAppendLineDetailed` stays in `internal/events/fsutil.go` | Avoids pulling events-only logic into the leaf |

## Files Delivered

| File | Change |
|---|---|
| `internal/fsutil/doc.go` | New — stdlib-only leaf declaration; neutral-error caller-classification convention |
| `internal/fsutil/fsutil.go` | New — `FsyncDir` (with `stat+IsDir` directory validation) and `MkdirAllDurable` |
| `internal/fsutil/fsutil_test.go` | New — 9 TDD table-driven tests (U4, Finding-2, dir-sync-disabled, non-durable passthrough) |
| `internal/core/durable_fs.go` | Modified — `mkdirAllDurable` replaced with thin wrapper; `fsyncDirCore` removed; `mkdirDirSyncFn = fsutil.FsyncDir` |
| `internal/events/stream.go` | Modified — `mkdirAllDurable` method and local `fsyncDir` removed; `appendDurable` uses `fsutil.MkdirAllDurable` via nil-safe `syncDir` closure |

## Runtime Verification

- **CI on reviewed feature HEAD (`440f92c7`)**: 5/5 checks green — `test`,
  `CLI Reference Drift`, `Detect code changes`, `Docline frontmatter gate`,
  `Markdown lint (P-008)`. The merge commit is `899b70e2`.
- **Local quality gates (all passes)**: `go test ./...` exit 0, `go vet ./...`
  exit 0, `golangci-lint run` exit 0, `gofmt` clean on all changed files.
- **Targeted runtime-verification tests**:
  - `TestAppendDurable_ParentFsyncFail_RetryResyncesParent` — U4 retry
    re-syncs parent, confirms exactly-one event written (no double-append).
  - `TestAppendEvent_DurableOnFreshTreeCreatesAndSyncsAncestors` — fresh tree
    creation plus ancestor fsync on first write.
  - `TestAppendEvent_DirFsyncFailureIsIndeterminate` — post-write fsync failure
    correctly surfaces as `ErrWriteIndeterminate`, not `ErrWriteNotApplied`.
  - `TestHandleAppendComment_RetryIdempotency_ExactlyOneEvent` — MCP
    `append_comment` path: retry is idempotent, exactly one event persisted.
  - `TestHandleAppendComment_IndeterminateAppend_ReturnsDistinctOutcome` —
    correct error-class surfacing through the MCP path.
  - All 8 pre-existing `TestMkdirAllDurable_*` in `internal/core` pass unchanged
    (`ErrWriteNotApplied` contract preserved for all core callers).

Verification method: automated test suite and local quality gates. No browser
or external-integration surfaces involved (pure filesystem durability refactor).

## Operational Readiness

**READY.** Rationale:

- **Rollback**: revert merge commit `899b70e2`. No data migration, no schema
  change, no index format change. Both `mkdirAllDurable` implementations (core
  and events) are restored by the revert; no callers are left stranded.
- **Error-classification broadening (intentional, documented)**: the new
  `mkdirAllDurable` wrapper in `internal/core/durable_fs.go` wraps ALL errors
  with `ErrWriteNotApplied` at the boundary (including `os.Stat` and
  `os.MkdirAll` errors that previously returned unwrapped). All such paths are
  pre-write and retry-idempotent. No core caller depended on those paths being
  unclassified (verified by grep). This broadening is conservative and safe.
- **Additive events hardening**: `appendDurable` gains Finding-2 ancestor
  re-confirm on nested partial creates. This is strictly additive — it
  fsyncs more aggressively but never skips a write.
- **Monitoring (manual structured checklist)**: no automated monitoring system
  is wired for this repository; the monitoring plan is recorded here as a manual
  observation requirement per `release-observability.instructions.md`.
  - **SLI / key signals**: durable event-append error rate; `ErrWriteIndeterminate`
    incidence on the item-log / hook-queue path; `ErrWriteNotApplied` spike from
    core mkdir paths after rollout.
  - **Observation location / query**: workspace structured log stream; grep the
    agent/CLI log for `ErrWriteIndeterminate` and `ErrWriteNotApplied`; compare
    against baseline established by shipment 109-S (durable_writes protocol).
  - **Baseline**: zero `ErrWriteIndeterminate` under normal operation (fsync
    errors are transient I/O conditions, not expected in routine use).
  - **Trigger threshold**: any sustained rise in `ErrWriteIndeterminate` on the
    append path after rollout, or a new cluster of `ErrWriteNotApplied` from
    core mkdir paths, indicates a regression.
  - **Response owner**: the operator who triggered the durable-writes workload.
  - **Observation window**: manual, ongoing. The `durable_writes` flag remains
    opt-in/false by default; the monitoring checklist activates when a workspace
    opts in.
  - **Observed outcome (this release)**: runtime-verification tests pass; no
    unexpected `ErrWriteIndeterminate` or `ErrWriteNotApplied` surfaced during
    the test suite.

## Copilot Review Summary

| Cycle | Findings | Disposition |
|---|---|---|
| 1 | Thread 1 (PRRT_kwDORzozKM6Ur2zF): `FsyncDir` should validate path is a directory before `Sync` — regular files succeed silently | Fixed in `440f92c7`: added `os.Stat + info.IsDir()` guard before `os.Open` |
| 1 | Thread 2 (PRRT_kwDORzozKM6Ur2zm): doc comment overstated "callers map onto `ErrWriteNotApplied`" — `syncDirIfEnabled` maps post-write failures to `ErrWriteIndeterminate` | Fixed in `440f92c7`: updated `FsyncDir` doc comment and `doc.go` to clarify pre-write vs post-write classification |

Both threads replied to and resolved via GraphQL `resolveReviewThread` mutation.
Second Copilot review confirmed `state: COMMENTED`, covered HEAD `440f92c7`,
0 unresolved threads.

## Shipment Reconcile

| Gate | Result |
|---|---|
| PRE (before archive) | **PROCEED** — all 7 items (`112-S`, `131-F`, `131.001-T` through `131.005-T`) confirmed in `.backlogit/queue/` with `queued` status |
| POST (after archive) | **PROCEED** — all 7 items confirmed in `.backlogit/archive/` with `archived` status |

Doctor: 1 pre-existing known orphan (`016.001-R` — predates 112-S), 0 new
orphans introduced by this shipment.

## Residual Risk

None identified. The `internal/fsutil` leaf is pure stdlib with no import risk.
The error-classification broadening in core is conservative (pre-write paths only).
The additive events hardening adds fsync coverage without removing any write.

The stash entry **50471E28** from shipment 109-S captures the next-layer
durable_writes hardening work (full `ErrWriteIndeterminate` caller reconciliation,
retry-idempotency completeness) and remains the authoritative follow-up tracker
for durable_writes second-layer work. Shipment 112-S does not alter the scope of
that stash entry; it only migrates the mkdir/fsync mechanics.

## Dark-Mode Record

- `DARK_MODE_START` / `DARK_MODE_SCOPE`: shipment 112-S, feature 131-F.
- `LOCAL_REVIEW_READY`: reviewed HEAD `440f92c7`, 0 unresolved P0/P1 local
  findings, 0 deferred items.
- `DARK_MODE_MERGE_AUTHORIZED`: PR #318, HEAD `440f92c7`, checks CLEAN (5/5),
  strategy=merge-commit (P-009), approval source=dark-mode activation record
  (in-scope 112-S), no admin fallback used (`NORMAL_MERGE_READY`).
- `DARK_MODE_COMPLETE`: merge commit `899b70e2` on `main`; shipment 112-S
  archived (`shipped`); all 6 member items (`131-F`, `131.001-T` through
  `131.005-T`) archived; reconcile pre+post PASS; no follow-up items from
  this shipment. Release unit declared closed on closure-PR merge.
