---
chunk_strategy: h1-h2-h3
description: 'Post-merge operational closure for shipment 079-S — CLI/MCP command parity phase-2: build the deferred CLI fallbacks worth building (link, hooks poll/ack, memory save, comment add, metadata discovery), each reusing a shared core/events path with MCP output-shape parity, flip 10 registry rows from mcp_only to resolvable cli_command guarded by the U6 drift+flag-parity gate, ship discoverability docs and regenerate cli-reference (feature 079-F, PR #172, merge a8e07ea, merged 2026-07-04T06:33:34Z by softwaresalt via operator-authorized admin merge over the review-required ruleset). Records the confirmed merge (true merge commit, parents 8bf53eb + 0d5accf, P-009 preserved), PR #172 CI 4/4 green (test 1.23, test 1.24, CLI Reference Drift, Docline frontmatter gate), the §1.9 Copilot gate re-verified on HEAD 0d5accf before merge (one concurrency thread on core.AppendComment fixed in 6257fab and resolved), the shipment ship result (079-S shipped; all 15 manifest items + shipment archived with the merge SHA recorded, source deliberation 051-DL co-archived; pre/post shipment-reconcile both PROCEED; P-007 archive integrity intact with only the intended queue->archive moves and no spurious archive deletions), whole-suite gates green, runtime verification PASS (five CLI families), release-readiness SHIPPED, git-revert rollback, knowledge graduation (one NEW compound learning captured on shared-EventWriter append serialization + compound-refresh report), source-artifact cleanup (no structured source_stash_id/source_deliberation_id on 079-F; originating stash 6C6ACE00 already retired by Stage during harvest; deliberation 051-DL co-archived by shipment ship), and the deferred/out-of-scope stash entries left untouched (merge_sync->phase-3, log_telemetry permanent mcp_only; stashes 21E17BFC, 9140F65C, EED25928, B55985DD carried forward to Stage).'
doc_type: closure
docline:
    ms.date: 2026-07-03T00:00:00Z
    ms.topic: reference
ingested_at: "2026-07-03T00:00:00Z"
schema_version: "1.0"
source: docs/closure/2026-07-03-079-S-cli-mcp-command-parity-phase2-post-merge-closure.md
title: 079-S CLI/MCP command parity phase-2 — Post-Merge Operational Closure
---

# Operational Closure — 079-S CLI/MCP command parity phase-2 (post-merge)

- **Shipment**: `079-S` · Feature `079-F` · 8 tasks + 6 subtasks (15 items)
- **PR**: #172 (`feat/079-cli-mcp-command-parity-phase2` → `main`)
- **Merge commit**: `a8e07ea38f8e153e9a29def264538bcab8222868`
- **Merged**: 2026-07-04T06:33:34Z by `softwaresalt` (Derek Williams)
- **Closure branch**: `post-merge/079-S`

## Merge

- Merge method: **merge commit** (P-009 preserved) — PR #172 `state: MERGED`,
  `mergeCommit.oid: a8e07ea…`, a **true merge commit** with two parents
  (`8bf53eb` base + `0d5accf` feature tip). Not squash, not rebase.
- Merge SHA `a8e07ea…` confirmed in `origin/main` history
  (`git merge-base --is-ancestor` exit 0); local `main` fast-forwarded to it.
- **Admin merge**: the active repository ruleset **"PR-Review"** requires an
  approving review with `require_last_push_approval`. The sole GitHub identity
  (`softwaresalt`) is the PR author and cannot self-approve, so no human approving
  review could exist. The operator (repo owner) explicitly authorized an **admin
  bypass** (`gh pr merge 172 --merge --admin --delete-branch`), which the ruleset
  permits via `bypass_actors: RepositoryRole(Admin)`. Not a force-bypass of P-009;
  merge-commit strategy preserved.

## P-009 merge strategy verification

Repo settings permit **merge commit only** (`allow_squash_merge=false`,
`allow_rebase_merge=false`). Merge commit `a8e07ea` has two parents, confirming a
real merge (no squash/rebase rewrite). **No P-009 violation.**

## §1.9 pre-merge Copilot gate (re-verified on HEAD `0d5accf`)

- Check 1 — no pending Copilot request: **PASS** (0 pending; `reviewRequests` empty).
- Check 2 — latest Copilot review covers HEAD: **PASS** (Copilot review `06:17:37Z`
  `commit.oid == 0d5accf == headRefOid`).
- Check 3 — zero unresolved Copilot threads: **PASS** (the single concurrency thread
  on `core.AppendComment` was resolved; `hasNextPage: false`).
- CI status rollup at HEAD `0d5accf`: **SUCCESS** (4/4); PR `MERGEABLE`.
- `reviewDecision: REVIEW_REQUIRED` — the PR-Review ruleset, satisfied by the
  operator-authorized admin bypass (not a Copilot-gate failure).

## Scope of the shipped change

CLI/MCP command parity, phase-2 (follows 078-F/078-S). Eight units, five new CLI
command families each reusing the shared `core`/`events` path (no logic
duplication, test-first, MCP output-shape parity):

- **U1 · `079.001-T` (+2 ST)** — `link` CLI group (`add`/`remove`/`list`) over
  `core.GetLinks`; never-null normalization (nil links → `[]`).
- **U2 · `079.002-T`** — `hooks` CLI (`poll`/`ack`) over
  `events.PollHookEvents` / `events.AckHookEvents`.
- **U3 · `079.003-T`** — `memory save` CLI over `events.SaveMemory`.
- **U4 · `079.004-T` (+2 ST)** — `comment add` CLI over `core.AppendComment`
  (extracted to share the MCP write path).
- **U5 · `079.005-T`** — `metadata` discovery CLI
  (`get_wit_metadata` / `list_types` / `list_templates`) over
  `core.ListTypes` / `core.DescribeType`.
- **U6 · `079.006-T` (+2 ST)** — flipped **10** `.autoharness/backlog-registry.yaml`
  rows from `mcp_only:true` to resolvable `cli_command`; extended the drift gate
  (`internal/cli/registry_parity_test.go`
  `TestRegistryParity_FlagAndPositionalParity`) with **flag/positional/required-flag
  parity** assertions; fixed a real pre-existing `stash add` drift.
- **U7 · `079.007-T`** — MCP→CLI discoverability docs updated.
- **U8 · `079.008-T`** — regenerated `docs/cli-reference/`.

A review-cycle hardening (`6257fab`) threaded the MCP server's shared
`*events.EventWriter` through `core.AppendComment` (MCP passes `s.Events`, CLI
passes `nil`) so the extraction preserves the server's per-item JSONL append
serialization — see the new compound learning. A second hardening (`c19b5c2`)
added required-flag coverage to the U6 parity gate.

## Shipment ship

- `backlogit shipment ship 079-S --sha a8e07ea… --message "Merge pull request #172 …"
  --author 42183845+softwaresalt@…` → `shipment_status: shipped`.
- `archived_ids` (17): all 15 manifest items + source deliberation `051-DL` +
  `079-S`. `returned_ids`: none.
- Archived shipment `079-S.md` carries
  `commit: a8e07ea38f8e153e9a29def264538bcab8222868`, `status: archived`.
- **Reconcile**: pre-mode (`expected: done`) → **PROCEED** (all 15 items already
  `pre-archived` — the auto-archive-on-done routing relocated them queue→archive
  during the build loop). post-mode → **PROCEED** (all 15 item files + `079-S.md`
  present in archive, status `archived`). Reports under `.backlogit/reconcile/`
  (`079-S-pre-2026-07-03T233640.md`, `079-S-post-2026-07-03T233730.md`).
- **P-007**: `git status -- .backlogit/archive/` shows the 15 archive item files
  modified (commit metadata) and the intended `queue → archive` moves for `079-S.md`
  and `051-DL.md` — **zero deletions under `.backlogit/archive/`**. No `git restore`
  needed.

## Runtime verification

**PASS** — see
`docs/closure/2026-07-03-079-S-cli-mcp-command-parity-phase2-runtime-verification.md`
(pre-merge, fresh binary exercised all five new CLI families). Quality gates on the
feature branch: `go build ./...` ✅, `go vet ./...` exit 0, `go test ./...` PASS,
`golangci-lint run` exit 0 (gofmt local-CRLF false positive; CI-LF gate
authoritative, 4/4 green).

## Release readiness

**SHIPPED** — merged, archived, and reconciled. PR #172 CI 4/4 green. The feature
change is additive CLI commands + two behavior-preserving core extractions; closure
branch carries docs + backlog archival only (no production code change).

## Monitoring

None required. The change adds five CLI command families mirroring existing MCP
operations, flips the registry map (guarded by the drift + flag-parity gate), and
performs two behavior-preserving core extractions. No new telemetry, external
contract, or persistence schema. The durable guards are the `TestRegistryParity_*`
and `CLI Reference Drift` CI gates on `main`.

## Rollback

`git revert a8e07ea38f8e153e9a29def264538bcab8222868` (single merge commit). The
change is isolated to `internal/cli/*` (new command families + tests),
`internal/core/*` (behavior-preserving `AppendComment`/`GetLinks` extractions),
`.autoharness/backlog-registry.yaml` (10 op-map rows), and docs. Zero
data-migration or schema impact. Reverting removes the new CLI commands and restores
the prior `mcp_only` rows — the drift gate would then flag it, which is the intended
guard.

## Source artifact cleanup

- **`custom_fields.source_stash_id`**: **absent** on `079-F` (only
  `harness_status: pending`), same as `078-F`. Automated stash retirement is a
  **no-op**. The originating phase-2 stash **`6C6ACE00`** is **already absent** from
  the active `.backlogit/stash.jsonl` (retired by Stage during harvest). Flagged
  only, not forced.
- **`custom_fields.source_deliberation_id`**: **absent** as a structured field on
  `079-F`, but the deliberation backlog artifact **`051-DL`** was **co-archived** by
  `backlogit shipment ship` as part of the released scope (present in `archived_ids`,
  now at `.backlogit/archive/051-DL.md`). The docs-file deliberation
  `docs/decisions/2026-07-03-cli-mcp-command-parity-phase2-deliberation.md` remains
  on `main` as the durable decision record (referenced by `079-F`).

## Knowledge graduation

- **New compound learning captured** (evidence-backed, lint-clean):
  `docs/compound/2026-07-04-core-extraction-shared-eventwriter-append-serialization.md`
  — when extracting an MCP write handler into a shared `core` function that both a
  long-lived MCP server and a one-shot CLI reuse, thread the caller's shared
  `*events.EventWriter` through the function (MCP passes `s.Events`; CLI passes
  `nil` for a one-shot writer). Minting a fresh writer per call inside the core
  function drops the server's shared-instance append serialization
  (`EventWriter.mu` only serializes callers sharing one instance).
- **Compound-refresh report**:
  `docs/closure/2026-07-03-079-S-cli-mcp-command-parity-phase2-compound-refresh.md`
  — existing parity entries (`2026-07-03-cli-mcp-honest-fallback-map-and-registry-drift-test`,
  `2026-06-27-cli-mcp-catalog-parity…`, `2026-05-07-mcp-cli-config-parity`)
  classified **keep** (distinct layers); one new capture.
- **Design decision graduation**: the MCP-to-CLI discoverability guide graduated
  during phase-1 (078-S) and is already on `main`; phase-2 extends its coverage via
  the U7 doc update (shipped in the feature PR). No further graduation needed.

## Compact-context

Assessed (`target: all`); **no compaction executed this cycle**. `docs/memory/`
remains below every compaction trigger (file-count / 500 KB / age), and no single
release unit exceeds the 10+-checkpoint mandatory trigger. The 079-S session memory
plus the four `docs/closure/2026-07-03-079-S-…` artifacts are the durable per-unit
record. Archive-only / newest-preserved constraints honored (nothing deleted or
moved).

## Backlog integrity

- `backlogit sync` re-run after all archival + knowledge mutations.
- `079-F`, `079-S`, `051-DL`, and all 14 tasks/subtasks `status: archived` in the
  index.

## Deferred / out-of-scope (left untouched, carried forward to Stage)

- **`merge_sync`** — CLI fallback deferred to **phase-3** (write-by-default; needs
  Rule-4 blast-radius guardrails). Retained `mcp_only` with rationale in the
  registry. Not re-stashed (pre-documented in the plan).
- **`log_telemetry`** — intentional-**permanent** `mcp_only` (read/report-only
  telemetry). Not re-stashed.
- **`export-command-map` pairing enhancement** — deferred to phase-3
  (pre-documented in the plan). Not re-stashed.
- **`EED25928`** (task) — external autoharness `.tmpl` source drift + Direction-3
  harvest-topology follow-up; **out-of-tree, out of scope** (Principle IV). Not
  touched.
- **`21E17BFC`** (feature, low) — singleton MCP server with multiplexed transport.
- **`9140F65C`** (task, low) — fix/enable npm package publishing in Release workflow.
- **`B55985DD`** (task, low) — reword misleading `make docs-lint --path` in historical
  ride-along artifacts.

## Verdict

**SHIPPED** — 079-S is merged (operator-authorized admin merge, merge-commit
strategy, P-009 preserved), archived, and reconciled (pre/post PROCEED, P-007
intact). Runtime verification PASS. One new compound learning graduated (shared
`EventWriter` append serialization). Source stash `6C6ACE00` already retired by
Stage; deliberation `051-DL` co-archived. Deferred phase-3 / permanent-`mcp_only`
items and the four low-priority stashes left untouched per operator instruction.
Remaining: operator P-014 approval of the **closure PR** before it is merged.
