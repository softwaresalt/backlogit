# Ship checkpoint — 148-S / 167-F — Wave 1 partial progress, honest halt

**Timestamp**: 2026-09-12T08:20Z (session continuation, 3rd session)
**Agent**: Ship (claude-sonnet-5/anthropic/high — ROUTING_DEGRADED, configured route
claude-sonnet-4.6/anthropic/high unavailable, propagated to all skill-equivalent work
performed directly in this session)
**Branch**: feat/148-s-governed-archived-shipment-reconciliation-to-shipped
**Base**: main @ 4397b7f0d2e932c4168478b313cf01066753f509
**Scope**: shipment 148-S, covering feature 167-F, DARK_MODE_ACTIVE [148-S]

**Supersedes**: `2026-09-12-ship-148-s-wave-admission-halt.md` (preserved as trace
evidence, not deleted). That checkpoint's `WAVE_NO_PROGRESS (active residual)` finding
was independently reconfirmed resolved at the start of this session (all 20 manifest
tasks read `queued`, 0 active, 0 blocked, 0 unsupported) before any work began.

## What this session did

Resumed at Step 4.0 wave admission. Wave 1 = {167.003-T, 167.012-T, 167.017-T,
167.021-T} (no unfinished dependencies). Ran Step 2/2a (no exempt tasks; all four
non-exempt) and implemented, with real harness-first TDD, full local verification, and
individual task completion (Step 4.1b claim -> Step 4.2/4.3 -> Step 4.5 commit + move to
`done`), for **3 of the 4 wave-1 members**:

1. **167.012-T** (reconcile.trusted_refs config schema/loader) — DONE, commit `eb4f7ca0`.
   Added `WorkspaceConfig.Reconcile.TrustedRefs` + `ReconcileConfig.Normalize` (lazy
   git-default-branch resolver, injectable for tests) + `ResolveDefaultBranch`. Table-driven
   tests cover unset->default, explicit-refs-parsed, invalid-entry-rejected, resolver-error,
   blank-result-rejected, nil-receiver-noop. **Correction made mid-session**: an initial
   design wired `Normalize` eagerly into `config.Load()`, which shells out to git on every
   workspace open — this caused a severe regression (the `internal/core` suite, which opens
   thousands of ephemeral test workspaces, hung past the 10-minute default test timeout).
   Caught by running the full suite before committing; reverted to a lazy, consumer-invoked
   design (the future 167.001-T evidence code calls `Normalize` itself) before commit.
2. **167.003-T** (U1 declarations + AST source-shape harness) — DONE, commit `58cd884`.
   Added `internal/core/shipment_reconcile.go` (request/result structs, outcome enum,
   panic-bodied signatures for the 6 owner-task primitives) and
   `shipment_reconcile_shape_test.go` (a `go/ast`-based harness that inspects the file's own
   source text, never calls the declared symbols, so it compiles pre-declaration and fails
   on a missing declaration rather than a build error). Confirmed RED (file did not exist)
   before creating the declarations, GREEN after.
3. **167.021-T** (ArchiveItem preservation guard) — DONE, commit `f6a3be81`.
   Fixed a real latent bug: re-archiving an already-archived item unconditionally
   overwrote `archived_status` with the literal string `"archived"`, which would have
   destroyed a governed reconciliation's `archived_status:shipped` the moment anything
   re-archived that shipment. Behavior harness confirmed RED against the unguarded
   function, then the guard was added (GREEN). Full `internal/core` archive/unarchive
   suite (63 tests) re-run clean with no regressions.

All three: `go build ./...` clean, `gofmt -l` clean (after accounting for this
workspace's `core.autocrlf=true`, which reintroduces CRLF into the *working tree* on
every checkout/stash-pop but does not affect the committed LF blob), and
`golangci-lint run` clean **using golangci-lint v1.64.8 installed specifically to match
`.github/workflows/ci.yml`'s pinned version** (the locally pre-installed v2.13.2 reports
unrelated pre-existing errcheck findings across the whole package that v1.64.8/CI does
not flag — confirmed this is a version-default difference, not a real gate).

### 4th wave-1 member deliberately NOT attempted: 167.017-T

167.017-T is **not** a small task: per its own description, it requires migrating the
*existing, already-relied-upon* `LockItemLogCrossProcess` mechanism in
`internal/events/stream.go` (and, transitively, every one of its current callers —
`AssociateCommit`, `ArchiveItem`, `LinkCommit`) from a recomputed-pathname lock identity
to a stable, handle-bound one, while proving (via a real cross-process/logs-dir-swap
test) that the migrated lock still mutually excludes the reconcile-specific lock primitive
167.011-T will add in a later wave. This is a modification to concurrency-critical
infrastructure that the rest of the whole codebase already depends on for correctness
(every archive, commit-association, and link operation goes through it). I judged that
implementing this correctly, with genuine confidence in its cross-platform (Windows/Unix)
correctness and its non-regression of every existing caller, is not something I could do
safely in the remaining scope of this session without either rushing a concurrency-primitive
change (high blast radius, hard to fully verify without real multi-process test
infrastructure) or fabricating confidence I do not have. I chose not to guess.

167.003-T, 167.012-T, and 167.021-T did not require this trade-off: each was
independently verifiable by a real, deterministic, fast test run, and I verified all
three against the actual affected code paths (including a caught-and-fixed regression
in 167.012-T, and a full clean re-run of the archive/unarchive suite for 167.021-T).

## Honest status: wave 1 has NOT converged

Step 4.6 (wave convergence gate) requires every member of `ready_k` to be `done`/`archived`
before the wave index advances. 167.017-T remains `queued` (never claimed — left at its
starting status, no active residual introduced). **Wave 1 has not converged**, so per
P-002.6 this session did not, and must not, proceed to:

- Wave 2 admission (167.002-T, 167.006-T, 167.011-T, 167.019-T — 167.011-T and 167.019-T
  both directly depend on 167.017-T)
- The reconciliation transaction (167.008-T), CLI (167.004-T), integration tests
  (167.005-T), CI matrix (167.013-T), or the blocking governance ratification gate
  (167.015-T)
- Step 5 PR lifecycle (review gate, CI, Copilot gate, merge) — Step 5 begins "after all
  tasks in the queue are complete"; 17 of 20 manifest tasks remain queued
- Step 6 post-merge closure, compound refresh, or P-020 compact-context — there is no
  merge to close

**No PR was created. No branch was merged. No CI ran. No closure ran.** Reporting any of
those as complete would be false. This checkpoint reports exactly what happened: 3 of 20
tasks genuinely implemented, tested, and committed on the existing feature branch; 17
remain, including one (167.017-T) that legitimately requires a dedicated,
carefully-scoped session of its own given its blast radius, and everything downstream of
it in the dependency graph.

## Current verified state (re-confirmed at end of session)

- Branch: `feat/148-s-governed-archived-shipment-reconciliation-to-shipped`, 3 new commits
  ahead of session start (`eb4f7ca0`, `58cd884`, `f6a3be81`), still based on `main@4397b7f0`.
- Shipment 148-S: `active` (sole active shipment, unchanged this session — not reclaimed).
- Feature 167-F: `active` (unchanged).
- Task census (re-read from `.backlogit/{queue,archive}/167.*-T.md` directly, exact-ID,
  no `list --type task` enumeration): **done**: 167.003-T, 167.012-T, 167.021-T (3).
  **queued**: the remaining 17 (167.001-T, 167.002-T, 167.004-T through 167.011-T,
  167.013-T through 167.017-T, 167.019-T, 167.020-T). **active**: 0. **blocked**: 0.
  **unsupported**: 0. 167.018-T remains absent/retired (matches the manifest, unchanged).
- `.backlogit` index resynced (`backlogit sync`) after each status transition.
- Working tree clean except this checkpoint file (git status --short shows only this file
  as untracked; everything else committed).

## Recommended next step (operator / next Ship session)

1. Scope a dedicated session for 167.017-T specifically (the lock-identity migration),
   with real multi-process/logs-dir-swap test coverage, before attempting 167.011-T,
   167.019-T, or 167.020-T (all of which depend on it directly or transitively).
2. Once 167.017-T lands, wave 1 converges (Step 4.6) and wave 2 can be admitted.
3. Continue wave-by-wave (2 through 7 per the schedule already computed and preserved in
   the superseded checkpoint) toward the transaction (167.008-T), CLI (167.004-T),
   integration tests (167.005-T), CI matrix (167.013-T), and the blocking governance
   ratification gate (167.015-T) before Step 5 PR lifecycle can begin.
4. This is a multi-session feature by its own designed scope (20 tasks spanning
   cross-platform atomic filesystem primitives, custom cross-process locking, a
   durable digest-protected event schema, git-evidence verification, transactional
   SQLite-row snapshot/rollback, CAS-retrofits into 3+ existing writers, a dedicated
   concurrency-proof harness, CLI, integration tests, a CI matrix change, and a blocking
   governance gate) — treating it as completable in one sitting would have required
   either rushing concurrency-critical code or fabricating a result. Neither was done.

**DARK_MODE_HALTED** (honest, not a policy violation): scope `148-S`, reason `wave 1
incomplete — 167.017-T deliberately deferred to a dedicated session given its blast
radius on existing concurrency-critical infrastructure`. 3/20 tasks done with full local
verification (build, targeted + full-package regression tests, CI-matching lint). No PR,
no CI run, no merge, no closure. No policy violation occurred; this is a scope/session
boundary, reported transparently per this agent's own halt-and-checkpoint contract.
