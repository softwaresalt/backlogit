---
chunk_strategy: h1-h2-h3
schema_version: "1.0"
title: "197-S P-020 compaction report"
description: "compact-context target all for the 197-S closure. Additive summary written; verbose originals retained because checkpoints, stash entries, and the governing plan reference them by path. Compaction status degraded, non-blocking."
doc_type: closure
source: docs/closure/2026-10-11-197-s-compaction-report.md
docline:
  date: 2026-10-11T01:52:00Z
  status: draft
  tags:
    - compaction
    - p-020
    - 197-S
---

# 197-S P-020 compaction report

## Invocation

* Skill: `compact-context`, `target: all`, default thresholds (`threshold_days` 14, `max_files` 40, `max_size_kb` 500).
* Result: compaction status `degraded` (non-blocking). The invocation happened, and the additive summary was written. Archive moves were deferred.

## Phase 1 and 2: assessment and candidates

| Area | Candidates | Decision |
|---|---|---|
| `docs/memory/2026-10-10/` and `docs/memory/2026-10-11/` | 10 files, 97,764 bytes, all for the completed 197-S release unit | Summarized additively into `docs/memory/compacted/2026-10-11-197-S-compacted.md`. Originals retained. |
| `docs/closure/` | No closure older than the threshold for this unit | Not compacted. The 197-S closure is new. |
| `docs/exec-plans/` | Plan `2026-10-08-cx-s-ship-closure-gate-correctness-plan.md` has appended review and erratum content | Not consolidated. Creating a decided-plan is a plan artifact, Stage-owned under P-010. Deferred to Stage. |
| Backlog checkpoints (`.backlogit/checkpoints/`) | Nine active `ship` checkpoints for 197-S | Preserved as active pointers. Resolved only at completion (see below). Not compacted. |
| Stale checkpoint for 153-S | Active, from 2026-10-08 | Untouched, as directed. |

## Phase 3: compaction actions

* Memory compaction: one additive summary written. No originals moved, so no space was recovered.
* Plan consolidation: none. Deferred to Stage.
* Closure compaction: none. Nothing older than the threshold.

## Phase 4: report

| Metric | Value |
|---|---|
| Files compacted (originals moved) | 0 |
| Additive summaries written | 1 |
| Space recovered | 0 KB |
| Active task checkpoints preserved | 9 (197-S, ship) |
| Plans consolidated into decided-plans | 0 (Stage-owned) |
| Closure records compacted | 0 |

## Why archive moves were deferred

The skill moves verbose originals to `docs/archive/`. Here, checkpoints, stash entries
(for example `B53056C0` and the follow-up IDs in the closure), and the governing plan
reference these files by path. Moving them now would break traceability and the
Stage-owned plan references. This is the same reason and the same `degraded` status as
the 196-S closure. Stage can perform the archive moves in a later cycle, after it
updates the references.

## Status

`compaction_status: degraded` (non-blocking, P-020).
