---
chunk_strategy: h1-h2-h3
description: 'Post-merge operational closure for shipment 075-S — surface the covering feature (id + title) in CLI and MCP shipment views (PR #164, merge 842e888). Records the confirmed merge (operator P-014 admin merge-commit under repo-owner authority; standard merge blocked by the PR-Review branch-protection ruleset requiring a formal approving review; merge SHA an ancestor of origin/main; local main fast-forwarded f316dfd..842e888), the §1.9 readiness re-confirmation at HEAD e94ca3e (0 pending Copilot requests, latest Copilot review covers HEAD, single review thread resolved, fully paginated), P-009 verification (merge-only repo settings + ruleset allowed_merge_methods ["merge"]), the shipment ship result (075-S shipped; 075.001-T/.002-T/.003-T, 075-F, 075-S archived with the merge SHA recorded; pre/post shipment-reconcile both PROCEED; P-007 archive integrity intact with no spurious deletions), release-readiness SHIPPED, no monitoring and git-revert rollback for the zero-blast-radius read-only display feature, runtime verification PASS across three scenarios (covering-feature present, zero-feature omission, read-only invariant), source-artifact cleanup (source stash D070FD3C already archived/retired by Stage during harvest; automated Step 6.7 retirement a no-op because 075-F carries no structured source_stash_id custom field — Stage-domain, flagged only), knowledge graduation (added a scoped 075-S reinforcement note to the exported-cache-zero-value-bypass compound learning recording the nil-DB read-path guard as a distinct-shape corroboration that does not reopen the CLOSED nil-precondition-fail-open family — no duplicate doc), compact-context assessment (no compaction executed — 10 files / 45.3 KB, below all thresholds, newest artifacts preserved), and the three carried-forward low-priority follow-up stash entries (21E17BFC, 9140F65C, 17D29DDC) for Stage.'
doc_type: closure
docline:
    ms.date: 2026-07-02T00:00:00Z
    ms.topic: reference
ingested_at: "2026-07-02T00:00:00Z"
schema_version: "1.0"
source: docs/closure/2026-07-02-075-S-covering-feature-display-post-merge-closure.md
title: 075-S covering feature display — Post-Merge Operational Closure
---

# Operational Closure — 075-S surface covering feature in shipment views (post-merge)

- **Shipment**: `075-S` · Feature `075-F` · Tasks `075.001-T`, `075.002-T`, `075.003-T`
- **PR**: #164 (`feat/075-covering-feature-display` → `main`)
- **Merge commit**: `842e8883899ba25ce9c31840c89806ed2e032549`
- **Merged**: 2026-07-03T01:51:13Z by `softwaresalt` (Derek Williams)
- **Closure branch**: `post-merge/075-covering-feature-display`

## Merge

- Merge method: **merge commit** (P-009 preserved). Standard `gh pr merge 164 --merge`
  was blocked by the `PR-Review` branch-protection ruleset
  (`allowed_merge_methods: ["merge"]`, formal approving review required) because no
  formal approving review exists — only `COMMENTED` reviews (Copilot + operator).
- Resolved via operator-authorized `gh pr merge 164 --merge --admin` under the
  operator's explicit **P-014** merge approval and repo-owner authority — **still a
  merge commit** (no squash, no rebase), same pattern as #156–#163.
- **Merge Confirmation Gate**: `state: MERGED`; merge SHA `842e888…` verified as an
  ancestor of `origin/main` (`git merge-base --is-ancestor` → exit 0). `origin/main`
  advanced `18d6db1..842e888`. Local `main` fast-forwarded `f316dfd..842e888`
  (the un-pushed Stage harvest commit `f316dfd` is an ancestor of the merge, so the
  ff was clean; operator in-flux working-tree files were preserved, never staged).

## §1.9 readiness (re-confirmed at HEAD e94ca3e before merge)

- HEAD unchanged at `e94ca3e12d4fdf06d1cbf9484164609559899499`.
- 0 pending Copilot review requests (`reviewRequests.nodes: []`).
- Latest Copilot review (2026-07-03T01:36:22Z) covers current HEAD `e94ca3e`.
- 0 unresolved review threads — the single thread (`DeriveCoveringFeature` nil-`DB`
  guard) is `isResolved: true`; `hasNextPage: false` (fully paginated, fail-closed).
- CI on PR #164 at HEAD: **4/4 green** (`test (1.23)`, `test (1.24)` required,
  `CLI Reference Drift`, `Docline frontmatter gate` — the last now passing after the
  operator-approved P-010 frontmatter fix on the inherited Stage plan).

## P-009 merge strategy verification

- Repo settings: `allow_merge_commit: true`, `allow_squash_merge: false`,
  `allow_rebase_merge: false`.
- Ruleset `PR-Review` (id 14767379): `allowed_merge_methods: ["merge"]`.
- Merge commit is the only permitted strategy — no P-009 violation.

## Scope of the shipped change

Added a **read-only, render-time projection** of a shipment's covering feature
(`{id, title}`) to the shipment read paths on both surfaces, implementing the
forward-convention remedy settled in the shipment-manifest-drift determination
(stash `B8FF7590`, Unresolved Question 1):

- CLI `backlogit shipment get` — top-level `covering_feature` object in the JSON body.
- CLI `backlogit shipment list` — a `COVERING FEATURE` table column
  (`{id} — {title}`).
- MCP `backlogit_get_shipment` / `backlogit_list_shipments` — the same top-level
  projection; both tool descriptions document it.

Both surfaces marshal one type, `core.ShipmentView` (`internal/core/shipment_covering.go`),
embedding `*models.Artifact` and adding `CoveringFeature *CoveringFeature
`json:"covering_feature,omitempty"``. `DeriveCoveringFeature` resolves the root covering
feature from the manifest (`custom_fields.items`) via a pure `bldb.GetItem` read (never
`loadArtifact`, never a write path). **Load-bearing invariants**: the derived object is a
top-level sibling, never written into `custom_fields`, never persisted (so it cannot
round-trip back through any write path — the retroactive manifest mutation the B8FF7590
determination forbids); a zero-feature shipment yields a nil pointer that `omitempty`
drops on all four surfaces. Tasks: `075.001-T` (core), `075.002-T` (cli), `075.003-T`
(mcp) — commits `2f6a795`, `796f9dc`, `37dad8b`.

## Shipment ship

- `backlogit shipment ship 075-S --sha 842e888…` → `shipment_status: shipped`.
- `archived_ids`: `075.001-T`, `075.002-T`, `075.003-T`, `075-F`, `075-S` (5).
  `returned_ids`: none.
- Archived shipment `075-S.md` carries `status: archived`, `archived_status: shipped`,
  `commit: 842e8883899ba25ce9c31840c89806ed2e032549`.
- **Reconcile**: pre-mode (`expected: done`) → **PROCEED** (all four manifest items
  pre-archived — the three tasks archived during the build loop; `075-F` moved
  `active` → `done` on the closure branch before the gate, routing to archive);
  post-mode → **PROCEED** (all archive files present, no spurious deletions).
  Reports: `.backlogit/reconcile/075-S-pre-20260702T185600.md`,
  `.backlogit/reconcile/075-S-post-20260702T185830.md`.
- **P-007**: `git status -- .backlogit/archive/` showed only untracked additions (`??`)
  of the five archived files and zero deletions (`D`); the intended `queue → archive`
  moves appear as `D` under `.backlogit/queue/`, not `.backlogit/archive/`. No
  `git restore` needed.

## Runtime verification

**PASS** — see `docs/closure/2026-07-02-075-S-covering-feature-display-runtime-verification.md`.
Exercised with a source-built binary (global `C:\Tools\backlogit.exe` v1.3.0 predates the
change) in a throwaway workspace: (A) shipment covering `001-F` emits top-level
`covering_feature {id, title}` on `get` and a `COVERING FEATURE` column on `list`, never
in `custom_fields`; (B) a task-only manifest omits `covering_feature` entirely and renders
a blank cell; (C) three `get` + three `list` calls leave the manifest file bytes and the
SQLite index byte-identical, and `covering_feature` is never persisted.

## Release readiness

**SHIPPED** — merged, archived, and reconciled. Whole-repo quality gates
(`go test ./...`, `go vet ./...`, `golangci-lint run`, `gofmt -l .`) passed during build
and CI (4/4 green at HEAD `e94ca3e`). No code change on the closure branch (docs + backlog
archival only).

## Monitoring

None required. The change adds a read-only projection to existing read paths; it
introduces no new runtime surface, no telemetry, no external contract, and no persistence.

## Rollback

`git revert 842e8883899ba25ce9c31840c89806ed2e032549` (single merge commit). The change is
isolated to `internal/core/shipment_covering.go`, the CLI shipment render path, the MCP
list/get handlers, and their tests. Zero data-migration or schema impact; the projection
is derived at render time so no persisted state is affected.

## Source artifact cleanup

- **Source stash `D070FD3C`** (the covering-feature forward-UX enhancement): **already
  archived / retired** by Stage during harvest — present in
  `.backlogit/archive/stash.jsonl` with a forward-link to feature `075-F` / shipment
  `075-S`, and confirmed **absent** from the active `.backlogit/stash.jsonl`.
- **Automated Step 6.7 retirement**: no-op — feature `075-F` carries no structured
  `custom_fields.source_stash_id` (only `harness_status: pending`), so the
  structured-field retirement path does not fire (same situation as 071-F / 072-F /
  073-F). Stash retirement is Stage-domain and already complete; **flagged only, not
  forced.**
- **Deliberation artifact**: `075-F` references the exec-plan
  `docs/exec-plans/2026-07-02-shipment-covering-feature-display-plan.md` and the
  determination `docs/decisions/2026-06-25-shipment-manifest-drift-determination-deliberation.md`;
  no `source_deliberation_id` custom field → nothing to archive here (Stage domain).
- **Archived source IDs**: `D070FD3C` (by Stage, pre-confirmed). **Skipped**: none.

## Knowledge graduation

Read-only display feature — no new durable lesson warranted, and **no duplicate doc
created**. Added a scoped **"Reinforcement — 075-S: distinct shape — nil-DB guard in a
read-only derivation (family stays CLOSED)"** section to the existing compound learning
`docs/compound/best-practices/exported-cache-zero-value-bypass-2026-06-29.md`. The
Copilot-caught `ws.DB == nil` guard (return `ok=false` instead of a nil-pointer panic in
`bldb.GetItem`) corroborates the doc's generalized "route an absent precondition to the
safe result" principle, but is explicitly recorded as a **distinct failure mode**
(panic-avoidance in a read path, not a fail-open validation skip) that **does not reopen**
the CLOSED nil-precondition-fail-open family (070-S / 072-S / 073-S). The docline
frontmatter gate passes on the edited doc.

## Compact-context

Assessed (`target: all`); **no compaction executed this cycle**. `docs/memory/` top-level
is 10 files / 45.3 KB — below every compaction trigger (40 files / 500 KB / 14-day age;
oldest file is 1 day old) and no single release unit exceeds the >10-checkpoint mandatory
trigger. The two 075-S session memory files (`…-075-S-HALT-inherited-stage-commit.md`,
`…-075-S-task3-mcp-checkpoint.md`) plus a new closure-session summary are committed to the
closure branch as the durable per-unit record; the batch
`docs/memory/compacted/2026-07-02-shipped-units-068-071-compacted.md` already covers prior
shipped units. Archive-only and newest-preserved constraints honored (nothing deleted or
moved).

## Backlog integrity

- `backlogit sync` → indexed 669 artifacts.
- `backlogit doctor` → **1 issue**: `[orphaned_artifact] 016.001-R` — a **known,
  pre-existing, unrelated** orphan (review artifact with no `parent_id`). **No new
  orphans or duplicates introduced by 075-S.**
- `backlogit sync` re-run after all archival + knowledge-graduation mutations
  (`CLOSURE_INDEX_SYNC_OK`).

## Follow-ups carried forward to Stage

No new follow-ups were generated by 075-S post-merge closure. The following three
low-priority stash entries remain in the active stash for Stage to triage:

- `21E17BFC` (feature) — singleton MCP server with multiplexed transport (contingency).
- `9140F65C` (task) — fix/enable npm package publishing in the Release workflow.
- `17D29DDC` (task) — consolidate the duplicate shipment-items normalization
  (`internal/mcp normalizeShipmentItems` vs `internal/core shipmentItems`) into a single
  exported `core.NormalizeShipmentItems`. **Surfaced during plan-review of 075-F** and
  deferred there to avoid scope creep; now the natural next-touch follow-up for this area.

Per operator instruction, **no stash entries were added or modified** during this closure
(the source-stash archival of `D070FD3C` was already completed by Stage and required no
`stash.jsonl` change here), to avoid a `.backlogit/stash.jsonl` fast-forward collision with
the Orchestrator's queued process-hardening chore stash.

## Verdict

**SHIPPED** — 075-S is merged, archived, and reconciled. Runtime verification PASS;
knowledge lightly reinforced (no duplicate; CLOSED family untouched); source stash
`D070FD3C` retired by Stage; three low-priority stash entries carried forward. Remaining:
operator P-014 approval of the **closure PR** before it is merged.
