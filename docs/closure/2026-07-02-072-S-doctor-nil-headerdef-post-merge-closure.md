---
chunk_strategy: h1-h2-h3
description: 'Post-merge operational closure for shipment 072-S — doctor --target nil-HeaderDef hardening (PR #158, merge d3f0fac). Records the confirmed merge (operator P-014 admin merge-commit; merge SHA an ancestor of origin/main), the shipment ship result (072-S shipped; 072-F, 072.001-T, 072-S archived with the merge SHA recorded; pre/post shipment-reconcile both PROCEED; P-007 archive integrity intact with no spurious deletions), release-readiness SHIPPED, no monitoring and git-revert rollback for the zero-blast-radius defensive edge fix, source-artifact cleanup (source stash C16DBBEB already archived/retired by Stage; automated Step 6.7 retirement a no-op because 072-F carries no structured source_stash_id custom field — Stage-domain, flagged only), knowledge graduation (reinforced the existing exported-cache-zero-value-bypass compound learning as a recurring nil-precondition-fail-open pattern rather than creating a duplicate), and the carried-forward follow-up stash 266816CE for the internal/core/artifacts.go write-path fail-open shape.'
doc_type: closure
docline:
    ms.date: 2026-07-02T00:00:00Z
    ms.topic: reference
ingested_at: "2026-07-02T00:00:00Z"
schema_version: "1.0"
source: docs/closure/2026-07-02-072-S-doctor-nil-headerdef-post-merge-closure.md
title: 072-S doctor --target nil-HeaderDef — Post-Merge Operational Closure
---

# Operational Closure — 072-S doctor --target nil-HeaderDef hardening (post-merge)

- **Date**: 2026-07-02
- **Mode**: `post-merge`
- **Shipment**: `072-S` · Feature `072-F` · Task `072.001-T`
- **PR**: #158 (`feat/072-doctor-target-nil-headerdef` → `main`)
- **Merge commit**: `d3f0facf530592c526e261b3818dc6e0d0dd0ced`
- **Pre-merge closure**: `docs/closure/2026-07-01-072-S-doctor-nil-headerdef-closure.md`
- **Runtime verification**: `docs/closure/2026-07-01-072-S-doctor-nil-headerdef-runtime-verification.md`
- **Readiness**: **SHIPPED**

## Merge confirmation

- PR #158 `state: MERGED`, merged 2026-07-02T13:53:57Z by softwaresalt (Derek Williams).
- Merge method: **merge commit** (P-009 preserved). Standard merge was blocked by
  the `PR-Review` branch-protection ruleset (`required_approving_review_count: 1`,
  `require_code_owner_review`, `require_last_push_approval`) because no formal
  approving review exists; merged via operator-authorized `--admin` under explicit
  P-014 approval + repo-owner authority. **Not** squash/rebase — ruleset
  `allowed_merge_methods: ["merge"]` and repo settings (`merge=true`,
  `squash=false`, `rebase=false`) both confirm P-009.
- Merge Confirmation Gate: `git merge-base --is-ancestor d3f0fac origin/main` → exit 0
  (merge SHA is the current `origin/main` HEAD).

## §1.9 readiness (re-checked at merge)

- Head unchanged at `a76b717` at merge time (0 unexpected HEAD drift).
- 0 pending Copilot review requests; latest Copilot review commit == headRefOid
  (`a76b717`, submitted 2026-07-02T07:14:16Z); 0 unresolved review threads (1
  thread total, resolved), fully paginated (`hasNextPage: false`). **PASS**,
  fail-closed satisfied.

## Shipment closure result

- `backlogit shipment ship 072-S --sha d3f0fac...` → `shipment_status: shipped`.
- `archived_ids`: `072.001-T`, `072-F`, `072-S` (3). `returned_ids`: none.
- Archived artifacts carry `status: archived` with `commit: d3f0fac`
  (072-S `archived_status: shipped`; 072-F, 072.001-T `archived_status: done`).
- **Reconcile**: pre-mode (`expected: done`) → **PROCEED** (both items pre-archived);
  post-mode → **PROCEED** (all archive files present, no spurious deletions).
  Reports: `.backlogit/reconcile/072-S-pre-2026-07-02T065700.md`,
  `.backlogit/reconcile/072-S-post-2026-07-02T065800.md`.
- **P-007**: `git status -- .backlogit/archive/` shows no archive deletions; only
  intended `queue → archive` moves of `072-F.md` and `072-S.md`. No restore needed.

## Invariants preserved (unchanged from pre-merge closure)

1. Versioned exit-code contract: `0` pass / `1` validation / `2` timeout /
   `3` scope\|io / `4` busy.
2. `DoctorTargetResult` schema (`schema_version 1.0.0`) unchanged.
3. `OK == true` iff `Kind == pass` on every return path.
4. Loaded-`HeaderDef` valid artifact still returns `kind=pass` (regression guard).
5. CLI and MCP behaviorally consistent via the single shared function.

## Monitoring plan

- None. Zero-blast-radius defensive edge fix; the changed branch is unreachable in a
  normally-initialized workspace. The unit test is the durable regression guard.

## Rollback

- **Trigger**: none anticipated.
- **Procedure** (if ever needed): `git revert d3f0facf530592c526e261b3818dc6e0d0dd0ced`;
  single-function, fully reversible.

## Source artifact cleanup

- **Source stash `C16DBBEB`** (071-S PR#156 Copilot follow-up K): **already archived /
  retired** by Stage during harvest — present in `.backlogit/archive/stash.jsonl`
  (`reason: archived`, `archived_at: 2026-07-01T17:35:26Z`) with a forward-link
  (`STAGE HARVESTED 2026-07-01 -> feature 072-F, task 072.001-T, queued shipment
  072-S`). Confirmed absent from the active stash (`backlogit stash get C16DBBEB` →
  not found). No Ship action taken.
- **Automated Step 6.7 retirement**: no-op — feature `072-F` carries no structured
  `custom_fields.source_stash_id` (only `harness_status: pending`), so the
  structured-field retirement path does not fire (same situation as 071-F). Since
  C16DBBEB is already retired, no follow-through is required; any residual
  structured-linkage backfill is **Stage-domain** and is flagged here, not forced by
  Ship.
- **Deliberation artifact**: none referenced by `072-F` (`references` points only to
  the exec-plan `docs/exec-plans/2026-07-01-doctor-target-nil-headerdef-hardening-plan.md`);
  nothing to archive.
- **Archived source IDs**: `C16DBBEB` (by Stage, pre-confirmed). **Skipped**: none.

## Knowledge graduation

- **Reinforced (UPDATE, not duplicate)**:
  `docs/compound/best-practices/exported-cache-zero-value-bypass-2026-06-29.md`.
  The 072-S fix is a textbook second instance of that learning at a different
  safety boundary (a validation precondition rather than a cache): a nil
  `HeaderDef` made `ValidateDoctorTargetResolved` skip validation and return
  `kind=pass` (fail-open); fixed to fail closed (`kind=io` / exit 3). Added a
  Reinforcement section, a recurrence note (the identical `if ws.HeaderDef != nil`
  fail-open shape at `internal/core/artifacts.go:224,:514`), retrieval tags
  (`validation-precondition`, `fail-closed`, `doctor-target`), and a description
  clause. Compound-refresh judgment: **keep + update** — no genuinely distinct
  durable lesson warranting a new doc; consolidating the recurrence under the
  existing rule strengthens retrievability.
- **No other docs required**: no structural change (`docs/ARCHITECTURE.md`), no
  agent/skill change (`AGENTS.md`), no new design decision (`docs/design-docs/` —
  reuses the existing io/exit-3 contract), no requirement change
  (`docs/product-specs/`).

## Follow-up (carried forward for Stage)

- **`266816CE`** (active stash, kind: task): `internal/core/artifacts.go:224` and
  `:514` guard `ValidateArtifactFields` behind `if ws.HeaderDef != nil` — the same
  fail-open shape 072-S fixed for `doctor --target`. Not doctor-path (no `kind=pass`
  misreport), so out of scope for 072-S. Stage to decide hard-fail vs warn on
  write-time absent header-def. Source:
  `docs/closure/2026-07-01-072-S-doctor-nil-headerdef-closure.md`; origin review of
  PR #158. No **new** post-merge follow-ups identified (no monitoring gaps, no
  deferred scope, no doc debt).

## Validation window / owner

- **Window**: n/a (merge-only, no rollout).
- **Owner**: Ship agent → operator. Post-merge closure complete; closure artifacts
  delivered via a separate closure PR awaiting operator P-014 approval.

## Readiness recommendation

**SHIPPED** — 072-S is merged, archived, and reconciled. Knowledge graduated;
follow-up `266816CE` carried forward to Stage. Remaining: operator approval of the
closure PR.
