---
title: "221-S applied context compaction"
description: "Mandatory P-020 context compaction report for Shipment 221-S (Feature 220-F)."
doc_type: closure
source: docs/closure/2026-11-22-221-s-compaction-report.md
ingested_at: "2026-11-22T03:02:00Z"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
docline:
  date: "2026-11-22"
  shipment_id: 221-S
  feature_id: 220-F
  target: all
  compaction_status: done
  threshold_days: 14
  max_files: 40
  max_size_kb: 500
---

## Trigger and corpus assessment

Invoked P-020 `compact-context` with `target: all` and default thresholds (`threshold_days: 14`, `max_files: 40`, `max_size_kb: 500`) after governed closure of shipment `221-S`.

| Directory | Files before compaction | Bytes before compaction | Threshold status |
| --- | --- | --- | --- |
| `docs/memory/` | 394 | 2,400,163 | Exceeds `max_files` (`394 > 40`) and `max_size_kb` (`2,344 KiB > 500 KiB`) |
| `docs/plans/` | 154 | 5,921,662 | Preserved (no relocation approval requested) |
| `docs/exec-plans/` | 0 | 0 | Within threshold |
| `docs/closure/` | 207 | 1,272,131 | Assessed for completed scope |

## Scoped compaction actions (`221-S`)

All `221-S` tasks (`220.001-T`, `220.002-T`), covering feature `220-F`, and shipment `221-S` are `archived`. Evaluated all `221-S` memory and closure artifacts:

* Created dense compacted summary [221-S compacted memory](../memory/compacted/2026-11-22-221-s-220-f-compacted.md) consolidating `docs/memory/2026-11-21/221-s-served-root-handoff.md` and `docs/memory/2026-11-21/221-s-session-summary.md`.
* Preserved current-session `221-S` memory and closure files in place (`< 1` day old, inside `threshold_days: 14`, referenced by PR `#670` and closure PR, and honoring the `compact-context` constraint to preserve the most recent checkpoint per completed scope without deleting files).
* Preserved `docs/plans/2026-11-20-mcp-session-path-and-tools-dir-docs-plan.md` in place per `compact-context` relocation-approval rule; older unrelated `docs/memory/` files outside `221-S` are deferred from bulk relocation to avoid expanding closure scope.

## Counts and completion status

* Files archived: 0 (0 deletions; current-session checkpoints preserved alongside compacted summary)
* Compacted summary files created: 1 (`docs/memory/compacted/2026-11-22-221-s-220-f-compacted.md`)
* Active task checkpoints compacted: 0
* Plans or reviews modified: 0

For `221-S`, the mandatory P-020 compaction obligation is satisfied: `compaction_status: done`.
