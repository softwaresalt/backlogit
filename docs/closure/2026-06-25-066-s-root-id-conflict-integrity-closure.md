---
chunk_strategy: h1-h2-h3
description: 'Operational closure record for shipment 066-S after feature PR #132 merged at 80ce5f12, covering archive lifecycle, monitoring via doctor audit, and rollback'
doc_type: closure
docline:
    ms.date: 2026-06-25T00:00:00Z
    ms.topic: reference
ingested_at: "2026-06-26T02:32:32Z"
schema_version: "1.0"
source: docs/closure/2026-06-25-066-s-root-id-conflict-integrity-closure.md
title: 066-S Post-Merge Closure — Root-ID Conflict Integrity
---

## Scope

Operational closure for shipment `066-S` ("Root-ID Conflict Integrity"),
hardening bug `0F65FBC9` — undetected top-level work-item root-ID conflicts
between the queue and archive surfaces.

Shipped scope (archived at merge commit `80ce5f12ef52a68feaecfb9bfdeb94f6f1f79fd3`,
PR #132):

* Shipment `066-S`
* Feature `066-F` — Root-ID Conflict Integrity: Detection, Allocation, and Archive Safety
* Task `066.001-T` — Hardened canonical root-ID-collision doctor audit + shared scanner
* Task `066.002-T` — Pre-write ID uniqueness guard in `CreateArtifact` (`ErrIDCollision`)
* Task `066.003-T` — `ArchiveItem` refuses a distinct occupied destination (`ErrArchiveDestinationOccupied`)
* Task `066.004-T` — Rehydrate duplicate-source warning (observational)
* Task `066.005-T` — End-to-end same-root-ID repro test

Files changed by the merged work: `internal/core/naming.go`, `internal/core/artifacts.go`,
`internal/core/archive.go`, `internal/db/rehydration.go`, new
`internal/core/canonical_scan.go` (+ tests). Planning artifacts:
`docs/decisions/2026-06-23-root-id-conflict-integrity-deliberation.md`,
`docs/exec-plans/2026-06-23-root-id-conflict-integrity-plan.md`.

Notable correctness fix during build: `5f86ee9d` forces `.backlogit/archive` into the
canonical scan set so a degraded/partial registry cannot blind the collision guard —
this closed the silent data-loss path.

## Outcome

| Check | Result | Evidence |
|---|---|---|
| Merge confirmation | passed | PR #132 `MERGED` at `80ce5f12...`; `git merge-base --is-ancestor` → ancestor (exit 0) |
| Closure execution base | passed | Work performed on dedicated branch `post-merge/066-root-id-integrity` cut from `origin/main` |
| Pre-ship reconcile (GI) | PROCEED | All 6 manifest items `pre-archived`; no missing/mismatch/orphan — `.backlogit/reconcile/066-S-pre-20260625-002744.md` |
| Shipment archival | passed | `066-S`, `066-F`, `066.001-T`..`066.005-T` all `status: archived`; queue copies relocated to `.backlogit/archive/` |
| Commit traceability | passed | Merge SHA `80ce5f12...` recorded in `commit_links` for all 7 artifacts |
| Archive integrity (P-007) | passed | `git status -- .backlogit/archive/` shows 0 working-tree deletions |
| Post-ship reconcile (GR) | PROCEED | All manifest items + shipment present in archive — `.backlogit/reconcile/066-S-post-20260625-003100.md` |
| Runtime verification | PASS | `backlogit doctor` clean; 066 guard tests pass fresh — see runtime-verification artifact |

## Backlog State

The backlog now reflects a closed, shipped scope:

* `066-S` moved `active` → `shipped` → `archived` (`.backlogit/archive/066-S.md`,
  `archived_status: shipped`).
* `066-F` rolled up `active` → `done` (all child tasks done) and then archived
  (`.backlogit/archive/066-F.md`, `archived_status: done`).
* `066.001-T`..`066.005-T` were already archived during build; ship attached the
  merge SHA and normalized them to `status: archived`.
* The `066-F` feature's `active` status before closure was a stale, un-persisted
  parent-rollup; `ComputeParentStatus(066-F)` resolves to `done` because all five
  children are done. The rollup correction reflected true state, not forced state.

## Invariants to Preserve

* Top-level root IDs remain unique across the entire canonical surface
  (queue + archive + routed dirs), independent of the SQLite index's freshness.
* `CreateArtifact` and `ArchiveItem` remain fail-loud on collisions; the legitimate
  same-path / half-archive recovery path must keep succeeding.
* `doctor --check-duplicates` continues to detect cross-directory root collisions
  without flagging level-2 (child ordinal) duplicates as root collisions.

## Pre-Deploy Audits

Not applicable in the deployment sense — this is a CLI/library integrity change with
no service, migration, flag, or config rollout. The equivalent pre-merge audit is the
`backlogit doctor` collision pass plus the merged guard tests, both green.

## Deployment / Rollout Path

**Merge-only.** The change ships as part of the `backlogit` binary (v1.2.0). It is
consumed wherever the CLI/MCP server is invoked; there is no separate deploy, canary,
or maintenance window. Distribution follows the normal release pipeline
(`release.yml`) when a tag is cut — out of scope for this closure.

## Healthy Signals

* `backlogit doctor` reports **No issues found** on real workspaces.
* `CreateArtifact` callers that attempt a colliding root ID receive a clear
  `ErrIDCollision` instead of silently overwriting.
* Archiving a top-level item that shares a filename with a distinct archived item is
  refused with `ErrArchiveDestinationOccupied` rather than destroying the occupant.
* Shipment ship/archive round-trips (like this very closure) complete with zero
  archive-file deletions reported by `git status`.

## Failure Signals

* `backlogit doctor` begins reporting duplicate root IDs across queue/archive.
* Users report a created or harvested artifact silently overwriting an existing one
  (the `0F65FBC9` symptom recurring).
* `ArchiveItem` raising `ErrArchiveDestinationOccupied` for the **same** logical item
  (a false positive blocking legitimate same-path / half-archive recovery).
* CI guard tests in `internal/core/066_*` or `internal/db/066_*` regress.

## Monitoring Plan

This is a CLI/library change, so monitoring is **guardrail-based**, not dashboard-based:

* **CI guardrails**: the merged `internal/core/066_*` and `internal/db/066_*` tests run
  on every PR via `ci.yml` (`test (1.23)` and `test (1.24)`). A regression fails CI.
* **Operational guardrail**: `backlogit doctor --check-duplicates` is the on-demand
  audit operators (and the Stage/Ship agents) run to detect root-ID collisions in a
  live workspace. Recommend including it in periodic backlog hygiene passes.
* No logs, metrics, or alert thresholds apply — there is no running service surface.

## Rollback Trigger

Roll back if either fires:

* The archive-overwrite refusal produces false positives that block normal shipment
  archival (legitimate same-path archival incorrectly refused), OR
* The pre-write collision guard rejects legitimate creates (e.g., a bulk-import or
  harvest flow that previously succeeded now fails with `ErrIDCollision`).

## Rollback Procedure

* Revert merge commit `80ce5f12ef52a68feaecfb9bfdeb94f6f1f79fd3` on `main` via a
  revert PR (merge-commit strategy, P-009), then rebuild the binary.
* This closure's backlog archival lives on `post-merge/066-root-id-integrity`; if the
  closure itself must be undone, revert/close that branch's PR — the shipped feature
  code is independent of the closure-branch artifacts.

## Risk Record

| ProposedAction | ActionRisk | Approval path | ActionResult |
|---|---|---|---|
| Roll feature `066-F` `active` → `done` to reflect the all-children-done rollup | low | Non-destructive backlog lifecycle; reflects computed parent status | applied |
| `backlogit shipment ship 066-S` archiving the shipped scope at the merge SHA | moderate | Operator-approved post-merge closure scope | applied |
| Commit backlog archival + closure docs on a dedicated branch (not `main`) | low | Ship Step 6.0 branch-per-release-unit | applied |

## Validation Window

One backlog hygiene cycle. Because the guards are exercised by CI on every PR and by
`doctor` on demand, no time-boxed observation window applies beyond confirming the next
few `CreateArtifact` / shipment-archival flows behave normally.

## Owner

Ship agent (this session) for closure; repository maintainer (`softwaresalt`) for the
shipped guard behavior going forward.

## Source Artifact Cleanup

Per Ship Step 6.7, source-artifact retirement is driven by `custom_fields` on the
shipped top-level items, not by heuristic search:

* `066-F.custom_fields` contains only `harness_status` — **no `source_stash_id`** and
  **no `source_deliberation_id`**. No `backlogit_stash_remove` or `backlogit_archive_item`
  mutation was performed.
* The originating bug stash `0F65FBC9` is **already absent** from the stash store (it was
  harvested during the Stage pipeline that produced `066-F`). Nothing to retire.
* The deliberation `docs/decisions/2026-06-23-root-id-conflict-integrity-deliberation.md`
  is a versioned doc (not an indexed backlog deliberation artifact); it is retained as
  durable design rationale, not archived.

Archived source artifacts: none. Skipped (already retired / not present): stash
`0F65FBC9`. No deliberation backlog artifact in scope.

## Follow-Ups (already stashed for Stage)

No **new** follow-ups were created by this closure. The follow-ups arising from the
shipped scope and its reviews are already present in the stash for Stage to triage:

* `B8FF7590` (high) — data repair: off-by-one drift in queued shipment manifests
  `060-S`/`061-S` and feature titles `061-F`/`062-F`. Split out during planning.
* `C55C5158` (medium, design-gated) — durable per-type high-water-mark counter.
* `D6B44FF6` (low) — optimize `scanCanonicalArtifacts` for bulk-create O(N^2) (066-S review P2).
* `2797E9F8` (low) — DI a `*slog.Logger` into `Rehydrate`/`warnOnDuplicateSourceIDs`
  (PR #132 Copilot review, 066.004-T).

## Knowledge Graduation

* Compound learning captured at
  `docs/compound/db-reliability/canonical-filesystem-scan-vs-index-id-allocation-2026-06-25.md`.
* Compound-refresh report at
  `docs/closure/2026-06-25-066-s-root-id-conflict-integrity-compound-refresh.md`.
* No `docs/ARCHITECTURE.md`, `AGENTS.md`, or product-spec change was required — the
  durable design rationale already lives in the deliberation and exec-plan, and the
  reusable lesson is graduated into the compound library above.

## Readiness Status

**READY** — the shipped scope is archived, traceable to the merge SHA, integrity-checked
(GI/GR + P-007), and runtime-verified. The closure artifacts are presented for review via
the post-merge closure PR; merge awaits explicit operator approval (P-014).
