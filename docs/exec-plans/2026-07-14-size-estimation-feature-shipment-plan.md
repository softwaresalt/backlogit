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
    gate: PASS
    review_provenance: "plan-review skill RUN 2026-07-15 by Stage against final plan bytes; single-model multi-persona (cross-model unavailable per skill fallback); P0=0 P1=0 P2=0"
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
| SE2 | Provenance metadata fields `size_source` and `size_ruleset_version` defined in schema; persisted through the shared `core.SetArtifactSize` seam | `108.001-T` (define) / `108.002-T` (persist) |
| SE3 | Estimate-history behavior on size/source change (append-only), emitted from the same core seam | `108.002-T` |
| SE4 | Derived composition/rollups exposed at render time with CLI/MCP parity, never persisted as authored | `108.003-T` |
| SE5 | CLI/MCP parity documentation and verification for feature/shipment sizing | `108.004-T` |

## Task Map (width-isolated, each <=2h)

### `108.001-T` — Schema and contract (schema only)

Define optional `size` semantics distinctly for `feature` and `shipment` in the
schema/registry: what a level's authored size means, the allowed enum, and the
new provenance fields `size_source` (e.g., `human`, `derived`, `ruleset`) and
`size_ruleset_version`. Schema/contract change only — no writer or render logic.

**Backward compatibility:** the change is purely additive. Existing task-level
`size` values without provenance remain valid; `size_source` and
`size_ruleset_version` are optional, and absent provenance is read as
unknown/legacy and never rewritten as `human`. No migration of existing
artifacts is performed.

### `108.002-T` — Provenance persistence and estimate history (core seam, both adapters)

Extend the canonical `core.SetArtifactSize` seam (`internal/core/artifact_size.go`)
— the single body-preserving size-mutation helper — rather than duplicating logic
in the CLI. On feature/shipment size writes it persists `size_source` and
`size_ruleset_version` and appends an estimate-history event (append-only log)
when size or source changes. Because both the CLI (`internal/cli/update.go`
`update --size`) and the MCP adapter (`internal/mcp/tools.go` `update_item` size
argument) already route through this one seam, provenance and history are covered
identically for CLI and MCP with no adapter-specific duplication. Derived values
MUST NOT be written with `size_source: human`. Core mutation-seam concern only.

### `108.003-T` — Derived composition/rollups at render (CLI + MCP parity)

Expose derived composition/rollups (aggregated child sizing) for feature and
shipment at render time via a shared render/query helper consumed by BOTH the CLI
(`get`/`queue` render) and the MCP surfaces (`get_item`, `get_shipment`), so
composition is presented with CLI/MCP parity rather than CLI-only. Derived
rollups are computed on read and are never persisted into the artifact body as
authored estimates. Render/query concern only.

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

## Constitution Check

- **I (Safety-First Go):** All work is Go; the core seam and render helper keep
  wrapped errors and must pass `go vet ./...` and `golangci-lint run` with zero
  warnings before commit. No `unsafe` usage.
- **II (Test-First, NON-NEGOTIABLE):** Every task is labelled `test-first`; a
  failing test precedes implementation for the schema contract, the core
  persistence/history seam, and the render helper.
- **III/IV (Workspace isolation / CLI containment):** Size writes go through the
  existing body-preserving `core.SetArtifactSize` path (`atomicfile` within the
  workspace root); no path traversal, no writes outside the cwd tree.
- **V (Structured Observability):** The append-only estimate-history event makes
  every size/source change traceable.
- **VI (Single Responsibility):** No new dependencies; the plan reuses the
  existing `core.SetArtifactSize` seam and existing schema/registry rather than
  adding libraries.
- **VII (Destructive Approval, NON-NEGOTIABLE):** No destructive operations —
  writes are body-preserving and history is append-only; the schema change is
  additive and non-migrating.
- **VIII (Safety Modes):** Freeze-scope posture — work is confined to the size
  subsystem (schema, core seam, render, docs) and explicitly decoupled from the
  formal-gate spike and docline guard.
- **IX (Git-Friendly Persistence):** Provenance and history serialize to
  human-readable Markdown/YAML with atomic writes.
- **X (Context Efficiency):** Rollups are computed on read and exposed through
  existing query/render surfaces; no bulk duplication.
- **XI (Merge Commit Preservation):** Not applicable at plan stage; Stage does
  not merge or ship.

Task Granularity: each of the four tasks is one concern (schema / core seam /
render / docs), targets well under three files and five functions, and covers
both CLI and MCP through a single shared seam or helper rather than duplicated
adapter logic — preserving both the 2-Hour Rule and width isolation. No
constitutional violation, waiver, or exception is planned.

## Plan Review

### Gate Decision: PASS

**Formal plan-review provenance:** RUN on 2026-07-15 by the Stage agent
following the `.github/skills/plan-review/SKILL.md` gate against the exact final
plan bytes above (Problem Frame through Constitution Check). Cross-model reviewer
invocation was unavailable in this environment; per the skill's explicit fallback
("If cross-model invocation is not available, run all personas with the caller's
model. Multi-model is preferred but not blocking."), all reviewer personas were
executed with the caller's model. This is a single-model multi-persona review,
disclosed as such — not a manufactured or waiver-based pass.

**Reviewer personas executed:**

| Persona | Trigger | Result |
|---|---|---|
| Constitution Reviewer | always-on | No violation. II (test-first labels), VI (no new deps; reuses `core.SetArtifactSize`), VII (additive schema, body-preserving/append-only writes) satisfied. Surfaced one advisory on schema backward-compat (now addressed in `108.001-T`). |
| Go Reviewer | always-on | Reuse of the single body-preserving `core.SetArtifactSize` seam avoids duplicated CLI logic; render helper is read-only. Advisory (P3): estimate-history storage location/format is left to implementation, which is appropriate at planning granularity. No P0/P1. |
| Scope Boundary Auditor | always-on | Width isolation holds — schema / core seam / render / docs are one concern each; covering both adapters via a shared seam or helper does not break isolation. Cleanly decoupled from formal-gate (105-F/106-F) and docline (107-F). No P0/P1. |
| Learnings Researcher | always-on | Consistent with the prior task-level size work (`071.007-T` established `core.SetArtifactSize` as the single body-preserving mutation seam). The plan extends that seam rather than contradicting it. No P0/P1. |
| Architecture Strategist | always-on | Cohesive schema→seam→render→docs chain; reusing one mutation seam directly resolves the "duplicate CLI logic" risk and yields inherent CLI/MCP parity. Clean dependency order. No P0/P1. |
| Agent-Native Parity Reviewer | triggered (plan exposes MCP `update_item`/`get_item`/`get_shipment` and CLI parity-sensitive sizing workflows) | Parity is designed in: `108.002-T` persists provenance through the shared core seam both adapters already call, `108.003-T` exposes rollups via a shared render helper consumed by CLI and MCP, and `108.004-T` verifies parity. No CLI-only gap. No P0/P1. |
| Security Lens Reviewer | not triggered (no auth/authz, secrets, external integrations, or cross-trust-boundary data; append-only history stays within the workspace) | — |

**Findings disposition:** P0 = 0, P1 = 0, P2 = 0, P3 = advisory only (estimate-history storage format and exact helper signatures are left to implementation; appropriate at planning granularity). The one Constitution Reviewer advisory (schema backward-compat) was closed by adding the explicit additive/non-migrating backward-compatibility note to `108.001-T`. No finding blocks harvest.

**Plan hardening:** Evaluated and NOT required. Although the plan touches the
schema and the core mutation seam, every change is additive and optional
(new provenance fields, feature/shipment size), body-preserving and append-only,
with no data migration, no destructive operation, and no CLI-distribution change.
These are not the elevated-blast-radius signals that gate on `plan-harden`.

**Disposition:** Gate PASS. Shipment `096-S` (feature `108-F` plus tasks
`108.001-T`–`108.004-T`) matches this task map and is queued for Ship to claim.
This plan remains decoupled from the formal-gate governance work and the docline
soft-key guard staged in the same PR.
