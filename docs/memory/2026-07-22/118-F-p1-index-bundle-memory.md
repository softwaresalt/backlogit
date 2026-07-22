---
chunk_strategy: h1-h2-h3
description: 'Session memory — DARK_MODE p1 index bundle: shipped 118-F (composite SQLite index idx_items_parent_type_id serving the batched task-children rollup + read-path comment accuracy fix). Operator-routed p1 bundle 0FA55F47 + FD8C4094. Impl PR #281 merge 63827213 (Copilot clean first round), closure PR #282 merge a9ba323a; 118-F + tasks + shipment 101-S archived on main. Records the 4-persona adversarial panel (Constitution scope P1 = verified false positive from a gitignored diff.txt; SQLite idx_items_parent retention MEASURED, not dropped), the queued->active->done transition-graph requirement, and the remaining 5 operator-gated stash entries left out of scope.'
doc_type: memory
schema_version: "1.0"
source: docs/memory/2026-07-22/118-F-p1-index-bundle-memory.md
title: 118-F p1 index bundle (DARK_MODE) — session memory
---

# 118-F p1 Index Bundle (DARK_MODE) — Session Memory

## Scope

DARK_MODE (P-017), operator AFK, PR + merge pre-authorized, admin fallback
NOT authorized, agent-intercom UNAVAILABLE (degraded visibility → inline
DARK_MODE event logging). Operator routing directive:

> `0FA55F47` as p1 for next round of work and can bundle `FD8C4094` in the
> deliverable.

- `0FA55F47` — add composite SQLite index to speed feature-child lookups used
  by size_composition batched rollups; measure query plan before/after.
- `FD8C4094` — fix a misleading `_txlock` / "deferred read lock acquires a
  writer lock" comment on a transaction-less autocommit read path.

## What Shipped

Feature **118-F**, tasks **118.001-T** (index) + **118.002-T** (comment),
shipment **101-S**.

- Branch `feat/db-task-children-index` (base `d505cca8`); harvest `a9220881`,
  code `3651bba5`, review refinements `6c9ea98c`.
- `internal/db/schema.go`: added
  `CREATE INDEX IF NOT EXISTS idx_items_parent_type_id ON items(parent_id, artifact_type, id)`
  after `idx_items_parent`, with a measurement-cited retention comment.
- `internal/db/queries.go`: reworded the `GetItemsByIDs` and
  `GetTaskChildrenByParentIDs` read-path comments — dropped the counterfactual
  claim that a bare autocommit SELECT takes a writer lock; now "bare autocommit
  SELECT (implicit read transaction); under WAL reads a committed snapshot,
  takes no writer lock, does not block writers."
- Tests: NEW `internal/db/task_children_index_test.go` (EXPLAIN QUERY PLAN,
  3 subtests) + `schema_gen_test.go` column-order assertion.

Query-plan measurement (recorded in 118.001-T):

- BEFORE: `SEARCH items USING INDEX idx_items_type (artifact_type=?)` +
  `USE TEMP B-TREE FOR LAST TERM OF ORDER BY`.
- AFTER: `SEARCH items USING INDEX idx_items_parent_type_id (parent_id=? AND artifact_type=?)`,
  no temp b-tree.
- parent-only: `SEARCH items USING INDEX idx_items_parent (parent_id=?)` —
  narrow index retained (planner prefers it for bare `parent_id=?`).

## Review

4-persona cross-model adversarial panel (SQLite / Go / Constitution / Scope).
Zero P0. Findings adjudicated:

- **Constitution P1 (scope creep)** = **verified FALSE POSITIVE**. The reviewer
  saw a `diff.txt` at repo root showing `internal/core/shipment_gate.go`
  changes. Verification: `diff.txt` is UNTRACKED and gitignored (not on the
  branch); `git diff origin/main...HEAD -- internal/core/` is EMPTY. Branch
  scope is `internal/db/` + `.backlogit/` only.
- **SQLite**: added multi-parent IN test; `idx_items_parent` retention
  empirically MEASURED and JUSTIFIED (not dropped) — see the compound learning;
  "deferred read lock" terminology corrected.
- **Go**: int64 EXPLAIN scans, idiomatic map assertion, coverage expansion.
  Bare `Close()` nits declined (matches the db package convention; lint-clean).
- **Scope P2**: before/after EXPLAIN plan recorded in the 118.001-T artifact.

LOCAL_REVIEW_READY on `6c9ea98c` = READY (zero unresolved P0/P1).

## Copilot Review + Merge

- Impl PR **#281**: Copilot **clean on the first round** ("reviewed 11 of 11
  files, generated no comments"), zero threads. §1.9 passed on `6c9ea98c`;
  CI 4/4 green; P-009 verified (allow_merge_commit only). Merge `63827213`
  (2 parents `d505cca8` + `6c9ea98c`). DARK_MODE_MERGE_AUTHORIZED.
- Closure PR **#282**: tasks + feature `queued→active→done` then
  `shipment claim` + `shipment ship 101-S --sha 63827213`; queue → archive.
  Copilot clean (generated no comments), §1.9 passed on `7a9fa47e`, CI green.
  Merge `a9ba323a` (2 parents `63827213` + `7a9fa47e`). 118-F + tasks + 101-S
  archived on origin/main.

## Gotchas Learned

- **Status transition graph is guarded**: `queued → done` is REJECTED by the
  `validate_status_transition` pre-hook. Valid path is
  `queued → active → done` (map at `internal/hooks/builtin_pre.go:16-23`;
  `done → archived` only via the archive / `shipment ship` path, not a status
  move). Tasks→done runs the F4 completion gate (`evidence_required=true`).
- **`backlogit query` logs to stderr**: an `INFO config loaded` line precedes
  JSON; redirect `2>$null` before `ConvertFrom-Json`.
- Composite-index-does-not-obsolete-narrow-index → captured as a compound
  learning (`docs/compound/2026-07-22-composite-index-prefix-does-not-obsolete-narrow-index.md`).

## Out of Scope / Next

Remaining 5 active stash entries are operator-gated and were NOT executed
(draining them = DARK_MODE scope expansion):

- `7F0A6E89` — EXTERNAL autoharness repo write → Principle IV containment. Cannot perform.
- `8CD8F46A` — persona-dispatch path → routed to full deliberation + decision (operator: p3).
- `0F2E5BA9` — list_items priority/owner parity → routed to deliberation (operator: p2).
- `131CEAE4` — durability/fsync redesign → routed to an isolated spike.
- `9D5BB492` — crash-window exactly-once → routed to a distinct spike.

Per the operator's latest routing, these need their own deliberation/spike
workflows and are left for the next round.
