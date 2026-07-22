---
chunk_strategy: h1-h2-h3
schema_version: "1.0"
title: "A composite index does not make its narrow single-column prefix index redundant — SQLite's planner may still prefer the narrow index for a bare equality on the leading column, so verify retention with EXPLAIN QUERY PLAN before dropping"
source: docs/compound/2026-07-22-composite-index-prefix-does-not-obsolete-narrow-index.md
doc_type: learning
description: "When you add a composite index like idx_items_parent_type_id ON items(parent_id, artifact_type, id) to serve a filtered+ordered query (WHERE parent_id IN (...) AND artifact_type='task' ORDER BY parent_id, id), it is tempting to also drop the pre-existing narrow index idx_items_parent ON items(parent_id) as a now-redundant prefix. That reasoning is wrong. SQLite's query planner weighs index size/selectivity, not just column-prefix coverage: for a bare `WHERE parent_id = ?` lookup with no secondary filter, the planner prefers the NARROWER single-column index (SEARCH items USING INDEX idx_items_parent (parent_id=?)) because it is physically smaller and cheaper to scan than the wider composite. Dropping the narrow index would silently regress every parent-only call site (e.g. the ListItems parent filter, hierarchy walks, adopt/doctor orphan checks) onto the fatter composite. The decisive tool is EXPLAIN QUERY PLAN run against BOTH query shapes: (1) the composite-serving shape must show `SEARCH ... USING INDEX <composite> (parent_id=? AND artifact_type=?)` with NO `USE TEMP B-TREE FOR ... ORDER BY` (the composite supplies the ordering, eliding the sort); (2) the bare-prefix shape must still show the narrow index. Lock both invariants in a test: assert index USE by name and assert absence of the temp b-tree on the single-parent case (sort elision on multi-row IN(...) is planner-version-sensitive, so only assert index USE there, not sort elision). Note modernc.org/sqlite returns EXPLAIN QUERY PLAN integer columns as int64 — scan the id/parent/notused columns into int64, not int. Bottom line: a prefix relationship is necessary but NOT sufficient for redundancy; measure, do not assume."
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

# Composite Index Prefix Does Not Obsolete the Narrow Index

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

"`idx_items_parent` is a strict column-prefix of `idx_items_parent_type_id`,
therefore it is redundant" is a **false** inference. Column-prefix coverage is
necessary but not sufficient for redundancy. SQLite's planner costs indexes by
physical size and selectivity, not by prefix containment alone.

## Measured Behavior (EXPLAIN QUERY PLAN)

| Query shape | Plan |
| --- | --- |
| `WHERE parent_id IN (...) AND artifact_type='task' ORDER BY parent_id, id` (BEFORE composite) | `SEARCH items USING INDEX idx_items_type (artifact_type=?)` + `USE TEMP B-TREE FOR LAST TERM OF ORDER BY` |
| same query (AFTER composite) | `SEARCH items USING INDEX idx_items_parent_type_id (parent_id=? AND artifact_type=?)` — no temp b-tree; the index supplies the ordering |
| bare `WHERE parent_id = ?` | `SEARCH items USING INDEX idx_items_parent (parent_id=?)` — the planner **prefers the narrow index** |

So the narrow index is retained: dropping it would push every parent-only
lookup onto the wider composite.

## How to Lock It

`internal/db/task_children_index_test.go` uses an `explainPlan` helper and
three subtests:

1. **single-parent** — assert `idx_items_parent_type_id` is used AND no
   `USE TEMP B-TREE` (stable sort-elision assertion).
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

A prefix relationship between two indexes is necessary but not sufficient to
call the narrow one redundant. Before dropping a narrow index that a composite
appears to subsume, run EXPLAIN QUERY PLAN on the bare-prefix query shape and
confirm the planner does not prefer the narrow index. Measure; do not assume.
