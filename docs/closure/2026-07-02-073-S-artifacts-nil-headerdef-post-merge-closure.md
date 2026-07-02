---
chunk_strategy: h1-h2-h3
description: 'Post-merge operational closure for shipment 073-S — create/update write-path nil-HeaderDef fail-closed hardening (PR #160, merge 00b9b1de). Records the confirmed merge (operator P-014 admin merge-commit under repo-owner authority; standard merge blocked by the PR-Review branch-protection ruleset requiring a formal approving review; merge SHA an ancestor of origin/main), the shipment ship result (073-S shipped; 073-F, 073.001-T, 073-S archived with the merge SHA recorded; pre/post shipment-reconcile both PROCEED; P-007 archive integrity intact with no spurious deletions), release-readiness SHIPPED, no monitoring and git-revert rollback for the zero-blast-radius defensive fail-closed fix, source-artifact cleanup (source stash 266816CE already archived/retired by Stage during harvest; automated Step 6.7 retirement a no-op because 073-F carries no structured source_stash_id custom field — Stage-domain, flagged only), knowledge graduation (reinforced the existing exported-cache-zero-value-bypass compound learning as the 3rd and final instance, closing the nil-precondition-fail-open recurrence family, rather than creating a duplicate), compact-context assessment (no compaction executed — volumes far below thresholds, newest artifacts preserved), and the four carried-forward low-priority follow-up stash entries (21E17BFC, D070FD3C, 9140F65C, 6B2C2E53) for Stage.'
doc_type: closure
docline:
    ms.date: 2026-07-02T00:00:00Z
    ms.topic: reference
ingested_at: "2026-07-02T00:00:00Z"
schema_version: "1.0"
source: docs/closure/2026-07-02-073-S-artifacts-nil-headerdef-post-merge-closure.md
title: 073-S create/update write-path nil-HeaderDef — Post-Merge Operational Closure
---

# Operational Closure — 073-S create/update write-path nil-HeaderDef hardening (post-merge)

- **Shipment**: `073-S` · Feature `073-F` · Task `073.001-T`
- **PR**: #160 (`feat/073-artifacts-write-nil-headerdef` → `main`)
- **Merge commit**: `00b9b1de4fa29b3776788df280fc8f75a648d04c`
- **Merged**: 2026-07-02T19:25:46Z by `softwaresalt` (Derek Williams)
- **Closure branch**: `post-merge/073-artifacts-write-nil-headerdef`

## Merge

- Merge method: **merge commit** (P-009 preserved). Standard `gh pr merge 160 --merge`
  was blocked by the `PR-Review` branch-protection ruleset
  (`required_approving_review_count: 1`, `allowed_merge_methods: ["merge"]`) because no
  formal approving review exists — only `COMMENTED` reviews (Copilot + operator).
- Resolved via operator-authorized `gh pr merge 160 --merge --admin` under the
  operator's explicit **P-014** merge approval and repo-owner authority — **still a
  merge commit** (no squash, no rebase).
- **Merge Confirmation Gate**: `state: MERGED`; merge SHA `00b9b1de…` verified as an
  ancestor of `origin/main` (`git merge-base --is-ancestor` → exit 0). Local `main`
  fast-forwarded `1853043..00b9b1d`.

## §1.9 readiness (re-confirmed at HEAD 8ff6b6a before merge)

- HEAD unchanged at `8ff6b6a21ebe093c603b3dfbafc7df06cd0ca22e`.
- 0 pending Copilot review requests (`reviewRequests.nodes: []`).
- Latest Copilot review (2026-07-02T19:10:56Z) covers current HEAD `8ff6b6a`.
- 0 unresolved review threads — the single thread (form-feed control char in
  `073.001-T` task body) is `isResolved: true`; `hasNextPage: false` (fully paginated).

## P-009 merge strategy verification

- Repo settings: `allow_merge_commit: true`, `allow_squash_merge: false`,
  `allow_rebase_merge: false`.
- Ruleset `PR-Review` (id 14767379): `allowed_merge_methods: ["merge"]`.
- Merge commit is the only permitted strategy — no P-009 violation.

## Scope of the shipped change

Hardened the create/update artifact write paths so a `nil` workspace `HeaderDef` no
longer silently skips required-field validation (a fail-open defect). A single shared
helper `requireHeaderDef(ws)` (`internal/core/artifacts.go:116`) returns an
`ErrConfig`-wrapped error when `ws.HeaderDef == nil`:

```go
fmt.Errorf("header definition not loaded; cannot validate artifact fields: %w", blerrors.ErrConfig)
```

The helper is called at both write sites **before**
`ApplyFieldDefaults`/`ValidateArtifactFields`:

- `CreateArtifact` (call site `internal/core/artifacts.go:253`)
- `UpdateArtifact` (call site `internal/core/artifacts.go:546`)

The pre-validation ordering is a load-bearing invariant (both downstream calls invoke
`headerDef.ResolveFieldSchema`, which dereferences the nil receiver with no nil-guard
and would panic). Wrapping `ErrConfig` maps the fault to **MCP internal (500) /
non-zero CLI exit**, never `validation_failed` (422) — a system/config fault, not user
input — and both CLI and MCP inherit the fix through the single shared core functions.
Tests: `internal/core/artifacts_headerdef_test.go` (3 scenarios — create fail-closed
with no file persisted; update fail-closed with on-disk title unchanged; loaded-path
create+update regression).

## Shipment ship

- `backlogit shipment ship 073-S --sha 00b9b1de…` → `shipment_status: shipped`.
- `archived_ids`: `073.001-T`, `073-F`, `073-S` (3). `returned_ids`: none.
- Archived artifacts carry `status: archived` with `commit: 00b9b1de…`
  (073-S `archived_status: shipped`; 073-F, 073.001-T `archived_status: done`).
- **Reconcile**: pre-mode (`expected: done`) → **PROCEED** (both manifest items
  pre-archived — `073.001-T` archived during build; `073-F` moved to `done` on the
  closure branch before the gate, routing to archive); post-mode → **PROCEED** (all
  archive files present, no spurious deletions).
  Reports: `.backlogit/reconcile/073-S-pre-2026-07-02T122914.md`,
  `.backlogit/reconcile/073-S-post-2026-07-02T122945.md`.
- **P-007**: `git status -- .backlogit/archive/` shows no archive deletions; only the
  intended `queue → archive` moves of `073-F.md` and `073-S.md` (which appear as `D`
  under `.backlogit/queue/`, not `.backlogit/archive/`) and an `M` on the pre-existing
  archived `073.001-T.md`. No restore needed.

## Release readiness

**SHIPPED** — merged, archived, and reconciled. Whole-repo quality gates
(`go test ./...`, `go vet ./...`, `golangci-lint run`, `gofmt -l .`) passed pre-merge
(CI 4/4 green at HEAD `8ff6b6a`); no code change on the closure branch (docs + backlog
archival only).

## Monitoring

None required. The change is a defensive fail-closed edge on an internal validation
precondition; it adds no new runtime surface, telemetry, or external contract. It only
converts a previously silent skip-and-succeed into an explicit config-fault error.

## Rollback

`git revert 00b9b1de4fa29b3776788df280fc8f75a648d04c` (single merge commit; isolated to
`internal/core/artifacts.go` + one new test file). Zero data-migration or schema impact.

## Source artifact cleanup

- **Source stash `266816CE`** (072-S PR#158 follow-up): **already archived / retired**
  by Stage during harvest — present in `.backlogit/archive/stash.jsonl`
  (`reason: archived`, `archived_at: 2026-07-02T18:34:33Z`) with a forward-link
  (`HARVESTED 2026-07-02 -> feature 073-F, task 073.001-T, shipment 073-S | plan
  docs/exec-plans/2026-07-02-artifacts-write-nil-headerdef-hardening-plan.md`).
  Confirmed absent from the active stash (`backlogit stash get 266816CE` → not found).
- **Automated Step 6.7 retirement**: no-op — feature `073-F` carries no structured
  `custom_fields.source_stash_id` (only `harness_status: pending`), so the
  structured-field retirement path does not fire (same situation as 071-F / 072-F).
  Stash retirement is Stage-domain and already complete; flagged only, not forced.
- **Deliberation artifact**: none referenced by `073-F` beyond the exec-plan
  `docs/exec-plans/2026-07-02-artifacts-write-nil-headerdef-hardening-plan.md`; no
  `source_deliberation_id` → nothing to archive.
- **Archived source IDs**: `266816CE` (by Stage, pre-confirmed). **Skipped**: none.

## Knowledge graduation

Reinforced the existing compound learning
`docs/compound/best-practices/exported-cache-zero-value-bypass-2026-06-29.md` — added a
**"Reinforcement — 073-S: 3rd instance / recurrence family CLOSED"** section rather
than creating a duplicate doc. 073-S remediates the 3rd and final site of the
nil-precondition-fail-open shape (070-S exported cache zero value → 072-S doctor
`--target` validation → 073-S create/update write paths). The `description` frontmatter
and tags were updated to record the family as fully closed; the docline frontmatter
gate (`backlogit docs lint`) passes (`valid: true`, 0 violations).

## Compact-context

Assessed (`target: all`); **no compaction executed this cycle**. `docs/memory/`
top-level is 4 files / 22 KB — far below the compaction triggers (40 files / 500 KB /
10 checkpoints per group). The two 073-S memory artifacts are the newest and are
preserved; the two 072-S artifacts were preserved by the 072-S closure one day ago and
remain the durable per-unit record; the batch `docs/memory/compacted/2026-07-02-shipped-units-068-071-compacted.md`
already covers prior shipped units. Archive-only and newest-preserved constraints
honored (nothing deleted or moved).

## Backlog integrity

- `backlogit sync` → indexed 662 artifacts.
- `backlogit doctor` → **1 issue**: `[orphaned_artifact] 016.001-R` — a **known,
  pre-existing, unrelated** orphan (review artifact with no parent_id). **No new
  orphans or duplicates introduced by 073-S.**

## Follow-ups carried forward to Stage

No new follow-ups were generated by 073-S post-merge closure. The following four
low-priority stash entries remain in the active stash for Stage to triage:
`21E17BFC`, `D070FD3C`, `9140F65C`, `6B2C2E53`.

## Verdict

**SHIPPED** — 073-S is merged, archived, and reconciled. Knowledge graduated (recurrence
family closed); source stash `266816CE` retired by Stage; four low-priority stash
entries carried forward. Remaining: operator approval of the closure PR before it is
merged.
