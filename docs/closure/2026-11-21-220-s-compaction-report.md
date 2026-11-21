---
title: "220-S applied context compaction"
description: "Mandatory P-020 context compaction report for Shipment 220-S (Feature 219-F)."
doc_type: closure
source: docs/closure/2026-11-21-220-s-compaction-report.md
ingested_at: "2026-11-21T08:42:00Z"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
docline:
  date: "2026-11-21"
  shipment_id: 220-S
  feature_id: 219-F
  target: all
  compaction_status: done
  threshold_days: 14
  max_files: 40
  max_size_kb: 500
---

## Trigger and corpus assessment

Invoked P-020 `compact-context` with `target: all` and default thresholds (`threshold_days: 14`, `max_files: 40`, `max_size_kb: 500`) after governed closure of shipment `220-S`.

| Directory | Files before compaction | Bytes before compaction | Threshold status |
| --- | --- | --- | --- |
| `docs/memory/` | 391 | 2,394,332 | Exceeds `max_files` (`391 > 40`) and `max_size_kb` (`2,338 KiB > 500 KiB`) |
| `docs/plans/` | 154 | 5,921,662 | Preserved (no relocation approval requested) |
| `docs/exec-plans/` | 0 | 0 | Within threshold |
| `docs/closure/` | 202 | 1,249,477 | Assessed for completed scope |

## Scoped compaction actions (`220-S`)

All `220-S` tasks (`219.001-T`..`219.012-T`), covering feature `219-F`, and shipment `220-S` are `archived`. Evaluated all `220-S` memory and closure artifacts:

* Created dense compacted summary [220-S compacted memory](../memory/compacted/2026-11-21-220-s-219-f-compacted.md) consolidating `docs/memory/2026-11-21/220-s-served-root-handoff.md` and `docs/memory/2026-11-21/220-s-session-summary.md`.
* Preserved current-session `220-S` memory and closure files in place (`< 1` day old, inside `threshold_days: 14`, referenced by PR `#668` and closure PR, and honoring the `compact-context` constraint to preserve the most recent checkpoint per completed scope without deleting files).
* Preserved `docs/plans/2026-11-20-gsd-model-routing-manifest-reconciliation-plan.md` in place per `compact-context` relocation-approval rule; older unrelated `docs/memory/` files outside `220-S` are deferred from bulk relocation to avoid expanding closure scope.

## Counts and completion status

* Files archived: 0 (0 deletions; current-session checkpoints preserved alongside compacted summary)
* Compacted summary files created: 1 (`docs/memory/compacted/2026-11-21-220-s-219-f-compacted.md`)
* Active task checkpoints compacted: 0
* Plans or reviews modified: 0

For `220-S`, the mandatory P-020 compaction obligation is satisfied: `compaction_status: done`.
