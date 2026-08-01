---
chunk_strategy: h1-h2-h3
description: 'Post-merge operational closure for shipment 115-S (feature 133-F — core.ShipShipment stops over-archiving covering features on partial-feature shipments, plus a Thread C deterministic-restore-order fix). PR #327 merged as merge commit 47dfcc93698a6b0b2c5420c701c365a538895580 after 4 rounds of Copilot review convergence (0 unresolved threads), §1.9 pre-merge readiness gate PASS, and mandatory local review readiness READY_WITH_FOLLOWUPS (0 P0/P1, 1 non-blocking P2). CI: 6/6 required checks green. Runtime verification: PASS via a compiled-binary dogfood test proving both the core fix and the Thread C ordering fix end-to-end. Shipment archival: 115-S shipped via the native cascade (archived_ids matches manifest + shipment record exactly, returned_ids empty); pre/post shipment-reconcile both PROCEED; P-007 archive integrity intact. Deployment path: merge-only (Go CLI source fix; no live service). Residual risk: the installed C:\Tools\backlogit.exe (v1.7.0, commit 7daf8c3) predates this fix by 9 days, so real backlog operations via that pinned binary do not yet benefit from it for any future partial-feature shipment with a real covering-feature hierarchy. Follow-ups: a P2 doc-comment wording nit and a pre-existing Links-data-loss issue (Thread A) are recorded here per dark-mode scope (not stashed).'
doc_type: closure
docline:
    ms.date: 2026-08-01T00:00:00Z
    ms.topic: reference
schema_version: "1.0"
source: docs/closure/2026-08-01-133-shipshipment-cascade-fix-closure.md
title: "115-S ShipShipment cascade fix — Post-Merge Operational Closure"
---

# Operational Closure — 115-S ShipShipment cascade fix (feature 133-F)

**Mode**: post-merge
**Shipment**: 115-S — `core.ShipShipment` stops over-archiving covering
features on partial-feature shipments
**Feature**: 133-F · **Tasks**: 133.002-T, 133.003-T, 133.004-T, 133.005-T,
133.006-T
**Merge**: PR #327, merge commit `47dfcc93698a6b0b2c5420c701c365a538895580`
(reviewed/merged HEAD `b8ee9a08f80293fec620d69289f705009250a7ff`), confirmed an
ancestor of `origin/main` via `git merge-base --is-ancestor`.
**Staging precursor**: PR #326, merge commit
`35bcc9dc35b2e4079908ed102d3ca081359101a7` (landed the pre-existing local Stage
commit `4518b71d` — feature/shipment planning artifacts — before shipment
intake).
**Nature**: Go source fix (core library + CLI-surfaced behavior), test-only
and doc/policy additions; no schema migration, no external API, no service
deployment.

## Release Readiness

**READY.** All gates cleared before merge, re-verified independently at
closure:

- **CI**: 6/6 required checks green at reviewed HEAD `b8ee9a08`.
- **Local review readiness** (P-014 authoritative signal, `code-review`
  sub-agent covering the full diff): `READY_WITH_FOLLOWUPS` — 0 P0/P1, 1
  non-blocking P2 (doc-comment wording on the `restored` flag's retry
  semantics; not fixed pre-merge per circuit-breaker "accept remaining, move
  on" guidance since it does not affect behavior).
- **Copilot review**: 4 rounds to convergence. Round 1–3 found and fixed 4
  threads (context-cancellation defer-fallback ordering, partial-archive-error
  ID loss, a nested-feature-already-archived-by-same-ship exclusion, and the
  P-015 subtree-exception documentation clarification); Round 4 (Thread C)
  found and fixed the non-deterministic map-iteration restore-order bug via
  TDD (8-independent-pair regression test, RED 5/8 pre-fix, GREEN ×3 post-fix,
  commit `b8ee9a08`). Zero unresolved threads at merge.
- **§1.9 pre-merge readiness gate**: PASS — no pending Copilot review request,
  latest review `commit.oid` matched HEAD exactly, zero unresolved
  Copilot-authored threads.
- **Merge strategy**: `--merge` (merge commit only), confirmed against repo
  settings disallowing squash/rebase (P-009).
- **Runtime verification**: **PASS** (see
  `docs/closure/2026-08-01-133-shipshipment-cascade-fix-runtime-verification.md`).
  Compiled-binary dogfood test against a disposable scratch workspace proved
  both invariants end-to-end: the explicit manifest member archived; both
  non-member covering-feature ancestors (2-level nested hierarchy) restored to
  their pre-ship `active` status and left in queue; `doctor` reported no
  issues.
- **Shipment archival (Step 6.1)**: `backlogit shipment ship 115-S --sha
  47dfcc93698a6b0b2c5420c701c365a538895580` succeeded. `archived_ids`:
  `133.002-T, 133.003-T, 133.004-T, 133.005-T, 133.006-T, 115-S, 133-F` (7 —
  exactly the 6 manifest items + the shipment record). `returned_ids`: `[]`.
  Pre-mode reconcile (`.backlogit/reconcile/115-S-pre-20260801T195600Z.md`):
  PROCEED. Post-mode reconcile
  (`.backlogit/reconcile/115-S-post-20260801T195800Z.md`): PROCEED — all 7
  archive files present with correct content, no P-007 deletions.
- **Process-gap catch-up**: individual per-task "done" completion (Step
  4.5.2) had been skipped during the build loop for all 5 tasks + the
  feature; remediated on the post-merge closure branch before shipment
  archival by marking all 6 items done (accurate bookkeeping catch-up — the
  underlying work was already merged, tested, and reviewed — not fabricated
  work). See the pre-mode reconcile report for detail.

## Pre-Deploy Audit

- No feature flags, migrations, or external rollout gates apply — this is a
  Go source-code behavior fix inside a CLI/library, released by normal build
  and (eventually) tagged release, not a live deployment.
- No config or access changes.
- No cross-service dependents — `backlogit` is a standalone CLI/MCP server;
  the fix is entirely internal to `internal/core`.
- Monitoring plan (below) is complete.

## Deployment / Rollout Path

**Merge-only.** There is no live service to deploy to. The fix becomes
available to any consumer that builds or installs a `backlogit` release
containing commit `47dfcc93` or later. See the important **residual risk**
below: this repository's own day-to-day backlog operations run through a
**separately pinned, pre-built release binary** (`C:\Tools\backlogit.exe`),
not a rebuild-per-operation workflow, so the fix does not automatically become
operative for this repository's own dogfooding until that binary is upgraded.

## Post-Deploy Checks

1. `git merge-base --is-ancestor 47dfcc93... origin/main` — confirmed
   (MERGE_CONFIRMED gate, already run at Step 6 entry).
2. `go test ./...`, `go vet ./...`, and `golangci-lint run` all pass on `main`
   at the merge commit (inherited from the pre-merge quality gates; no
   regressions expected since no further source changes landed after merge).
   `gofmt -l .` flags the two touched `internal/core` files as a pre-existing,
   repo-wide Windows CRLF line-ending checkout artifact (both files are 100%
   CRLF internally; not new drift introduced by this change) — per PR #327's
   own testing record, this is a known false positive and the Linux/LF CI
   gate is authoritative, not a gate failure grouped with the passing checks
   above.
3. `backlogit doctor` (via the compiled dogfood binary) reports no issues
   after a ship operation exercising the fixed code path.
4. `backlogit version` on the installed `C:\Tools\backlogit.exe` — confirms
   current pin is `1.7.0` / `7daf8c3`, predating the fix; tracked as a
   residual/follow-up risk (below), not a closure blocker for 115-S itself
   since 133-F's own topology has no covering-feature ancestor to protect.

## Risky Action Record (strict-safety)

| # | ProposedAction | ActionRisk | ActionResult |
|---|---|---|---|
| 1 | Preserve 3 pre-existing dirty config files in named stash `dark-factory-preexisting-20260731` | moderate | `applied` — stash created, confirmed present throughout, left un-popped per instructions |
| 2 | Push Stage commit to `chore/stage-115-S`, open PR #326, normal-merge | high | `applied` — merged `35bcc9dc`, staging artifacts confirmed on `origin/main` |
| 3 | Claim/execute shipment 115-S on `feat/133-shipshipment-cascade-fix`: TDD harness, build loop, quality gates, local review, PR #327, CI/review remediation, normal merge | high | `applied` — merged `47dfcc93`, reviewed HEAD `b8ee9a08` |
| 4 | Post-merge closure branch/PR (this document + reconcile reports + shipment archival + compound-refresh) | high | `applied` (branch/commits); PR creation/merge — pending, tracked as the next step in this same closure |
| 5 | Admin fallback / destructive action | n/a | **not attempted** — not authorized under this dark-mode activation; no branch-protection rejection or other trigger condition arose |

## Invariants Preserved

- Archive-only discipline: all `.backlogit/` moves this session were
  git-detected renames (queue→archive), zero deletions (P-007 verified via
  `git status --short -- ".backlogit/archive/"`).
- P-016 single-worktree topology: exactly one worktree, one active branch at a
  time, throughout (staging → feature → post-merge closure), reconfirmed at
  each transition.
- Merge-commit-only (P-009): both PR #326 and PR #327 merged with `--merge`;
  no squash/rebase/admin-bypass used or needed.
- Named stash `dark-factory-preexisting-20260731` preservation: confirmed
  present and untouched at each checkpoint (re-confirmed again at the end of
  this closure — see Final Verification below).
- Orphan exclusion (P-015 protected-set discipline extended to reconcile):
  `133.001-T` (archived during Stage, not a shipment manifest member) was
  never referenced by any move/archive/reconcile operation this session —
  confirmed absent from `archived_ids`.

## Monitoring Plan

- **SLI**: `backlogit doctor` clean run (0 findings) on any workspace after a
  `ShipShipment` call involving a covering-feature hierarchy.
- **Dashboard/query**: no live dashboard; the observable signal is CI's
  `go test ./...` (includes the permanent regression test
  `TestRestoreRolledUpNonMemberFeatures_RestoresDeepestFirstRegardlessOfMapOrder`)
  plus the `doctor --check-over-archived-features` check-only audit added
  alongside this fix (cross-references shipment manifests against
  `returned_to_backlog` event provenance).
- **Baseline**: prior to this fix, any partial-feature shipment with a
  covering-feature ancestor would over-archive that ancestor 100% of the time
  (deterministic bug, not probabilistic). Post-fix baseline: 0 occurrences
  across the unit test suite and the compiled-binary dogfood test.
- **Alert threshold**: any `doctor --check-over-archived-features` finding, or
  any future shipment's `shipment ship` response showing a non-member
  covering feature in `archived_ids`, is a **candidate** regression. Before
  escalating, confirm the archived feature is not a legitimate descendant of
  an explicit shipment-manifest member re-parented under it via `AdoptItem`
  (the documented subtree exception in `ShipShipment`, review-fix 133.004-T:
  `featureScopeRoots` can capture such a feature as "non-member" for snapshot
  purposes even though `collectArchiveCandidateIDs` correctly archives it as a
  genuine descendant of the explicit member). Only findings that do not match
  this subtree exception are true regressions — investigate those
  immediately.
- **Owner**: the Ship agent (this session) / repository maintainer
  (`softwaresalt`) for any future manual `doctor` runs against real backlog
  state.

## Healthy / Failure Signals

- **Healthy**: `go test ./...` green; `doctor` clean; a partial-feature
  shipment's `archived_ids` contains only its own manifest items + shipment
  record, `returned_ids` empty or correctly listing genuinely-returned
  siblings; non-member covering features remain in `.backlogit/queue/` at
  their pre-ship status.
- **Failure**: `doctor --check-over-archived-features` reports a finding that
  is not explained by the subtree exception (below); a non-member covering
  feature (with no explicit-member ancestor of its own) disappears from
  `.backlogit/queue/` after a `shipment ship` call; the regression test suite
  fails.

## Rollback Trigger and Procedure

- **Trigger**: `doctor --check-over-archived-features` (or manual inspection)
  finds a covering feature archived that was not an explicit shipment
  manifest member **and is not a legitimate descendant of one via
  `AdoptItem` re-parenting** (the subtree exception noted under Monitoring
  Plan above), on any repository state at or after `47dfcc93`.
- **Procedure**: `git revert -m 1 47dfcc93...` (the full SHA)
  (subject to the required operator approval) on `main` — `47dfcc93` is a
  **merge commit**, so a plain `git revert 47dfcc93` fails without `-m 1` to
  select the mainline parent; alternatively revert the specific offending
  commit within it — then rebuild/reissue the CLI, and restore any
  incorrectly-archived artifact via `git restore`/`git revert` on
  `.backlogit/` per the existing P-015 git-revert-on-cascade procedure. Fully
  reversible — no data migration, no external state, all changes are
  git-tracked file moves and Go source.

## Validation Window

- **Immediate**: the compiled-binary dogfood test (already executed this
  session) plus the permanent regression test suite (executed on every
  `go test ./...` run, including every future CI run on `main`).
- **Ongoing**: every future shipment closure processed by the Ship agent
  through Step 6.1 acts as a live validation instance; the pre/post
  shipment-reconcile gate is the per-shipment checkpoint.
- **Owner**: Ship agent (each future closure), repository maintainer
  (`softwaresalt`) for any out-of-band `doctor` audits.

## Source Artifact Cleanup

- `133-F` `custom_fields`: `{"harness_status":"pending"}` — **no**
  `source_stash_id` or `source_deliberation_id` key present. Per Ship Step 6.7
  / operational-closure Step 5, source-artifact retirement runs only off
  those two `custom_fields` keys; neither is present, so **no stash or
  deliberation retirement was performed** — skipped and logged here, per
  protocol, rather than heuristically inferring provenance from `labels`
  (`C0909DB5`) or `references` (a decision doc and an exec-plan doc), which
  predate this `custom_fields` convention and are out of scope for this
  literal cleanup step.

## Knowledge Graduation

- **Compound refresh** (`docs/closure/2026-08-01-115-S-compound-refresh.md`):
  - **update** —
    `docs/compound/2026-07-31-p015-single-artifact-safe-close-for-partial-feature-shipments.md`:
    recorded that the root-cause fix it called for has shipped; the manual
    safe-close procedure is now a defense-in-depth fallback per the already-
    updated P-015 policy text, not the mandatory-only path.
  - **new** —
    `docs/compound/2026-08-01-self-hosted-cli-version-skew-merged-fix-not-yet-operative.md`:
    a merged fix to a self-hosted tool's own source does not protect real
    operations through an already-installed, separately-pinned release binary
    of that tool until the binary itself is upgraded.
  - **new** —
    `docs/compound/2026-08-01-n-independent-pair-test-design-for-go-map-iteration-nondeterminism.md`:
    an N-independent-pair test design (N=8 here) reliably exposes Go
    map-iteration-order bugs that single-pair tests only catch ~50% of the
    time.
- No `docs/ARCHITECTURE.md`, `AGENTS.md`, or product-spec changes warranted —
  this is an internal correctness fix to already-documented lifecycle
  behavior; P-015 policy text was already updated as part of the shipped work
  itself (commits `c444d5ae`, `73301353`, both merged in PR #327).

## Follow-ups

Per this release unit's explicit dark-mode scope constraint, the following are
recorded here as Ship's required closure output and reported to the
Orchestrator — **not** created as new stash/backlog entries:

1. **(P2, non-blocking)** `shipment_lifecycle.go`'s `restored` flag doc
   comment could read as implying more aggressive retry semantics than
   implemented. Documentation-only; no behavior risk.
2. **(Deferred, pre-existing, systemic)** `loadArtifact` can silently drop
   `Links` data on certain read paths (Copilot Thread A on PR #327). Confirmed
   pre-existing and out of 133-F's scope; deferred with rationale recorded on
   the PR #327 review thread.
3. **(Operational risk, highest priority of the three)** The installed
   `C:\Tools\backlogit.exe` (`v1.7.0`, commit `7daf8c3`, built
   `2026-07-23T22:32:43Z`) predates this fix (`47dfcc93`, `2026-08-01`) by 9
   days. Any future partial-feature shipment with a real covering-feature
   ancestor, closed through this pinned CLI before it is upgraded, remains
   exposed to the original over-archiving bug. Recommend cutting and
   installing a new `backlogit` release at or after `47dfcc93` before closing
   the next partial-feature-shaped shipment.
