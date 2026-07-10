---
chunk_strategy: h1-h2-h3
description: 'Post-merge operational closure for shipment 074-S — doctor --target scope-vs-io fault classification (PR #162, merge f2bdb7a6). Records the confirmed merge (operator P-014 admin merge-commit under repo-owner authority; standard merge blocked by the PR-Review branch-protection ruleset requiring a formal approving review; merge SHA an ancestor of origin/main), the shipment ship result (074-S shipped; 074-F, 074.001-T, 074-S archived with the merge SHA recorded; pre/post shipment-reconcile both PROCEED; P-007 archive integrity intact with no spurious deletions), release-readiness SHIPPED, no monitoring and git-revert rollback for the zero-blast-radius, exit-code-neutral diagnostic-quality fix, source-artifact cleanup (source stash 6B2C2E53 already archived/retired by Stage during harvest with a forward-link to 074-F/074.001-T/074-S; automated Step 6.7 retirement a no-op because 074-F carries no structured source_stash_id custom field — Stage-domain, flagged only), knowledge graduation (NONE — routine, low-novelty error-classification fix reusing the existing (ok, err) two-channel contract; distinct from the already-closed nil-precondition-fail-open family; the 071-S PR#156 review-follow-up arc J+K is now fully closed with K shipped as 072-S), compact-context assessment, and the three carried-forward low-priority follow-up stash entries (21E17BFC, D070FD3C, 9140F65C) for Stage.'
doc_type: closure
docline:
    ms.date: 2026-07-02T00:00:00Z
    ms.topic: reference
ingested_at: "2026-07-02T00:00:00Z"
schema_version: "1.0"
source: docs/closure/2026-07-02-074-S-doctor-target-scope-io-classification-post-merge-closure.md
title: 074-S doctor --target scope-vs-io classification — Post-Merge Operational Closure
---

# Operational Closure — 074-S doctor --target scope-vs-io fault classification (post-merge)

- **Shipment**: `074-S` · Feature `074-F` · Task `074.001-T`
- **PR**: #162 (`feat/074-doctor-target-scope-io-classification` → `main`)
- **Merge commit**: `f2bdb7a6711c46326720026d3ff0bc6f822ece1e`
- **Merged**: 2026-07-02T22:00:22Z by `softwaresalt` (Derek Williams)
- **Closure branch**: `post-merge/074-doctor-target-scope-io`

## Merge

- Merge method: **merge commit** (P-009 preserved). Standard `gh pr merge 162 --merge`
  was blocked by the `PR-Review` branch-protection ruleset
  (`required_approving_review_count: 1`, `allowed_merge_methods: ["merge"]`) because no
  formal approving review exists — only a `COMMENTED` Copilot review.
- Resolved via operator-authorized `gh pr merge 162 --merge --admin` under the
  operator's explicit **P-014** merge approval and repo-owner authority — **still a
  merge commit** (no squash, no rebase).
- **Merge Confirmation Gate**: `state: MERGED`; merge SHA `f2bdb7a…` verified as an
  ancestor of `origin/main` (`git merge-base --is-ancestor` → exit 0). Remote
  `origin/main` advanced `5aa1c2c..f2bdb7a`.

## §1.9 readiness (re-confirmed at HEAD 7243db0 before merge)

- HEAD unchanged at `7243db0b27563df46457d35c8f6437ec2db8879d`.
- 0 pending Copilot review requests (`reviewRequests.nodes: []`).
- Latest Copilot review (`COMMENTED`, 2026-07-02T21:51:12Z) covers current HEAD
  `7243db0`.
- 0 unresolved review threads (`reviewThreads.nodes: []`, `hasNextPage: false` — fully
  paginated, fail-closed).

## P-009 merge strategy verification

- Repo settings: `allow_merge_commit: true`, `allow_squash_merge: false`,
  `allow_rebase_merge: false`.
- Ruleset `PR-Review` (id 14767379): `allowed_merge_methods: ["merge"]`.
- Merge commit is the only permitted strategy — no P-009 violation.

## Scope of the shipped change

Reclassified `doctor --target` path-resolution faults so diagnostic text is no longer
dropped. In `PrepareDoctorTarget` (`internal/core/doctor_target.go`), a
`confineToStorageRoot` **path-resolution error** (`err != nil`) is now classified
`DoctorTargetIO` with the underlying wrapped error preserved in `res.Message`, instead
of the previous `DoctorTargetScope` with dropped text. Genuine **containment
violations** (`ok == false`, including the 071-S `!pathContained` symlink-escape guard)
remain `DoctorTargetScope`.

- Mechanism: reuse the existing `(ok, err)` two-channel contract of
  `confineToStorageRoot` — non-nil `err` = IO/resolution fault, `ok == false` =
  containment violation. **No sentinel error** introduced.
- Testability: an unexported boundary seam `var confineFn = confineToStorageRoot`
  lets a single non-parallel, `defer`-restored test force the resolution-error path.
- **Exit-code-neutral**: both `scope` and `io` map to exit `3`
  (`doctorTargetExitCode` unchanged); **no** `DoctorTargetResult` schema bump; **no**
  new kind. Production behavior is byte-for-byte unchanged (seam default =
  `confineToStorageRoot`).
- Single core-level fix inherited by both CLI (`runDoctorTargetMode`) and MCP
  (`handleDoctor`) surfaces.
- Files (2): `internal/core/doctor_target.go`, `internal/core/doctor_target_test.go`.
  Task commit `70cc219fe0626c4860cab66d04cced9c7e45c203`.

## Shipment ship

- `backlogit shipment ship 074-S --sha f2bdb7a…` → `shipment_status: shipped`.
- `archived_ids`: `074.001-T`, `074-F`, `074-S` (3). `returned_ids`: none.
- Archived artifacts carry `status: archived` with `commit: f2bdb7a…`
  (074-S `archived_status: shipped`; 074-F, 074.001-T `archived_status: done`).
- **Reconcile**: pre-mode (`expected: done`) → **PROCEED** (both manifest items
  pre-archived — `074.001-T` archived during build; `074-F` moved to `done` on the
  closure branch before the gate, routing to archive); post-mode → **PROCEED** (all
  three archive files present, no spurious deletions).
  Reports: `.backlogit/reconcile/074-S-pre-20260702T150302.md`,
  `.backlogit/reconcile/074-S-post-20260702T150430.md`.
- **P-007**: `git status -- .backlogit/archive/` shows no archive deletions; only the
  intended `queue → archive` moves of `074-F.md` and `074-S.md` (which appear as `D`
  under `.backlogit/queue/`, not `.backlogit/archive/`) and an `M` on the pre-existing
  archived `074.001-T.md` (ship stamped `commit`/`archived_status`). No restore needed.

## Release readiness

**SHIPPED** — merged, archived, and reconciled. Whole-repo quality gates passed
pre-merge (CI 4/4 green at HEAD `7243db0`: CLI Reference Drift, Docline frontmatter
gate, test 1.23, test 1.24); no code change on the closure branch (docs + backlog
archival only).

## Monitoring

None required. The change is an exit-code-neutral diagnostic-quality improvement on an
existing internal error-classification branch. It adds no new runtime surface,
telemetry, exit code, schema field, or external contract — it only preserves the
underlying resolution-error text (`kind=io`) that was previously dropped (`kind=scope`).

## Rollback

`git revert f2bdb7a6711c46326720026d3ff0bc6f822ece1e` (single merge commit; isolated to
`internal/core/doctor_target.go` + its test file). Zero data-migration, schema, or
exit-code impact.

## Source artifact cleanup

- **Source stash `6B2C2E53`** (071-S PR#156 Copilot follow-up J): **already archived /
  retired** by Stage during harvest — present in `.backlogit/archive/stash.jsonl`
  (`reason: archived`, `archived_at: 2026-07-02T20:23:06Z`) with a forward-link embedded
  in its text (`HARVESTED 2026-07-02 -> feature 074-F, task 074.001-T, shipment 074-S
  (queued); plan docs/exec-plans/2026-07-02-doctor-target-scope-io-classification-plan.md`).
  Confirmed absent from the active stash (`backlogit get 6B2C2E53` → not found;
  `backlogit stash list` does not include it).
- **Automated Step 6.7 retirement**: no-op — feature `074-F` carries no structured
  `custom_fields.source_stash_id` (only `harness_status: pending`), so the
  structured-field retirement path does not fire (same situation as 071-F / 072-F /
  073-F). Stash retirement is Stage-domain and already complete; flagged only, not
  forced.
- **Deliberation artifact**: none referenced by `074-F` beyond the exec-plan
  `docs/exec-plans/2026-07-02-doctor-target-scope-io-classification-plan.md`; no
  `source_deliberation_id` → nothing to archive.
- **Archived source IDs**: `6B2C2E53` (by Stage, pre-confirmed). **Skipped**: none.

## Knowledge graduation

**NONE — no new compound doc, no reinforcement.** 074-S is a routine, low-novelty
error-classification fix that reuses Go's existing `(ok, err)` two-channel contract to
distinguish an IO/path-resolution fault from a containment violation and preserve
diagnostic text. This introduces no new durable, reusable engineering lesson.

- It is **distinct from the nil-precondition-fail-open recurrence family**
  (`docs/compound/best-practices/exported-cache-zero-value-bypass-2026-06-29.md`), which
  was already declared **CLOSED** by 073-S — 074-S changes diagnostic quality, not a
  fail-open/fail-closed contract, so that doc is untouched.
- The thematically nearest existing learning
  (`docs/compound/best-practices/empty-string-vs-sentinel-in-classification-2026-05-09.md`)
  concerns data-layer classification **strings** (empty-string vs sentinel discipline),
  a different mechanism from 074-S's `(ok, err)` error-category channel; forcing a
  reinforcement there would dilute its focus. No update warranted.
- **071-S PR#156 review-follow-up arc CLOSED**: the two accepted post-merge follow-ups
  from the 071-S review are now both shipped — follow-up **K** shipped as **072-S**
  (doctor `--target` nil-HeaderDef) and follow-up **J** shipped here as **074-S**
  (scope-vs-io classification). No outstanding 071-S review follow-ups remain.

## Compact-context

Assessed (`target: all`); **no compaction executed this cycle**. `docs/memory/`
top-level volume remains far below the compaction triggers (40 files / 500 KB /
10 checkpoints per group). The 074-S memory checkpoint
(`docs/archive/memory/2026-07-02-ship-074-S-scope-io-classification.md`) is the newest per-unit
record and is preserved; prior shipped units are already covered by their own closures
and the existing `docs/memory/compacted/` batch. Archive-only and newest-preserved
constraints honored (nothing deleted or moved).

## Backlog integrity

- `backlogit sync` → indexed 664 artifacts.
- `backlogit doctor` → **1 issue**: `[orphaned_artifact] 016.001-R` — a **known,
  pre-existing, unrelated** orphan (review artifact with no parent_id). **No new
  orphans or duplicates introduced by 074-S.**

## Follow-ups carried forward to Stage

No new follow-ups were generated by 074-S post-merge closure. The following three
low-priority stash entries remain in the active stash for Stage to triage:
`21E17BFC` (singleton MCP server contingency), `D070FD3C` (surface covering-feature ID
in shipment views), `9140F65C` (npm publishing in the Release workflow).
(The previously-carried `6B2C2E53` was harvested into 074-S and is now retired.)

## Verdict

**SHIPPED** — 074-S is merged, archived, and reconciled. No knowledge graduation
warranted (routine, low novelty); the 071-S PR#156 review-follow-up arc (J + K) is now
fully closed. Source stash `6B2C2E53` retired by Stage; three low-priority stash entries
carried forward. Remaining: operator approval of the closure PR before it is merged.
