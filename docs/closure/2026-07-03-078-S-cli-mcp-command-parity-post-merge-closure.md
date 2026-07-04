---
chunk_strategy: h1-h2-h3
description: 'Post-merge operational closure for shipment 078-S — CLI/MCP command parity: make the MCP->CLI fallback map honest and guard it with a registry drift test, fill the highest-value CLI gaps (shipment add, checkpoint create), fold in the 7ECBAC7E shipment-list null-vs-array fix, and ship a discoverability guide + parity audit matrix (feature 078-F, PR #170, merge e2ab16c, merged 2026-07-04T01:47:06Z by softwaresalt via operator-authorized admin merge over the review-required ruleset). Records the confirmed merge (true merge commit, parents 219ed96 + 7261e36, P-009 preserved), PR #170 CI 4/4 green (test 1.23, test 1.24, CLI Reference Drift, Docline frontmatter gate), the §1.9 Copilot gate re-verified on HEAD before merge, the shipment ship result (078-S shipped; all 15 manifest items + shipment archived with the merge SHA recorded; pre/post shipment-reconcile both PROCEED; P-007 archive integrity intact with only the intended queue->archive shipment move and no spurious archive deletions), whole-suite gates green, runtime verification PASS, release-readiness SHIPPED, git-revert rollback, knowledge graduation (one NEW compound learning captured + compound-refresh report; the discoverability design doc already graduated on main), source-artifact cleanup (no structured source_stash_id/source_deliberation_id on 078-F; originating stash E16F4664 and folded 7ECBAC7E already retired by Stage during harvest; deliberation record deliberately left for Stage per operator instruction), and the follow-up/out-of-scope stash entries carried forward to Stage (2827CB5F deliberation reconciliation, plus 21E17BFC, 9140F65C, EED25928, B55985DD, 6C6ACE00).'
doc_type: closure
docline:
    ms.date: 2026-07-03T00:00:00Z
    ms.topic: reference
ingested_at: "2026-07-03T00:00:00Z"
schema_version: "1.0"
source: docs/closure/2026-07-03-078-S-cli-mcp-command-parity-post-merge-closure.md
title: 078-S CLI/MCP command parity — Post-Merge Operational Closure
---

# Operational Closure — 078-S CLI/MCP command parity (post-merge)

- **Shipment**: `078-S` · Feature `078-F` · 6 tasks + 8 subtasks (15 items)
- **PR**: #170 (`feat/078-cli-mcp-command-parity` → `main`)
- **Merge commit**: `e2ab16c0e893d6bcb260162099b0d3f7e87530c2`
- **Merged**: 2026-07-04T01:47:06Z by `softwaresalt` (Derek Williams)
- **Closure branch**: `post-merge/078-cli-mcp-command-parity`

## Merge

- Merge method: **merge commit** (P-009 preserved) — PR #170 `state: MERGED`,
  `mergeCommit.oid: e2ab16c…`, a **true merge commit** with two parents
  (`219ed96` base + `7261e36` feature tip). Not squash, not rebase.
- Merge SHA `e2ab16c…` confirmed in `origin/main` history
  (`git merge-base --is-ancestor` exit 0); local `main` fast-forwarded to it.
- **Admin merge**: the active repository ruleset **"PR-Review"** (id 14767379)
  requires 1 approving review with `require_code_owner_review` +
  `require_last_push_approval`. The sole GitHub identity (`softwaresalt`) is the
  PR author and cannot self-approve, so no human approving review could exist.
  The operator (repo owner) explicitly authorized an **admin bypass**
  (`gh pr merge 170 --merge --admin`), which the ruleset permits via
  `bypass_actors: RepositoryRole(Admin)/pull_request`. Not a force-bypass of
  P-009; merge-commit strategy preserved.

## P-009 merge strategy verification

Repo settings: `allow_merge_commit=true`, `allow_squash_merge=false`,
`allow_rebase_merge=false` — merge commit is the **only** permitted strategy.
Merge commit `e2ab16c` has two parents, confirming a real merge (no squash/rebase
rewrite). **No P-009 violation.**

## §1.9 pre-merge Copilot gate (re-verified on HEAD `7261e36`)

- Check 1 — no pending Copilot request: **PASS** (0 pending).
- Check 2 — latest Copilot review covers HEAD: **PASS** (`copilot@7261e36 == HEAD`).
- Check 3 — zero unresolved Copilot threads: **PASS** (0 unresolved).
- CI status rollup at HEAD: **SUCCESS**; PR `MERGEABLE`.

## Scope of the shipped change

CLI/MCP command parity, phase-1 (Option B deliberation). Six units:

- **U1 · `078.001-T`** — Parity audit matrix published:
  `docs/reviews/2026-07-03-cli-mcp-parity-matrix.md`.
- **U2 · `078.002-T` (+2 ST)** — Corrected the `.autoharness/backlog-registry.yaml`
  MCP->CLI op-map and locked it with a **drift test**
  (`internal/cli/registry_parity_test.go`: `EveryMCPToolMappedOrDeferred`,
  `EveryCLICommandResolves`, `NoOrphanMCPTool`, `DiscoverabilityConsistency`).
- **U3 · `078.003-T` (+2 ST)** — New `shipment add` CLI command
  (`internal/cli/shipment.go` `newShipmentAddCmd`), output-shape parity with MCP
  (`internal/cli/shipment_add_test.go`).
- **U4 · `078.004-T` (+2 ST)** — `shipment list` items normalized `null` → `[]`
  across CLI + MCP (`internal/cli/shipment_list_items_test.go`); folds in stash
  `7ECBAC7E`.
- **U5 · `078.005-T`** — MCP-to-CLI fallback + discoverability guide (durable
  design doc): `docs/design-docs/2026-07-03-mcp-to-cli-fallback-guide.md`.
- **U6 · `078.006-T` (+2 ST)** — New `checkpoint create` CLI command
  (`internal/cli/checkpoint.go` `newCheckpointCreateCmd`),
  `internal/cli/checkpoint_create_test.go`.

A late review-cycle hardening reverted the `docs_migrate` fallback template to
plan-only (blast-radius parity with the MCP `apply=false` default), documenting
`--apply --yes` as an opt-in escalation in a YAML comment.

## Shipment ship

- `backlogit shipment ship 078-S --sha e2ab16c… --message "Merge pull request #170 …"
  --author softwaresalt` → `shipment_status: shipped`.
- `archived_ids` (16): all 15 manifest items + `078-S`. `returned_ids`: none.
- Archived shipment `078-S.md` carries `commit: e2ab16c0e893d6bcb260162099b0d3f7e87530c2`.
- **Reconcile**: pre-mode (`expected: done`) → **PROCEED** (all 15 items already
  `pre-archived` — the done-status routing relocated them queue→archive in the
  merge). post-mode → **PROCEED** (all 15 item files + `078-S.md` present in
  archive, status `archived`). Reports under `.backlogit/reconcile/`.
- **P-007**: `git status -- .backlogit/` shows the 15 archive item files modified
  (commit metadata) and the intended `queue/078-S.md → archive/078-S.md` shipment
  move (rename) — **zero deletions under `.backlogit/archive/`**. No `git restore`
  needed.

## Runtime verification

**PASS** — see
`docs/closure/2026-07-03-078-S-cli-mcp-command-parity-runtime-verification.md`
(pre-merge, on HEAD `7261e36`). Fresh binary exercised `shipment add` (JSON
`{item_id,shipment_id,status:added}`, sentinel not-found, arg validation),
`shipment list` (items `[]`/`[item]`, never null), and `checkpoint create`→list
roundtrip (oneof agent validation). Quality gates on the feature branch: `go test
./...` PASS, `go vet ./...` exit 0, `golangci-lint run` exit 0 (gofmt local-CRLF
false positive; CI LF gate authoritative, 4/4 green).

## Release readiness

**SHIPPED** — merged, archived, and reconciled. PR #170 CI 4/4 green. Closure
branch carries docs + backlog archival only (no production code change).

## Monitoring

None required. The change adds two new CLI commands mirroring existing MCP
operations, an honest registry map guarded by a compile-time-adjacent drift test,
and an output-shape normalization. No new telemetry, external contract, or
persistence schema.

## Rollback

`git revert e2ab16c0e893d6bcb260162099b0d3f7e87530c2` (single merge commit).
The change is isolated to `internal/cli/*` (shipment/checkpoint commands + tests),
`.autoharness/backlog-registry.yaml` (op-map + templates), and docs. Zero
data-migration or schema impact. Reverting removes the two new CLI commands and
restores the prior (dishonest) registry map — the drift test would then fail,
which is the intended guard.

## Source artifact cleanup

- **`custom_fields.source_stash_id`**: **absent** on `078-F` (only
  `harness_status: pending`), same as `071-F`–`077-F`. Automated stash retirement
  is a **no-op**. The originating parity stash **`E16F4664`** and the folded
  shipment-list stash **`7ECBAC7E`** are both **already absent** from the active
  `.backlogit/stash.jsonl` (retired by Stage during harvest). Flagged only, not
  forced.
- **`custom_fields.source_deliberation_id`**: **absent** on `078-F`. The
  deliberation record `docs/decisions/2026-07-03-cli-mcp-command-parity-deliberation.md`
  is referenced but has no structured link and is **deliberately left un-archived**
  for Stage to reconcile (its lines 86/88/94 drift is tracked in stash `2827CB5F`).
  Per operator instruction and P-010, Ship does not touch or archive it.

## Knowledge graduation

- **New compound learning captured** (evidence-backed, lint-clean):
  `docs/compound/2026-07-03-cli-mcp-honest-fallback-map-and-registry-drift-test.md`
  — four durable rules (honest fallback map + drift test; gap-fill discipline;
  output-shape parity `[]`-never-`null`; fallback blast-radius parity).
- **Compound-refresh report**:
  `docs/closure/2026-07-03-078-S-cli-mcp-command-parity-compound-refresh.md` —
  existing parity entries (`2026-06-27-cli-mcp-catalog-parity…`,
  `2026-05-07-mcp-cli-config-parity`, `manual-schema-registry-drift-detection…`)
  classified **keep** (distinct layers), one new capture.
- **Design decision graduation**: the MCP-to-CLI fallback discoverability guide
  shipped as a durable design doc
  (`docs/design-docs/2026-07-03-mcp-to-cli-fallback-guide.md`) and is already on
  `main`; no further graduation needed.

## Compact-context

Assessed (`target: all`); **no compaction executed this cycle**. `docs/memory/`
remains below every compaction trigger (file-count / 500 KB / age), and no single
release unit exceeds the 10+-checkpoint mandatory trigger. The 078-S session
memory plus the three `docs/closure/2026-07-03-078-S-…` artifacts are the durable
per-unit record. Archive-only / newest-preserved constraints honored (nothing
deleted or moved).

## Backlog integrity

- `backlogit sync` re-run after all archival + knowledge mutations.
- `078-F`, `078-S`, and all 13 tasks/subtasks `status: archived` in the index.

## Follow-ups carried forward to Stage

- **`2827CB5F`** (task) — reconcile the Stage-owned deliberation record
  (`…-cli-mcp-command-parity-deliberation.md`) drift on lines 86/88/94 (matrix
  location, task-count 5→6, checkpoint-create deferred→shipped). Created this
  shipment; **P-010 — Stage domain**.
- **`EED25928`**, **`B55985DD`** (tasks) — external autoharness `.tmpl` source
  drift; **out-of-tree, out of scope** (Principle IV). Not touched.
- **`21E17BFC`** (feature, low) — singleton MCP server with multiplexed transport.
- **`9140F65C`** (task, low) — fix/enable npm package publishing in Release workflow.
- **`6C6ACE00`** (task, low) — carried forward.

## Verdict

**SHIPPED** — 078-S is merged (operator-authorized admin merge, merge-commit
strategy, P-009 preserved), archived, and reconciled (pre/post PROCEED, P-007
intact). Runtime verification PASS. One new compound learning graduated; the
discoverability design doc already on `main`. Source stashes `E16F4664` /
`7ECBAC7E` already retired by Stage; deliberation record left for Stage per
operator instruction. Remaining: operator P-014 approval of the **closure PR**
before it is merged.
