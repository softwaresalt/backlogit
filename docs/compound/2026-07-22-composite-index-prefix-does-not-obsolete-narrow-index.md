---
chunk_strategy: h1-h2-h3
schema_version: "1.0"
title: "SQLite composite index vs its narrow prefix index: planner preference while both exist is not proof the narrow index is non-redundant (the composite can serve the bare prefix lookup too) — decide retain-vs-drop with a post-removal EXPLAIN QUERY PLAN plus a benchmark, and scope any ORDER BY sort-elision claim to the exact IN(...) arity measured"
source: docs/compound/2026-07-22-composite-index-prefix-does-not-obsolete-narrow-index.md
doc_type: learning
description: "When you add a composite index like idx_items_parent_type_id ON items(parent_id, artifact_type, id) to serve a filtered+ordered query (WHERE parent_id IN (...) AND artifact_type='task' ORDER BY parent_id, id), do NOT assume you can drop the pre-existing narrow index idx_items_parent ON items(parent_id) just because parent_id is the composite's leading column — but also do NOT assume the narrow index is proven non-redundant merely because EXPLAIN QUERY PLAN shows the planner preferring it while both indexes exist. Two distinct nuances bit this review. (1) ORDER BY sort-elision is IN-arity-sensitive: for a single-parent shape (WHERE parent_id IN (?) ... ORDER BY parent_id, id) the composite supplies the ordering and the plan shows NO USE TEMP B-TREE, but for a multi-row IN (?,?,...) the temp-b-tree elision is planner-version-sensitive, so assert only index SELECTION for the multi-parent case and reserve the no-temp-b-tree assertion for the single-parent shape. Scope any no-temp-b-tree claim to the exact IN arity you measured, or the learning contradicts itself. (2) Planner preference is not proof of non-redundancy: because parent_id is the composite's leading column, the composite CAN serve a bare WHERE parent_id = ? as an index seek too; when both indexes exist the planner merely prefers the physically smaller narrow index. So dropping the narrow index would fall back to the composite (still an index seek, marginally wider b-tree), not to a table scan. To justify retain-vs-drop you therefore need a representative benchmark AND a post-removal EXPLAIN QUERY PLAN confirming the fallback plan and cost, not just a both-exist preference read. We retained idx_items_parent as the conservative default (dropping an existing index is the riskier direction and the marginal write/storage saving was not worth a benchmark this round). Also: modernc.org/sqlite returns EXPLAIN QUERY PLAN integer columns as int64 — scan id/parent/notused into int64, not int."
docline:
    date: 2026-07-22T00:00:00Z
    severity: low
    tags:
        - sqlite
        - modernc-sqlite
        - index
        - composite-index
        - query-plan
        - explain-query-plan
        - db
        - performance
        - order-by-elision
---

# Composite Index vs Its Narrow Prefix Index — Preference Is Not Proof

## Context

Feature 118-F added a composite index to speed the batched task-children
rollup used by `size_composition`:

```sql
CREATE INDEX IF NOT EXISTS idx_items_parent_type_id
    ON items(parent_id, artifact_type, id);
```

The query it serves (`internal/db/queries.go`, `GetTaskChildrenByParentIDs`):

```sql
SELECT ... FROM items
WHERE parent_id IN (?, ?, ...) AND artifact_type = 'task'
ORDER BY parent_id, id;
```

An adversarial SQLite review raised a plausible P3: since the new composite
index has `parent_id` as its leading column, is the pre-existing
`idx_items_parent ON items(parent_id)` now dead weight that should be dropped?

## The Trap

Two tempting-but-wrong inferences pull in opposite directions:

**(A) "The narrow index is a strict column-prefix of the composite, therefore it
is redundant."** Prefix coverage is necessary but not sufficient to justify a
drop. But do not over-correct into:

**(B) "EXPLAIN QUERY PLAN shows the planner preferring the narrow index,
therefore it is proven non-redundant."** Also wrong. Because `parent_id` is the
composite's leading column, the composite CAN serve a bare `WHERE parent_id = ?`
as an index seek too. When both indexes exist the planner merely prefers the
physically smaller narrow index. Dropping it would fall back to the composite
(still an index seek, just a wider b-tree) — a marginal cost, not a table-scan
cliff. Deciding retain-vs-drop needs a benchmark plus a post-removal plan check,
not a both-exist preference read.

## Measured Behavior (EXPLAIN QUERY PLAN)

Sort-elision is sensitive to the `IN` arity, so the shapes are kept separate:

| Query shape | Plan |
| --- | --- |
| single-parent `WHERE parent_id IN (?) AND artifact_type='task' ORDER BY parent_id, id` (BEFORE composite) | `SEARCH items USING INDEX idx_items_type (artifact_type=?)` + `USE TEMP B-TREE FOR LAST TERM OF ORDER BY` |
| single-parent `IN (?)` (AFTER composite) | `SEARCH items USING INDEX idx_items_parent_type_id (parent_id=? AND artifact_type=?)` — no temp b-tree; the index supplies the ordering |
| multi-parent `IN (?,?,...)` (AFTER composite) | composite is SELECTED (`... USING INDEX idx_items_parent_type_id`); temp-b-tree elision here is planner-version-sensitive, so treat this as index-selection only |
| bare `WHERE parent_id = ?` (both indexes present) | `SEARCH items USING INDEX idx_items_parent (parent_id=?)` — the planner **prefers the narrow index**; the composite could also serve this via its leading column |

Retention decision: `idx_items_parent` was kept as the conservative default —
dropping it would fall back to the composite (still an index seek), and the
marginal write/storage saving was not worth a benchmark this round.

## How to Lock It

`internal/db/task_children_index_test.go` uses an `explainPlan` helper and
three subtests:

1. **single-parent `IN (?)`** — assert `idx_items_parent_type_id` is used AND no
   `USE TEMP B-TREE` (the only shape where sort-elision is stable enough to
   assert).
2. **multi-parent `IN(?,?)`** — assert `idx_items_parent_type_id` is used
   only (do NOT assert sort elision; it is planner-version-sensitive on
   multi-row IN).
3. **parent-only** — assert `idx_items_parent (` is used AND
   `idx_items_parent_type_id` is NOT (disambiguate the substring overlap by
   matching the trailing `(`).

## Gotchas

* `modernc.org/sqlite` returns EXPLAIN QUERY PLAN integer columns as `int64`
  — scan `id`, `parent`, `notused` into `int64`.
* When asserting `idx_items_parent` is used but the composite is not, the
  composite name **contains** the narrow name as a substring; match
  `"idx_items_parent ("` (with the trailing space+paren) and separately assert
  `NotContains "idx_items_parent_type_id"`.

## Rule

A prefix relationship does not by itself make the narrow index droppable — but
neither does mere planner preference make it provably non-redundant, since the
composite can serve the bare-prefix lookup via its leading column. To decide
retain-vs-drop:

1. Run EXPLAIN QUERY PLAN on the bare-prefix shape **after** hypothetically
   removing the narrow index, to see the actual fallback plan (it will be the
   composite, still an index seek) and its cost.
2. Add a representative benchmark when the fallback is an index seek — the
   difference is marginal, so a both-exist preference read is not evidence.
3. Default to conservative retention when the saving is unmeasured; dropping an
   existing index is the riskier direction.

And always scope an ORDER BY sort-elision assertion to the exact `IN` arity you
measured. Measure; do not assume — in either direction.
