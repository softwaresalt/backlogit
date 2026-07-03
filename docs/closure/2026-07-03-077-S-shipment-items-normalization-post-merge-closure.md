---
chunk_strategy: h1-h2-h3
description: 'Post-merge operational closure for shipment 077-S — consolidate the duplicated shipment-items read-edge normalization into a single exported core.NormalizeShipmentItems, delete the internal/mcp normalizeShipmentItems copy, and relocate the never-null JSON wire-shape invariant into the core function return contract (PR #168, merge c848740, merged 2026-07-03T20:02:37Z by softwaresalt). Behavior-preserving internal Go refactor (net -93 lines across internal/core + internal/mcp; the mapping logic + all-cases unit test moved to core, the MCP end-to-end never-null guard retained). Records the confirmed merge (merge commit, P-009 preserved), PR #168 CI 4/4 green (test 1.23, test 1.24, CLI Reference Drift, Docline frontmatter gate), the shipment ship result (077-S shipped; 077.001-T/077-F/077-S archived with the merge SHA recorded; pre/post shipment-reconcile both PROCEED; P-007 archive integrity intact with only intended queue->archive moves and no spurious archive deletions), whole-suite gates green on the closure branch (go test PASS, go vet 0, golangci-lint 0; gofmt local-CRLF false positive noted, CI LF gate authoritative), runtime verification PASS (never-null wire shape enforced by the core return contract, exercised by the moved core suite + the retained MCP end-to-end guard), release-readiness SHIPPED, no monitoring and git-revert rollback for the zero-blast-radius internal refactor, backlog integrity (backlogit sync re-run; doctor shows only the known pre-existing unrelated orphan 016.001-R with no new orphans or duplicates), knowledge graduation (no new compound learning and no duplicate — the never-null / single-source-of-truth pattern is already recorded in the covering-feature and exported-cache learnings; 077-S is a targeted consolidation of an already-shipped invariant), compact-context assessment (no compaction executed — below all thresholds), and the low-priority follow-up stash entries carried forward to Stage (7ECBAC7E, E16F4664, EED25928, B55985DD, 21E17BFC, 9140F65C).'
doc_type: closure
docline:
    ms.date: 2026-07-03T00:00:00Z
    ms.topic: reference
ingested_at: "2026-07-03T00:00:00Z"
schema_version: "1.0"
source: docs/closure/2026-07-03-077-S-shipment-items-normalization-post-merge-closure.md
title: 077-S shipment-items normalization — Post-Merge Operational Closure
---

# Operational Closure — 077-S consolidate shipment-items normalization (post-merge)

- **Shipment**: `077-S` · Feature `077-F` · Task `077.001-T`
- **PR**: #168 (`feat/077-shipment-items-normalization` → `main`)
- **Merge commit**: `c8487407d5ddb19d26c754ce82606df929e35f46`
- **Merged**: 2026-07-03T20:02:37Z by `softwaresalt` (Derek Williams)
- **Closure branch**: `post-merge/077-shipment-items-normalization`

> **Interruption context**: PR #168 merged successfully, but the Ship post-merge closure was
> interrupted by a devbox Windows update before it ran. At resume, `077.001-T` was already
> `archived` while `077-F` and `077-S` remained `active` with no closure/runtime-verification
> artifacts. This closure completes the interrupted tail (feature done → shipment archived →
> reconcile → artifacts → knowledge/compaction assessment).

## Merge

- Merge method: **merge commit** (P-009 preserved) — PR #168 `state: MERGED`,
  `mergeCommit.oid: c8487407…`, same pattern as prior shipments.
- Merge SHA `c8487407…` is the current local `main` HEAD (`git rev-parse HEAD`).
- CI on PR #168 at merge HEAD: **4/4 green** — `test (1.23)`, `test (1.24)` (required),
  `CLI Reference Drift`, `Docline frontmatter gate`.

## P-009 merge strategy verification

Merge commit is the recorded strategy (`mergeCommit.oid` present, no squash/rebase rewrite). The
repository ruleset permits only merge commits — no P-009 violation.

## Scope of the shipped change

**Behavior-preserving internal Go refactor** (net **-93** lines; 408 insertions / 93 deletions
across 16 files, most of which is the test relocation and the plan/memory/backlog artifacts). One
task, `077.001-T` ("Export core.NormalizeShipmentItems and delete MCP duplicate"):

- `internal/core/shipment.go` — `shipmentItems` renamed to exported **`NormalizeShipmentItems`**
  and hardened to **never return nil** (nil artifact / nil `CustomFields` / missing `items` / nil
  raw → `[]string{}`; `[]string` copied; `[]any` string-filtered). Single source of truth.
- `internal/mcp/tools.go` — the duplicated inline `normalizeShipmentItems` mutator was **deleted**;
  `handleListShipments` now delegates to `core.NormalizeShipmentItems` (thin adapter, nil-map guard
  retained solely as an assignment target).
- `internal/core/shipment_covering.go`, `shipment_lifecycle.go`, and core tests — call-site renames.
- Test consolidation: the all-cases mapping unit test **moved** to
  `internal/core/shipment_normalize_test.go` (with an added empty-`[]string` case); the end-to-end
  never-null guard `TestListShipments_EmptyItems_NeverNull` **stays** in `internal/mcp`.

Root motivation: remove the duplicated `[]any`→`[]string` switch (a logical true-duplicate where the
MCP copy was a mutator and the core copy a pure reader, diverging on the empty-`[]string` edge) and
give the never-null JSON wire-shape one authoritative home. Source: stash `17D29DDC`, deferred from
the 075-F plan-review.

## Shipment ship

- `backlogit shipment ship 077-S --sha c8487407… --message "Merge pull request #168 …"
  --author softwaresalt` → `shipment_status: shipped`.
- `archived_ids`: `077.001-T`, `077-F`, `077-S` (3). `returned_ids`: none.
- Archived shipment `077-S.md` carries `commit: c8487407d5ddb19d26c754ce82606df929e35f46`.
- **Reconcile**: pre-mode (`expected: done`) → **PROCEED** (`077.001-T` archived during the build
  loop; `077-F` moved `active` → `done` on the closure branch before the gate, routing to archive).
  post-mode → **PROCEED** (all three archive files present: `077-F.md`, `077-S.md`, `077.001-T.md`).
- **P-007**: `git status -- .backlogit/archive .backlogit/queue` shows the two new archive files as
  untracked additions (`??` `077-F.md`, `077-S.md`), `077.001-T.md` modified (commit metadata), and
  the intended moves as deletions (`D`) under `.backlogit/queue/` only — **zero deletions under
  `.backlogit/archive/`**. No `git restore` needed.

## Runtime verification

**PASS** — see
`docs/closure/2026-07-03-077-S-shipment-items-normalization-runtime-verification.md`. Whole-suite
gates on the closure branch: `go test ./...` **PASS**, `go vet ./...` exit 0, `golangci-lint run`
exit 0. The never-null wire shape is enforced by the core return contract and exercised by the
moved core unit suite (nil/missing/`[]string`/`[]any`/empty-`[]string`, never-nil) plus the retained
MCP end-to-end guard (`custom_fields.items` is always a JSON array, never `null`). Behavior
preserved across the renamed call sites.

## Release readiness

**SHIPPED** — merged, archived, and reconciled. PR #168 CI 4/4 green. No code change on the closure
branch (docs + backlog archival only).

## Monitoring

None required. The change consolidates an existing normalization path and introduces no new runtime
surface, telemetry, external contract, or persistence.

## Rollback

`git revert c8487407d5ddb19d26c754ce82606df929e35f46` (single merge commit). The change is isolated
to `internal/core/shipment*.go` and `internal/mcp/tools.go` (plus tests and backlog/plan/memory
artifacts). Zero data-migration or schema impact. The pre-refactor behavior is a strict subset of
the post-refactor never-null contract, so reverting only reintroduces the empty-`[]string` nil edge.

## Source artifact cleanup

- **Source stash `17D29DDC`** (consolidate shipment-items normalization): **already archived** by
  Stage during harvest — present in `.backlogit/archive/stash.jsonl` with a forward-link to
  `077-F` / `077.001-T` / `077-S`, and confirmed **absent** from the active `.backlogit/stash.jsonl`.
- **Automated retirement**: no-op — `077-F` carries no structured `custom_fields.source_stash_id`
  (only `harness_status: pending`), same situation as `071-F`–`076-F`. Stash retirement is
  Stage-domain and already complete; **flagged only, not forced.**
- **Deliberation artifact**: none — `077-F` has no `source_deliberation_id` (deliberate SKIPPED /
  folded into the plan for this solo trivial refactor, per the Stage session memory).

## Knowledge graduation

**No new compound learning and no duplicate.** The two invariants at the heart of 077-S —
never-null JSON wire shape and a single shared shaper as the source of truth — are already recorded
in `docs/compound/best-practices/exported-cache-zero-value-bypass-2026-06-29.md` (fail-closed / guard
the nil precondition) and reinforced by the 075-S covering-feature closure (shared `core.ShipmentView`
shaper, items-never-null). 077-S is a targeted consolidation of an already-shipped invariant rather
than a new hard-won lesson, so the compound library is left as-is (checked; no stale guidance to
refresh).

## Compact-context

Assessed (`target: all`); **no compaction executed this cycle**. `docs/memory/` remains below every
compaction trigger (file-count / 500 KB / 14-day age), and no single release unit exceeds the
>10-checkpoint mandatory trigger. The 077-S closure-session memory plus the two new
`docs/closure/2026-07-03-077-S-…` artifacts are the durable per-unit record. Archive-only and
newest-preserved constraints honored (nothing deleted or moved).

## Backlog integrity

- `backlogit sync` re-run after all archival mutations (`Indexed 679 artifacts`).
- `backlogit doctor` → only the **known, pre-existing, unrelated** orphan `016.001-R` (a review
  artifact with no `parent_id`). **No new orphans or duplicates introduced by 077-S.**
- `077-F`, `077-S`, `077.001-T` all `status: archived` in the index.

## Follow-ups carried forward to Stage

- `E16F4664` (feature, **medium**) — **highest-priority pending** — audit CLI command parity vs the
  MCP tool surface + registry, fill or document gaps, improve discoverability, and define a clean
  CLI fallback when MCP is unavailable. Operator request 2026-07-03; broader superset of `7ECBAC7E`.
- `7ECBAC7E` (task, low) — close the CLI `shipment list` null-vs-`[]` parity gap (normalize via
  `core.NormalizeShipmentItems` in the CLI list handler + a cross-surface shape-consistency guard
  test). Deferred from this shipment's plan-review to keep 17D29DDC in scope.
- `EED25928` (task, low) — deferred harvest-delivery topology + upstream `.tmpl` docline drift.
- `B55985DD` (task, low) — reword the ride-along `make docs-lint --path` wording.
- `21E17BFC` (feature, low) — singleton MCP server with multiplexed transport (contingency).
- `9140F65C` (task, low) — fix/enable npm package publishing in the Release workflow.

## Verdict

**SHIPPED** — 077-S is merged, archived, and reconciled. Runtime verification PASS (never-null wire
shape enforced by the single `core.NormalizeShipmentItems` contract; MCP duplicate deleted);
knowledge left as-is (already covered, no duplicate); source stash `17D29DDC` retired by Stage; six
prior/related low-and-medium-priority stash entries carried forward. Remaining: operator P-014
approval of the **closure PR** before it is merged.
