---
chunk_strategy: h1-h2-h3
schema_version: "1.0"
title: 'Optional size estimation for feature and shipment artifacts plan'
source: docs/exec-plans/2026-07-14-size-estimation-feature-shipment-plan.md
doc_type: plan
description: 'Bounded plan extending optional size estimation to feature and shipment levels with level-specific semantics, provenance metadata, estimate history, and non-persisted derived rollups.'
docline:
    date: 2026-07-14T23:40:00Z
    linked_stash_ids:
        - D7B1B33D
    review_state: passed
---

# Optional Size Estimation for Feature and Shipment Artifacts

## Problem Frame

Size estimation currently exists as an optional `size` field on tasks (enum
XS-XL). Stash `D7B1B33D` asks to extend optional size estimation to the feature
and shipment levels **without** conflating human-authored estimates with
machine-derived rollups. The core risk is provenance: a derived composition
(e.g., a feature size implied by its child tasks) must never be written back as
though a human authored it. The work is backlogit-internal (schema contract plus
Go CLI), and it must stay decoupled from the formal-gate spike and the docline
guard staged alongside it.

## Requirements Trace

| ID | Requirement | Task |
|---|---|---|
| SE1 | Level-specific size semantics for feature vs shipment (schema/contract) | `108.001-T` |
| SE2 | Provenance metadata: `size_source` and `size_ruleset_version` | `108.001-T` |
| SE3 | Estimate-history behavior on size/source change (append-only) | `108.002-T` |
| SE4 | Derived composition/rollups exposed at render time, never persisted as authored | `108.003-T` |
| SE5 | CLI/MCP parity and documentation for feature/shipment sizing | `108.004-T` |

## Task Map (width-isolated, each <=2h)

### `108.001-T` — Schema and contract (schema only)

Define optional `size` semantics distinctly for `feature` and `shipment` in the
schema/registry: what a level's authored size means, the allowed enum, and the
new provenance fields `size_source` (e.g., `human`, `derived`, `ruleset`) and
`size_ruleset_version`. Schema/contract change only — no writer or render logic.

### `108.002-T` — Provenance persistence and estimate history (CLI writer)

On feature/shipment size writes, persist `size_source` and
`size_ruleset_version`, and append an estimate-history event (append-only log)
when size or source changes. Derived values MUST NOT be written with
`size_source: human`. CLI writer/logging concern only.

### `108.003-T` — Derived composition/rollups at render (CLI render)

Expose derived composition/rollups (aggregated child sizing) for feature and
shipment at render time only. Derived rollups are computed on read and are never
persisted into the artifact body as authored estimates. Render/query concern
only.

### `108.004-T` — Documentation and CLI/MCP parity (docs)

Document the feature/shipment sizing contract, provenance fields, and
render-only rollups; verify CLI and MCP surfaces expose sizing identically.
Documentation and parity verification concern only.

## Sequencing

`108.001-T` (schema/contract) lands first. `108.002-T` and `108.003-T` depend on
it and may proceed in parallel. `108.004-T` depends on `108.002-T` and
`108.003-T`.

## Non-Goals

* No coupling to the formal-gate architecture spike or the docline soft-key
  guard staged in the same PR.
* No mandatory sizing; the field stays optional at every level.
* No persisting of derived rollups as human-authored estimates.
