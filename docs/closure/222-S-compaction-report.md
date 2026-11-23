---
title: "222-S P-020 compaction report"
description: "Mandatory post-merge compact-context invocation evidence for Shipment 222-S."
doc_type: closure
source: docs/closure/222-S-compaction-report.md
ingested_at: "2026-11-23T22:51:00Z"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
docline:
  shipment_id: 222-S
  feature_id: 221-F
  compaction_status: done
---

## Invocation

The `compact-context` skill ran with `target: memory/all` after Shipment
`222-S` implementation merge and governed shipment closure. `ENGRAM_DEGRADED`
applied, so no Engram CLI or MCP surface was invoked. This invocation satisfies
the unconditional P-020 post-merge floor.

## Candidate assessment

| Surface | Candidates | Action |
| --- | --- | --- |
| Completed-release memory | 3 | Compacted into one durable summary and moved to date-preserving archive paths |
| Backlog checkpoints | 0 | All checkpoints were excluded |
| Plans | 0 | No qualifying completed plan required consolidation in this run |
| Closure records | 0 | New 222-S closure evidence remained the canonical current record |

## Result

* Files compacted: `3`
* Compacted summaries created: `1`
* Files archived or moved: `3`
* Compacted bytes: `7,248` source bytes
* Summary bytes: `4,494`
* Live `docs/memory/` byte reduction estimate: `2,754` bytes (`38.0%`)
* Active checkpoints preserved: `all`
* Plans preserved: `all`
* Other shipment memories preserved: `all`
* `.gsd`, `.kbdb`, and unrelated workspace state preserved: `yes`
* Closure records compacted: `0`
* Skill result: success
* Final `compaction_status`: `done`

## Trace

Created:

* `docs/memory/compacted/2026-11-23-222-s-221-f-compacted.md`

Moved, without deletion:

* `docs/memory/2026-11-22/222-s-served-root-handoff.md` to
  `docs/archive/memory/2026-11-22/222-s-served-root-handoff.md`
* `docs/memory/2026-11-22/222-s-session-summary.md` to
  `docs/archive/memory/2026-11-22/222-s-session-summary.md`
* `docs/memory/2026-11-23/222-s-final-test-gate-halt.md` to
  `docs/archive/memory/2026-11-23/222-s-final-test-gate-halt.md`

Updated:

* `docs/closure/222-S-compaction-report.md`
* `docs/closure/222-S-221-F-post-merge-closure.md`
* `docs/closure/evidence/222-S-221-F-close-evidence.json`
