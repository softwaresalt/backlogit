---
title: Stage Shipment Grouping Triage
description: Stash triage session analyzing 17 entries for staleness and shipment grouping
ms.date: 2026-04-11
---

## Session Summary

Reviewed 17 stash entries and 2 queued deliberations. Identified 4 stale entries
overlapping shipped 026-F work and removed them. Grouped 13 remaining entries
into 5 shipment candidates.

## Stale Entries Removed

| Stash ID   | Concern                          | Addressed By        |
|------------|----------------------------------|---------------------|
| `AE9DB2B6` | SQLite PRAGMAs via DSN           | 026.001-T, 026.002-T |
| `DF8FDB7B` | Semantic links durable           | 026.003-T – 026.006-T |
| `847DCF02` | Markdown-first invariants        | 026.007-T – 026.010-T |
| `C710BEDB` | MCP consistency and cascades     | 026.011-T – 026.015-T |

## Shipment Candidates

### Shipment A: Build and Documentation Integrity

Quick win, high breadth, low risk.

| Stash ID   | Pri    | Kind    | Summary                                   |
|------------|--------|---------|--------------------------------------------|
| `FFF344F2` | high   | task    | Formatting verification (22 unformatted files, Makefile does not fail) |
| `BC78CBDA` | high   | task    | Documentation accuracy (workflow.md, installation.md, AGENTS.md) |
| `E3627E50` | medium | task    | Stash docs (stale subcommand names)        |

### Shipment B: CLI Surface Parity

Single feature focus, substantial scope.

| Stash ID   | Pri  | Kind    | Summary                                    |
|------------|------|---------|--------------------------------------------|
| `D7B72D92` | high | feature | CLI add/update coherence (missing flags, duplicate writes) |

### Shipment C: Core Operations Correctness

Bug fix batch across existing core operations.

| Stash ID   | Pri    | Kind    | Summary                                   |
|------------|--------|---------|--------------------------------------------|
| `6D175713` | high   | feature | Adopt item atomic ID rewrite and file rename |
| `042C1812` | high   | task    | Export command map containment policy       |
| `3F71C20D` | medium | task    | Query-gate semicolons; section-write marker duplication |
| `D6E4B181` | medium | task    | Stash harvest TOCTOU race; rejected status gap |

### Shipment D: Hooks Ecosystem

Feature development, needs deliberation pipeline for internal hooks.

| Stash ID   | Pri    | Kind    | Summary                                   |
|------------|--------|---------|--------------------------------------------|
| `2599179A` | high   | feature | Internal lifecycle hooks                    |
| `C7550B6E` | medium | feature | External system hooks (009-DL queued)       |
| `011-DL`   | medium | delib   | Event traceability enrichment               |

### Shipment E: Stash and Session Resilience

Medium priority infrastructure improvements.

| Stash ID   | Pri    | Kind    | Summary                                   |
|------------|--------|---------|--------------------------------------------|
| `46CC1C9D` | medium | feature | Stash archive JSONL                         |
| `A8F688A7` | medium | task    | Stash hygiene in Stage workflow              |
| `F51BAEC0` | medium | feature | Disaster recovery (needs deliberation)      |

## Recommended Priority Order

1. Shipment A — fastest to ship, removes quality debt
2. Shipment C — fixes real correctness bugs
3. Shipment B — improves daily CLI experience
4. Shipment D — largest scope, needs deliberation for 2599179A
5. Shipment E — medium priority infrastructure

## Queued Deliberations

- `009-DL` (External system hooks) — aligns with Shipment D
- `011-DL` (Event traceability) — aligns with Shipment D

## Next Steps

Operator should select a shipment candidate to stage next. Each candidate
requires the deliberate → impl-plan → plan-review → harvest pipeline before
Ship can claim it.
