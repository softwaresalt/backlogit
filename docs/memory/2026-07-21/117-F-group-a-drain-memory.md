---
chunk_strategy: h1-h2-h3
description: 'Session memory — DARK_MODE Group A drain: shipped 117-F (size_composition parity on flat list read surfaces + batched rollup API + staleness doc; impl PR #277 merge 2fc776b0, closure PR #278 merge b95f80f3, 117-F archived on main). Records the 7-persona adversarial review adjudication, the 3-round Copilot review convergence (2->2->0) including the substantive omitempty-defeats-arrays-always contract catch, and the halt decision leaving 4 operator-gated EXCLUDED + 3 new low-priority follow-up stash entries out of scope.'
doc_type: memory
schema_version: "1.0"
source: docs/memory/2026-07-21/117-F-group-a-drain-memory.md
title: 117-F Group A drain (DARK_MODE) — session memory
---

# 117-F Group A Drain (DARK_MODE) — Session Memory

## Scope

DARK_MODE (P-017) dark-factory pipeline, operator AFK, PR + merge
pre-authorized, admin fallback NOT authorized, agent-intercom UNAVAILABLE
(degraded visibility → inline DARK_MODE event logging). Requested scope:
drain "Group A" — the self-generated size_composition follow-up stash
entries deemed safe and in-scope.

Group A = stash `60336CC0` (list-surface parity), `A6A1B47E` (batched
rollup API / N+1), `7063A9F4` (staleness doc), all harvested into feature
**117-F**.

## Completed

- **117-F shipped + archived.** Impl PR **#277** → merge commit `2fc776b0`
  (2 parents `ca8d0c61` + `b2a3d1f9`, P-009 verified). Closure PR **#278** →
  merge commit `b95f80f3` (2 parents `2fc776b0` + `b07e15fc`). 117-F +
  children 117.001-T / 117.002-T archived on origin/main; provenance
  `60336CC0` preserved; merge commit `2fc776b0` tracked on 117-F.
- **Tasks**: 117.001-T (batched rollup API) done; 117.002-T (staleness doc)
  done; 117-F done → archived (cascade-archived done children).

## What shipped (117-F)

- `db.GetTaskChildrenByParentIDs` + `core.SizeCompositions` batched rollup
  (chunked at 900 IDs/query; features referenced by shipment manifests are
  prefetched so batched output == per-artifact output).
- Flat-list parity: `core.ListWithSizeComposition` projects the rollup onto
  CLI `list --json` and MCP `list_items`; tool description discloses the field.
- Shared `core.QueueViewWithSizeComposition` wired to BOTH CLI
  `queue view --json` and MCP `get_queue` (deleted the two duplicate shapers);
  degrades to unprojected payload (warn + nil err) on rollup error so the two
  transports cannot drift on projection or degradation.
- Human render N+1 removed via `batchCompositions` (table/tile/grouped).
- `SizeCompositionResult.Skipped` `omitempty` dropped so it always emits `[]`.
- Durable sizing-contract doc: read-time freshness / best-effort multi-read
  view (not a point-in-time snapshot) + chunked-batching (not O(1)) caveats.

## Reviews

- **Adversarial 7-persona cross-model panel** (Constitution, Go, Learnings,
  Architecture, SQLite, Scope, Agent-Native Parity). Consensus P1 (duplicated
  + inconsistent queue projection across CLI/MCP) → resolved via the shared
  core shaper. Scope P1 (human render N+1) → resolved via `batchCompositions`.
  0 unresolved P0/P1 at LOCAL_REVIEW_READY (`READY_WITH_FOLLOWUPS`).
- **Copilot review converged 2 → 2 → 0 across 3 rounds** (all threads replied
  + programmatically resolved via `gh api graphql resolveReviewThread`):
  - r1: "chunked batching not O(1)" doc-accuracy (core comment + design doc).
  - r2: "point-in-time snapshot" → best-effort multi-read view; **substantive**:
    `Skipped omitempty` defeats the arrays-always-`[]` contract (empty →
    field absent, not `[]`) while `histogram`/`members` have no omitempty →
    dropped omitempty + added CLI/MCP empty-case `skipped: []` assertions.
  - r3: clean (zero new threads). §1.9 gate passed on `b2a3d1f9`.

## Halt decision (Group A complete)

Remaining 7 active stash entries are ALL out of declared Group A scope;
draining them would be scope expansion (DARK_MODE stop condition), so halted:

- **4 EXCLUDED (operator-gated)**: `131CEAE4` (fsync/durability redesign — high
  blast radius), `7F0A6E89` (external autoharness repo write — Principle IV
  containment violation, cannot perform), `8CD8F46A` (plan-review enable-vs-waive
  — governance/waiver), `9D5BB492` (crash-window exactly-once — product-gated).
- **3 new low-priority follow-ups** deferred from the 117-F review with
  rationale: `0F2E5BA9` (evaluate list_items priority/owner request-contract
  parity — needs a cross-transport deliberation, not a one-sided add),
  `0FA55F47` (composite index `idx_items_parent_type_id` — needs perf
  measurement), `FD8C4094` (fix misleading `_txlock` comment on
  `db.GetItemsByIDs` — trivial, pre-existing).

## Key files

- `internal/core/size_composition.go` — `SizeCompositions` (batched, chunked),
  `QueueViewWithSizeComposition` (shared shaper, graceful degradation),
  `ListWithSizeComposition`, `SizeCompositionResult` (Skipped no longer omitempty).
- `internal/cli/queue_cmd.go` (delegates to core shaper), `internal/cli/list.go`
  (`batchCompositions`), `internal/mcp/tools.go` (get_queue delegates; list_items
  description discloses field).
- `docs/design-docs/2026-07-19-size-estimation-contract.md` — freshness +
  batching caveats.

## Next steps

- Operator triage of the 4 EXCLUDED stash entries (each needs a decision).
- Optional: regroup the 3 new low-priority follow-ups into a future shipment
  (`0F2E5BA9` first needs a deliberation on cross-transport request-contract
  symmetry before any list_items filter widening).
