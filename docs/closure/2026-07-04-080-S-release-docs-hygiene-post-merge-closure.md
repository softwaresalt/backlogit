---
chunk_strategy: h1-h2-h3
description: 'Post-merge operational closure for shipment 080-S — release pipeline and documentation hygiene (feature 080-F, PR #174, merge d0ebb4f, merged 2026-07-04T18:03:49Z by softwaresalt via operator-authorized admin merge over the review-required ruleset). Three mutually-independent P3-only hygiene units: 080.001-T guarded the release.yml npm-publish job on NPM_TOKEN presence via an env-indirection preflight step (boolean has_token output, if-gated publish steps, no red X when the token is absent), 080.002-T added a characterization test pinning retired packaging script package.json output (isolated temp copy; shell script + Go wrapper, 2-file stop rule), and 080.003-T corrected misleading make docs-lint --path wording in exactly two files. Records the confirmed merge (true merge commit, parents af26c71 + e718a81, P-009 preserved and verified merge-commit-only), PR #174 CI 4/4 green (test 1.23, test 1.24, CLI Reference Drift, Docline frontmatter gate), the §1.9 Copilot gate re-verified on HEAD e718a81 before merge (Copilot reviewed 16/16 files with zero comments and zero unresolved threads), the shipment ship result (080-S shipped; all 4 manifest items + shipment archived with the merge SHA recorded; pre/post shipment-reconcile both PROCEED; P-007 archive integrity intact with only intended queue->archive moves and no spurious deletions), whole-suite gates green, runtime verification PASS WITH FOLLOW-UP (release.yml guard statically verified; characterization test executed), release-readiness SHIPPED, git-revert rollback, knowledge graduation (no ARCHITECTURE/AGENTS changes — pure hygiene; compound-refresh classified all entries keep, no supersession), source-artifact cleanup (no structured source_stash_id/source_deliberation_id on 080-F; originating stashes 9140F65C + B55985DD already retired by Stage during harvest; deliberation is a docs/decisions design record, not a queue artifact), and the deferred/out-of-scope stash entries left untouched (34F11E5A external package registry / NPM_TOKEN provisioning, EED25928 external .tmpl, 21E17BFC contingency).'
doc_type: closure
docline:
    ms.date: 2026-07-04T00:00:00Z
    ms.topic: reference
ingested_at: "2026-07-04T00:00:00Z"
schema_version: "1.0"
source: docs/closure/2026-07-04-080-S-release-docs-hygiene-post-merge-closure.md
title: 080-S release pipeline & docs hygiene — Post-Merge Operational Closure
---

# Operational Closure — 080-S release pipeline & docs hygiene (post-merge)

- **Shipment**: `080-S` · Feature `080-F` · 3 tasks (`080.001-T`, `080.002-T`, `080.003-T`)
- **PR**: #174 (`feat/080-release-docs-hygiene` → `main`)
- **Merge commit**: `d0ebb4fc4d37b332ed128b4413e7e64ccc0d0095`
- **Merged**: 2026-07-04T18:03:49Z by `softwaresalt` (Derek Williams)
- **Closure branch**: `post-merge/080-S`

## Merge

- Merge method: **merge commit** (P-009 preserved) — PR #174 `state: MERGED`,
  `mergeCommit.oid: d0ebb4f…`, a **true merge commit** with two parents
  (`af26c71` base + `e718a81` feature tip). Not squash, not rebase.
- Merge SHA `d0ebb4f…` confirmed in `origin/main` history
  (`git merge-base --is-ancestor` exit 0); local `main` fast-forwarded to it.
- **Admin merge**: the active repository ruleset **"PR-Review"** requires an approving
  review the sole GitHub identity (`softwaresalt`, the PR author) cannot self-supply.
  The operator (repo owner) explicitly authorized an **admin bypass**
  (`gh pr merge 174 --merge --admin --delete-branch`). Not a force-bypass of P-009;
  merge-commit strategy preserved. Same posture as 078-S / 079-S.

## P-009 merge strategy verification

Repo settings permit **merge commit only** (`allow_merge_commit=true`,
`allow_squash_merge=false`, `allow_rebase_merge=false`). Merge commit `d0ebb4f` has two
parents, confirming a real merge (no squash/rebase rewrite). **No P-009 violation.**

## §1.9 pre-merge Copilot gate (re-verified on HEAD `e718a81`)

- Check 1 — no pending Copilot request: **PASS** (`reviewRequests.nodes == []`).
- Check 2 — latest Copilot review covers HEAD: **PASS** (Copilot review `2026-07-04T09:07:49Z`
  `commit.oid == e718a81 == headRefOid`).
- Check 3 — zero unresolved Copilot threads: **PASS** (`reviewThreads.nodes == []`,
  `hasNextPage: false`). Copilot reviewed 16/16 changed files and generated no comments.
- CI status rollup at HEAD `e718a81`: **SUCCESS** (4/4); PR `MERGEABLE`.
- `reviewDecision: REVIEW_REQUIRED` — the PR-Review ruleset, satisfied by the
  operator-authorized admin bypass (not a Copilot-gate failure).

## Scope of the shipped change

Three mutually-independent, low-risk (P3-only) hygiene units:

- **Unit A · `080.001-T` (`ci`)** — `.github/workflows/release.yml` npm-publish token guard:
  an env-indirection preflight step (`id: preflight`) tests `NPM_TOKEN` presence and emits a
  boolean `has_token`; both publish steps are `if: steps.preflight.outputs.has_token == 'true'`.
  No red X when the token is intentionally absent. SHA pins, `contents: read`,
  `persist-credentials: false`, and the existing `continue-on-error: true` preserved; the token
  value is never echoed.
- **Unit B · `080.002-T` (`test`)** — characterization test pinning `retired packaging script`
  output (`retired packaging characterization script` + `retired packaging characterization test`).
  6 valid, version-stamped `package.json` + synced wrapper `optionalDependencies`, run against an
  isolated `mktemp -d` copy so tracked files are never mutated. 2-file stop rule respected;
  `retired packaging script` unchanged; `npm pack` optional/off-by-default.
- **Unit C · `080.003-T` (`docs`)** — corrected misleading `make docs-lint --path` wording in
  exactly two files, distinguishing repo-wide `make docs-lint` (no args) from scoped
  `go run ./cmd/backlogit docs lint --path <file>`.

## Shipment ship result

`backlogit shipment ship 080-S --sha d0ebb4f … --author "Derek Williams <…>"`:

- `shipment_status: shipped`
- `archived_ids: [080.001-T, 080.002-T, 080.003-T, 080-F, 080-S]`
- `returned_ids: []`
- `commit_sha: d0ebb4fc4d37b332ed128b4413e7e64ccc0d0095`

The three task items and the feature were **pre-archived** (marked done during the build loop);
`shipment ship` stamped the merge SHA onto them and archived the shipment artifact itself.

## Shipment-reconcile

- **Pre-mode** (`expected_status: done`) → **PROCEED**. All 4 manifest items `pre-archived`
  (valid); shipment `080-S` active in queue; orphan scan clean.
  (`.backlogit/reconcile/080-S-pre-2026-07-04T110538.md`)
- **Post-mode** (`merge_commit_sha: d0ebb4f`) → **PROCEED**. All 4 manifest items + the shipment
  artifact present in `.backlogit/archive/`; **P-007** deleted-file guard clean (no archive
  deletions; only intended `queue→archive` moves), no `git restore` required.
  (`.backlogit/reconcile/080-S-post-2026-07-04T110705.md`)

## Whole-suite gates (merge HEAD `e718a81`)

`go test ./...` ✅ · `go vet ./...` ✅ · `golangci-lint run` ✅ (0 findings) · `gofmt -l` = CRLF
false-positives only (new `.go` file LF-clean; CI-LF authoritative) · `actionlint` ✅ ·
`backlogit docs lint` ✅ (0 violations) · CI 4/4 green.

## Runtime verification

**PASS WITH FOLLOW-UP** — the `release.yml` guard was verified statically (actionlint + YAML +
both-branch logic walkthrough) since the tag-triggered workflow cannot be exercised in-tree
without a real `NPM_TOKEN`; the characterization test executed green locally and in CI.
(`docs/closure/2026-07-04-080-S-release-docs-hygiene-runtime-verification.md`)

## Release readiness

**SHIPPED.** Merge-only rollout — CI-workflow YAML + a test artifact + docs; no deployed
service, migration, or binary command surface changed. The release-workflow guard takes effect
on the next `v*.*.*` tag push.

## Knowledge graduation

- **`docs/ARCHITECTURE.md`** — no change. 080-S introduced no structural/module change.
- **`AGENTS.md`** — no change. No agent or skill was added or modified.
- **`docs/design-docs/`** — no change. No design decision graduated; the deliberation
  (`docs/decisions/2026-07-04-release-docs-hygiene-deliberation.md`) remains the point-in-time
  decision record.
- **`docs/product-specs/`** — no change. No requirement change.
- **`docs/compound/`** — compound-refresh classified all related entries **keep**; nothing
  superseded or invalidated, and no new hard-won learning warranted capture (routine hygiene).
  See `docs/closure/2026-07-04-080-S-release-docs-hygiene-compound-refresh.md`.

## Source-artifact cleanup

- 080-F carries **no structured `source_stash_id` / `source_deliberation_id`** custom_fields
  (the source stashes are referenced in prose; the deliberation is in `references`).
- Originating stashes **`9140F65C`** (npm-publish guard + retired packaging script validation) and
  **`B55985DD`** (docs-lint wording) were **already retired by Stage** during the 080-S harvest —
  both absent from the current stash list. Nothing for Ship to retire.
- The deliberation `docs/decisions/2026-07-04-release-docs-hygiene-deliberation.md` is a
  permanent design record (a `docs/` file, not a backlog queue artifact) — retained, not archived.

## Deferred / out-of-scope (left untouched — Principle IV)

- **`34F11E5A`** — external npm-org + `NPM_TOKEN` provisioning (human-only; the external slice
  carved out of 080-S and re-stashed by Stage). Once provisioned, the guard shipped in 080.001-T
  lets the publish steps run automatically on the next release with no workflow change.
- **`EED25928`** — external autoharness `.tmpl` parity edits (out-of-tree).
- **`21E17BFC`** — singleton-MCP-server contingency (trigger condition not met).

## Rollback

- **Trigger**: post-merge CI on `main` fails on the docline gate or test matrix, OR the first
  tagged release after merge shows the guard inverting publish behavior or leaking the token.
- **Procedure**: `git revert` the additive commits on a fresh branch and open a revert PR.
  Purely additive change (guard + test + docs) → low-risk revert; no data migration to unwind.

## Follow-up

- **Observe the guard on the next real tagged release** (workflow end-to-end confirmation).
- Optional P3 test hardening (deferred): wrap the shell-test exec in `exec.CommandContext` with a
  timeout in `retired packaging characterization test`.
- **Stashed `2EF8B7AD`** (housekeeping, low): compact/archive the `docs/closure/` backlog — the
  directory reached 87 files / ~581 KB, exceeding the compact-context 500 KB threshold, with stale
  records back to 2026-04-07. Deferred out of this closure PR to keep it scoped (bulk historical
  archival would be unrelated scope creep). Stage to schedule as a dedicated compact-context unit.

## Compact-context (Step 6.8)

- **080-S's own artifacts are same-day and durably persisted** (this closure doc, the
  compound-refresh, the two reconcile reports, and the finalized session memory are committed on
  `post-merge/080-S`) — not compaction candidates.
- `docs/memory/`: 26 files / ~156 KB — under thresholds; no stale (>14d) files (prior sessions
  already archived older checkpoints under `docs/memory/compacted/` + `docs/archive/`).
- `docs/closure/`: 87 files / ~581 KB — **exceeds the 500 KB threshold** with many stale records.
  Repo-wide historical closure compaction is a legitimate candidate but **deferred** to protect
  this PR's scope/reviewability; surfaced as stash `2EF8B7AD` for a dedicated Stage-scheduled unit.

## Session outcome

080-S **SHIPPED** and closed. Feature PR #174 merged (`d0ebb4f`); backlog archived; post-merge
closure committed on `post-merge/080-S`. Closure PR opened and held at merge-ready pending its own
operator P-014 approval (§1.10 — approval does not carry over from the feature PR).
