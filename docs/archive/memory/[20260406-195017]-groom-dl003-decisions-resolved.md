---
title: "Groomer Checkpoint: DL003 Operator Decisions Resolved"
description: Checkpoint after operator resolved all DL003 open questions with direction changes
ms.date: 2026-04-07
---

## Session Context

Continuation from `[20260406-192757]-groom-deliberation-dl003-checkpoint.md`. Operator reviewed DL003 and provided binding direction on all open questions plus a major direction override.

## Operator Decisions (2026-04-07)

### Direction Override: Naming Overhaul Approved

Operator **rejected** the recommendation to keep the existing prefix system. The numeric hierarchy naming overhaul from 3C7BCC11 is approved as **non-negotiable**. Rationale: no external dependencies exist, and internal migration via temporary scripts is expected and acceptable work.

New ID format: `001-F`, `001.001-T`, `001.001.001-ST`, `001.001.001-B`

### Resolved Open Questions

| # | Question | Decision | Rationale |
|---|----------|----------|-----------|
| 1 | Bug level placement | Level 3 default, **configurable** to 2 or 3 | Mirrors Azure DevOps flexibility |
| 2 | Custom fields migration | **Migrate** to item_links | item_links are durable domain metadata; custom_fields are untyped property bags |
| 3 | Status cascade | **Blocking** | Advisory silently swallows unhandled work |
| 4 | Orphan adoption ID | **Keep provenance IDs** | Easier to maintain than cascading renames |

## Updated Implementation Streams (5 total)

1. **Naming overhaul** (3C7BCC11) — numeric hierarchy + migration scripts
2. **Bug parenting** (BA3DB37B) — configurable level placement
3. **Typed relationship links** (AA10AF37, 6A545842) — item_links table + custom_fields migration
4. **Status reconciliation** (CE39AE5D) — blocking bidirectional cascade
5. **Orphan lifecycle** (51B11D29/DL002) — parent_id null + adopt/reparent

**Dependency order**: Stream 1 → Stream 2 → Streams 3+4 (parallel) → Stream 5

## Artifacts

| Artifact | Status |
|----------|--------|
| DL003 | Updated with operator decisions |
| DL002 | Subsumed by DL003 Stream 5 |

## Next Steps

1. Invoke `impl-plan` on DL003 to generate implementation plan
2. `plan-review` gate
3. `harvest` into shipment-ready backlog
