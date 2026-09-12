# Ship checkpoint — 148-S / 167-F — Wave 2 complete, wave 3 half-complete (10/20 tasks done)

**Timestamp**: 2026-09-12T19:15Z (session continuation, 4th session)
**Agent**: Ship (claude-sonnet-5/anthropic/high — ROUTING_DEGRADED, configured route
claude-sonnet-4.6/anthropic/high unavailable, propagated per prior session's
established directive)
**Branch**: feat/148-s-governed-archived-shipment-reconciliation-to-shipped
**Base**: main @ 4397b7f0d2e932c4168478b313cf01066753f509 (unchanged this session)
**Scope**: shipment 148-S, covering feature 167-F, DARK_MODE_ACTIVE [148-S]

**Supersedes**: `2026-09-12-ship-148-s-wave1-partial-honest-halt.md` (preserved as
trace evidence). That checkpoint deliberately deferred 167.017-T (the item-log
lock identity migration) to a dedicated session given its blast radius. This
session was that dedicated session, and it completed 167.017-T plus 9 further
tasks.

## What this session did

Resumed at Step 4.0 wave admission with 167.017-T as the sole remaining wave-1
member (167.003-T, 167.012-T, 167.021-T already done from the prior session).

### Wave 1 (converged)
1. **167.017-T** — Stable, handle-bound item-log lock (C) identity, independent
   of the swappable logs directory. New `events.ItemLogLockPath` roots the
   sidecar at `.backlogit/.locks/itemlog/<hex(itemID)>` (workspace-stable,
   never `logsDir`-derived); hex-encodes itemID (traversal-safe); no-follow/
   reparse-safe handle opens on both platforms (true `O_NOFOLLOW` on Unix,
   `FILE_FLAG_OPEN_REPARSE_POINT` + post-open attribute check on Windows);
   writes a durable identity-version marker for future migrations. Migrated
   ~14 call sites across `internal/core`, `internal/db`, `internal/mcp` to
   thread the new stable `locksRoot` parameter. Added
   `docs/design-docs/2026-09-12-item-log-lock-identity-migration-runbook.md`
   documenting the mandatory full-fleet quiescence precondition (legacy
   binaries cannot honor a runtime lease). Commits `c741d856`, `672cf206`.

### Wave 2 (converged — all 4 members)
2. **167.011-T** — `lockShipmentReconcileItemLog`: the reconcile transaction's
   own acquirer for the SAME stable sidecar 167.017-T established, opened
   directory-handle-relative (true `unix.Openat` on Unix; hardened
   reparse-safe CreateFile on Windows) — proven to mutually exclude
   `events.LockItemLogCrossProcess` in both directions, including when the
   cross-process side observes a different logs directory. Commits `d0dab89d`,
   `13836433`.
3. **167.006-T** — `writeShipmentReconcileArchiveFile`: handle-relative atomic
   archive-file writer (true `openat`/`renameat` on Unix; reparse-rejection +
   temp+rename on Windows). Seam-injectable fsync classification
   (`shipmentReconcileFSSeams`, mirrors `internal/atomicfile`) makes the
   pre-rename-`ErrWriteNotApplied` / post-rename-`ErrWriteIndeterminate`
   split deterministically testable on any host OS. Commits `2230661c`,
   `ee6e191b`.
4. **167.002-T** — `EventShipmentReconciledShipped` event type, full delta
   schema, `detectDuplicateJSONMembers` (raw token-level recursive duplicate-
   member scanner run BEFORE canonicalization — mirrors the existing
   `checkpoint_strict.go` token-walk pattern), `ValidateShipmentReconciledShippedEvent`
   (shape + prepared-event/digest comparison covering `request_identity_digest`
   and `trusted_ref_tip`), and doctor-local recognition
   (`reconciledShippedEventPresence` as an additive OR-signal that never
   weakens the existing `shippedEventPresence`/144-F guard 2). Commits
   `b1dcbbde`, `e04a0615`.
5. **167.019-T** — `guardArchivedStatusUnchangedSince`: reload/CAS-under-lock
   clobber protection wired into all 4 snapshot-before-lock writers
   (`AssociateCommit`, `AddDependency`, `RemoveDependency`, `AddArtifactLink`).
   Added `persistArtifactPreLockHook` (nil-in-production test seam) enabling
   a fully deterministic (non-sleep-based) race reproduction. Commits
   `b9addd11`, `54c9725e`.

### Wave 3 (2 of 4 members done so far)
6. **167.013-T** — Added a `windows-latest` CI job (`test-windows`) scoped to
   the reconcile lock/writer tests, gated identically to the existing ubuntu
   `test` job. Verified with `actionlint` (clean) and by running the exact new
   job command locally on this Windows dev machine. Commits `60765ec1`,
   `fb2fcf02`.
7. **167.007-T** — `appendShipmentReconcileEvent`: always-fsync durable event
   append primitive running under the caller's already-held item-log lock
   (never acquires it itself). Pre-write shape validation (no write on
   malformed input) → handle-relative append (true Unix `openat`; Windows
   reparse-safe `CreateFile` + explicit seek-to-end) → partial-trailing-line
   rejection (crash-signature detection) → exact re-read + re-validate of the
   durable bytes → `IndexEvent` with index-failure-does-not-unreconcile
   semantics. Commits `bae84a90`, `88b6b610`.

Every task followed real TDD: tests written first, observed RED against the
167.003-T panic-body declarations (or, for 167.002/167.013/167.019, against a
missing symbol / unmigrated behavior), then implemented GREEN. Every task's
commit records the exact verification performed: `go build ./...` on both
Windows (native) and Linux (`GOOS=linux` cross-compile), `golangci-lint run`
using **v1.64.8** (matching `.github/workflows/ci.yml`'s pinned version — the
locally-preinstalled v2.13.2 reports unrelated errcheck noise this repo's CI
does not flag), and a full `go test ./internal/core/...` run after every task
(300–450s each, always clean, zero regressions across the whole session).

## Current verified state (re-confirmed at end of session)

- Branch: `feat/148-s-governed-archived-shipment-reconciliation-to-shipped`,
  now **14 commits ahead** of this session's start (7 feature commits +
  7 paired backlog-state commits), still based on `main@4397b7f0`. Working
  tree clean (`git status --short` empty).
- Shipment 148-S: `active` (sole active shipment, unchanged — not reclaimed
  this session, per the operator's explicit instruction that the prior
  force-claim is already authorized/audited and must not be repeated).
- Feature 167-F: `active` (unchanged).
- Task census (re-read directly from `.backlogit/{queue,archive}/167.*-T.md`,
  exact-ID, no `list --type task` enumeration):
  **done (10)**: 167.002-T, 167.003-T, 167.006-T, 167.007-T, 167.011-T,
  167.012-T, 167.013-T, 167.017-T, 167.019-T, 167.021-T.
  **queued (10)**: 167.001-T, 167.004-T, 167.005-T, 167.008-T, 167.009-T,
  167.010-T, 167.014-T, 167.015-T, 167.016-T, 167.020-T.
  **active**: 0. **blocked**: 0. **unsupported**: 0. 167.018-T remains
  absent/retired (unchanged, matches manifest).
- `.backlogit` index resynced (`backlogit sync`) at session start; every
  subsequent `move`/`comment` mutation logged an "index may be stale" notice
  that is expected and self-resolves on the next `sync`/query.

## Wave 3 status: 2 of 4 members done, not yet converged

Wave 3's ready set (all dependencies terminal-`done`) is
**{167.001-T, 167.007-T, 167.010-T, 167.013-T, 167.016-T}**. 167.007-T and
167.013-T are now done. **167.001-T and 167.016-T remain queued** — wave 3
has NOT converged, so per P-002.6 this session did not, and must not, report
completion around it.

Both remaining wave-3 members are substantial, "high complexity" tasks I
deliberately did not rush this session, having already delivered 10 solid
task completions:

- **167.001-T** (delivery-evidence domain): git-shelling merge-commit
  verification (`git rev-list --parents`/`cat-file`, `merge-base
  --is-ancestor` against a *trusted-ref pinned tip*, `boundedHelperTimeout`),
  a domain-separated canonical request-identity digest, closure-evidence
  file handling (no-follow/reparse-safe, deterministic delivery-merge
  grammar keyed on role annotations), and the full evidence digest
  composition. This is genuinely a distinct, security-sensitive domain
  (git process invocation + trust-anchor resolution) that deserves its own
  focused session rather than being squeezed in at the end of an already
  very long one.
- **167.016-T** (snapshot/rollback primitive): full-row SQLite snapshot
  (every `items` column, not just `custom_fields`/`updated_at` — a partial
  snapshot would corrupt restore given `UpsertItem`'s `INSERT OR REPLACE`
  semantics), file restore through the 167.006-T handle-bound writer
  (never a path-based write, which would reopen the exact symlink-swap race
  167.006-T closes), a non-cascading single-row delete (never
  `db.DeleteItem`, which cascades and would destroy valid auxiliary index
  state — item logs, deps, links, stash links, commit links), and
  indeterminate-vs-restore write-outcome classification. Also genuinely
  substantial and worth a focused pass rather than a rushed one.

Downstream of these: 167.008-T (the transaction itself, consumes 167.001,
167.007, 167.010, 167.014, 167.016, 167.017, 167.019, 167.020, 167.021 —
still blocked on most of its inputs), 167.004-T (CLI), 167.005-T (integration
tests), 167.009-T, 167.014-T, 167.015-T (precondition gate), 167.020-T
(concurrency-proof harness) all remain queued and unreachable until their own
dependencies land.

**No PR was created. No branch was merged. No CI ran on this PR. No closure
ran.** Step 5 (PR lifecycle) begins only "after all tasks in the queue are
complete" — 10 of 20 manifest tasks remain queued. Reporting any Step 5/6
outcome would be false; this checkpoint reports exactly what happened.

## Recommended next step (operator / next Ship session)

1. Resume at Step 4.0 wave-3 admission. `ready_k` = {167.001-T, 167.016-T}
   (167.007-T and 167.013-T already done; 167.010-T's own dependencies
   — 167.002-T, 167.003-T — are satisfied, so it is ALSO still a valid
   wave-3 ready-set member not yet attempted — re-derive the exact
   `ready_k` from live status at resume time rather than trusting this
   list verbatim, per P-002.6 Step 4.0).
2. Give 167.001-T (git evidence) and 167.016-T (snapshot/rollback) each
   their own careful, harness-first pass; both are self-contained enough to
   attempt independently once picked up.
3. Once wave 3 converges, wave 4+ becomes reachable per the dependency graph
   already recorded in the prior superseded checkpoint and re-derivable via
   `backlogit dep list <id>` for any task.
4. Continue wave-by-wave toward the transaction (167.008-T), CLI (167.004-T),
   integration tests (167.005-T), and the blocking governance ratification
   gate (167.015-T) before Step 5 PR lifecycle can begin.

**DARK_MODE_HALTED** (honest, not a policy violation): scope `148-S`, reason
`wave 3 incomplete — 167.001-T and 167.016-T deliberately deferred after a
long, highly productive session (10/20 tasks completed this run across waves
1–3, zero regressions, full TDD discipline maintained throughout) to avoid
rushing two more genuinely substantial, security/durability-sensitive
primitives`. No PR, no CI run, no merge, no closure. No policy violation
occurred; this is a scope/session-quality boundary, reported transparently.
