---
chunk_strategy: h1-h2-h3
description: 'Post-merge operational closure for shipment 076-S — harden the Stage harvest pipeline so exec-plan docs are born-compliant with the docline frontmatter contract and a pre-harvest lint gate HALTs invalid frontmatter before it can ride into a Ship feature PR (PR #166, merge ef9dc20). Docs/harness-only change (no Go code): impl-plan/SKILL.md gains a Plan Frontmatter Contract + MANDATORY Phase 4 self-lint; harvest/SKILL.md gains a Phase 1.5 docline gate. Records the confirmed merge (operator P-014 admin merge-commit under repo-owner authority; standard merge blocked by the PR-Review branch-protection ruleset requiring a formal approving review; merge SHA an ancestor of origin/main; origin/main advanced 414063c..ef9dc20; local main fast-forwarded 8e5f89f..ef9dc20 with operator in-flux files preserved), the §1.9 readiness re-confirmation at HEAD 74db7ec (0 pending Copilot requests, latest Copilot review covers HEAD, all 9 review threads resolved after the operator accepted the 3 remaining P3 make-docs-lint --path nitpicks on ride-along artifacts as wont-fix, fully paginated fail-closed), P-009 verification (merge-only repo settings + ruleset allowed_merge_methods ["merge"]), the shipment ship result (076-S shipped; 076.001-T/.002-T, 076-F, 076-S archived with the merge SHA recorded; pre/post shipment-reconcile both PROCEED; P-007 archive integrity intact with no spurious deletions), release-readiness SHIPPED, no monitoring and git-revert rollback for the zero-blast-radius docs/harness change, runtime verification PASS (born-compliant plan lints clean scoped + repo-wide; the 075-S defect is flagged with 3 violations and exit 1), source-artifact cleanup (source stash A9D74372 already archived/retired by Stage with a forward-link; automated Step 6.7 retirement a no-op because 076-F carries no structured source_stash_id custom field — Stage-domain, flagged only), an operator-directed new follow-up stash B55985DD to reword the ride-along --path wording, knowledge graduation (a scoped 076-S reinforcement note added to the docline-frontmatter-contract compound learning recording born-compliant agent-authored plans + the Stage self-lint-against-its-own-gate pattern — no duplicate doc), compact-context assessment (no compaction executed — below all thresholds, newest artifacts preserved), and the low-priority follow-up stash entries carried forward to Stage (B55985DD, EED25928, 21E17BFC, 9140F65C, 17D29DDC).'
doc_type: closure
docline:
    ms.date: 2026-07-02T00:00:00Z
    ms.topic: reference
ingested_at: "2026-07-02T00:00:00Z"
schema_version: "1.0"
source: docs/closure/2026-07-02-076-S-harvest-docline-frontmatter-hardening-post-merge-closure.md
title: 076-S harvest docline frontmatter hardening — Post-Merge Operational Closure
---

# Operational Closure — 076-S harden Stage harvest docline frontmatter integrity (post-merge)

- **Shipment**: `076-S` · Feature `076-F` · Tasks `076.001-T`, `076.002-T`
- **PR**: #166 (`feat/076-docline-frontmatter-hardening` → `main`)
- **Merge commit**: `ef9dc20468d865bbaf7d7b1e9b982ff7f4045422`
- **Merged**: 2026-07-03T05:53:01Z by `softwaresalt` (Derek Williams)
- **Closure branch**: `post-merge/076-docline-frontmatter-hardening`

## Merge

- Merge method: **merge commit** (P-009 preserved). Standard `gh pr merge 166 --merge`
  was blocked by the `PR-Review` branch-protection ruleset
  (`allowed_merge_methods: ["merge"]`, formal approving review required) because no
  formal approving review exists — only `COMMENTED` reviews (Copilot + operator).
- Resolved via operator-authorized `gh pr merge 166 --merge --admin` under the
  operator's explicit **P-014** merge approval and repo-owner authority — **still a
  merge commit** (no squash, no rebase), same pattern as #156–#165.
- **Merge Confirmation Gate**: `state: MERGED` (`mergedAt: 2026-07-03T05:53:01Z`); merge
  SHA `ef9dc20…` verified as an ancestor of `origin/main`
  (`git merge-base --is-ancestor` → exit 0). `origin/main` advanced `414063c..ef9dc20`.
  Local `main` fast-forwarded `8e5f89f..ef9dc20` (clean ff; the four operator in-flux
  working-tree files have identical committed baselines on `origin/main` and the feature
  HEAD, so they were preserved and never staged).

## §1.9 readiness (re-confirmed at HEAD 74db7ec before merge)

- HEAD unchanged at `74db7ecc286d5811eb980c962ce7caea587b1da4`.
- 0 pending Copilot review requests (`reviewRequests.nodes: []`).
- Latest Copilot review (2026-07-03T05:06:49Z) covers current HEAD `74db7ec`.
- 0 unresolved review threads — all **9** threads `isResolved: true`; `hasNextPage: false`
  (fully paginated, fail-closed). The 3 threads still open at intake
  (`PRRT_kwDORzozKM6OF0kp`, `…0k1`, `…0lG`) were **accepted by the operator as won't-fix**
  and resolved with a documented reply referencing the tracked follow-up stash `B55985DD`.
  These are bot-authored (`copilot-pull-request-reviewer`), so auto-resolution is permitted.
- CI on PR #166 at HEAD: **4/4 green** (`test (1.23)`, `test (1.24)` required,
  `CLI Reference Drift`, `Docline frontmatter gate`).

## P-009 merge strategy verification

- Repo settings: `allow_merge_commit: true`, `allow_squash_merge: false`,
  `allow_rebase_merge: false`.
- Ruleset `PR-Review`: `allowed_merge_methods: ["merge"]`.
- Merge commit is the only permitted strategy — no P-009 violation.

## Scope of the shipped change

**Docs/harness-only** (543 insertions across 10 files; **no Go code**). Two shift-left
docline gates were added upstream of every path by which a Stage-authored plan reaches a
commit, implementing directions (1) and (2) from the plan (direction 3 + upstream `.tmpl`
drift deferred to stash `EED25928`):

- `.github/skills/impl-plan/SKILL.md` (**076.001-T**) — a **Plan Frontmatter Contract**
  section (gate-required `doc_type: plan` + top-level `title`/`source`; green-reference
  parity fields; the unquoted-`#` YAML truncation pitfall; optional `backlogit docs
  migrate` derivation) and a **MANDATORY Phase 4 self-lint** run at authoring time via the
  CI entrypoint.
- `.github/skills/harvest/SKILL.md` (**076.002-T**) — a **Phase 1.5 docline gate** that runs
  before decomposition / backlog mutation / the enclosing Stage harvest commit and
  **HALTs** on any violation, plus two telemetry lines. Guardrail added: "Do not decompose
  or commit a plan that fails `backlogit docs lint`."

Both gates pin to `go run ./cmd/backlogit docs lint` (== `make docs-lint`), never a stale
installed binary, so the self-lint agrees with the non-bypassable CI Docline backstop.
Root cause addressed: the 075-S PR #164 blocker, where a harvest commit carried a plan with
`doc_type: exec-plan` and no top-level `title`/`source` into the Ship feature branch and
failed the CI Docline gate.

## Shipment ship

- `backlogit shipment ship 076-S --sha ef9dc20…` → `shipment_status: shipped`.
- `archived_ids`: `076.001-T`, `076.002-T`, `076-F`, `076-S` (4). `returned_ids`: none.
- Archived shipment `076-S.md` carries `commit: ef9dc20468d865bbaf7d7b1e9b982ff7f4045422`.
- **Reconcile**: pre-mode (`expected: done`) → **PROCEED** (all three manifest items
  pre-archived — the two tasks archived during the build loop; `076-F` moved
  `active` → `done` on the closure branch before the gate, routing to archive); post-mode
  → **PROCEED** (all archive files present, no spurious deletions).
  Reports: `.backlogit/reconcile/076-S-pre-2026-07-02T225800.md`,
  `.backlogit/reconcile/076-S-post-2026-07-02T225830.md`.
- **P-007**: `git status -- .backlogit/archive/` showed only untracked additions (`??`) of
  `076-F.md`/`076-S.md` and modifications (`M`) of the two task files (commit metadata),
  with zero deletions (`D`); the intended `queue → archive` moves appear as `D` under
  `.backlogit/queue/`, not `.backlogit/archive/`. No `git restore` needed.

## Runtime verification

**PASS** — see
`docs/closure/2026-07-02-076-S-harvest-docline-frontmatter-hardening-runtime-verification.md`.
Validated behaviorally against the unchanged `backlogit docs lint` CLI: (A) the shipped
born-compliant plan lints `valid`/0 violations scoped (`--path`); (B) the repo-wide corpus
stays green; (C) a temp plan replicating the 075-S defect (`doc_type: exec-plan`, no
top-level `title`/`source`) is flagged with exactly 3 violations (`title` required,
`source` required, `doc_type` `unknown_doc_type`) and exit status 1 — proving the harvest
HALT gate catches non-compliant frontmatter.

## Release readiness

**SHIPPED** — merged, archived, and reconciled. CI 4/4 green at HEAD `74db7ec`. No code
change on the closure branch (docs + backlog archival + one operator-directed stash entry).

## Monitoring

None required. The change adds documentation/harness guidance and delegates enforcement to
the existing `backlogit docs lint` CLI and the pre-existing CI Docline gate; it introduces
no new runtime surface, telemetry, external contract, or persistence.

## Rollback

`git revert ef9dc20468d865bbaf7d7b1e9b982ff7f4045422` (single merge commit). The change is
isolated to two `.github/skills/**/SKILL.md` files (plus the plan/memory/backlog
artifacts). Zero code, data-migration, or schema impact.

## Source artifact cleanup

- **Source stash `A9D74372`** (Stage harvest docline hardening): **already archived /
  retired** by Stage during harvest — present in `.backlogit/archive/stash.jsonl`
  (`archived_at: 2026-07-03T04:04:20Z`, `reason: archived`) with a forward-link
  `[HARVESTED 2026-07-02 -> feature 076-F, shipment 076-S …]`, and confirmed **absent**
  from the active `.backlogit/stash.jsonl` (the only active reference is `EED25928`'s text,
  which cites A9D74372 as its origin).
- **Automated Step 6.7 retirement**: no-op — feature `076-F` carries no structured
  `custom_fields.source_stash_id` (only `harness_status: pending`), so the structured-field
  retirement path does not fire (same situation as 071-F–075-F). Stash retirement is
  Stage-domain and already complete; **flagged only, not forced.**
- **Deliberation artifact**: none — `076-F` has no `source_deliberation_id` (deliberate was
  SKIPPED / folded into the plan per the Stage memory).
- **Archived source IDs**: `A9D74372` (by Stage, pre-confirmed). **Skipped**: none.

## Follow-up stash added (operator-directed)

Per explicit operator instruction, a new low-priority follow-up was stashed on this closure
branch to fix the imprecise `make docs-lint --path` wording that the Copilot review
correctly flagged on the **ride-along** artifacts (not the shipped deliverables):

- **`B55985DD`** (task, low) — Reword the `make docs-lint --path` wording in
  `docs/exec-plans/2026-07-02-stage-harvest-docline-frontmatter-hardening-plan.md`
  (~L107, ~L126) and `.backlogit/archive/076.002-T.md` (~L20) to distinguish repo-wide
  `make docs-lint` (no args) from scoped `go run ./cmd/backlogit docs lint --path
  <plan_path>`. The shipped `SKILL.md` deliverables were already corrected in PR #166; this
  is cleanup of historical/auto-generated ride-along docs only. Source: PR #166 Copilot
  threads `PRRT_kwDORzozKM6OF0kp`/`0k1`/`0lG`, operator-accepted 2026-07-02, fix deferred.

## Knowledge graduation

Added a scoped **"Reinforcement — 076-S: born-compliant agent-authored plans + Stage
self-lint against its own gate"** section to the existing compound learning
`docs/compound/2026-06-26-docline-frontmatter-contract.md`. 076-S extends that doc's
**born-compliant generation** pillar from `cmd/gen-docs` machine-generated docs to
**agent-authored** plan docs, and adds the corroborating pattern that a producer (Stage)
self-validates its own output against the very CI gate that will judge it downstream
(self-lint + Phase 1.5 harvest gate, both pinned to the CI `make docs-lint` entrypoint).
No duplicate doc created; the docline gate passes on the edited compound file.

## Compact-context

Assessed (`target: all`); **no compaction executed this cycle**. `docs/memory/` remains
below every compaction trigger (file-count / 500 KB / 14-day age), and no single release
unit exceeds the >10-checkpoint mandatory trigger. The 076-S closure-session memory plus
the two new `docs/closure/2026-07-02-076-S-…` artifacts are committed to the closure branch
as the durable per-unit record. Archive-only and newest-preserved constraints honored
(nothing deleted or moved).

## Backlog integrity

- `backlogit sync` re-run after all archival + stash + knowledge-graduation mutations
  (`CLOSURE_INDEX_SYNC_OK`).
- `backlogit doctor` → only the **known, pre-existing, unrelated** orphan
  `[orphaned_artifact] 016.001-R` (a review artifact with no `parent_id`). **No new orphans
  or duplicates introduced by 076-S.**

## Follow-ups carried forward to Stage

- `B55985DD` (task) — **NEW** this closure: reword the ride-along `make docs-lint --path`
  wording (operator-directed).
- `EED25928` (task) — deferred direction 3 (harvest-delivery topology) + upstream `.tmpl`
  drift for the docline frontmatter contract (the generating templates live in the external
  autoharness repo; update there per Principle IV).
- `21E17BFC` (feature) — singleton MCP server with multiplexed transport (contingency).
- `9140F65C` (task) — fix/enable npm package publishing in the Release workflow.
- `17D29DDC` (task) — consolidate the duplicate shipment-items normalization into a single
  exported `core.NormalizeShipmentItems`.

## Verdict

**SHIPPED** — 076-S is merged, archived, and reconciled. Runtime verification PASS
(born-compliant plan passes; the 075-S defect is caught); knowledge lightly reinforced (no
duplicate); source stash `A9D74372` retired by Stage; one operator-directed follow-up
(`B55985DD`) added and four prior low-priority entries carried forward. Remaining: operator
P-014 approval of the **closure PR** before it is merged.
