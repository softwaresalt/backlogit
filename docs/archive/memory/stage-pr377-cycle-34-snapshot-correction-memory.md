---
chunk_strategy: h2
description: "PR #377 cycle-34 correction of exact-M snapshot membership, explicit non-shipment fallback, live status-source verification, a real fallback assertion, and green-regression JSON shape; policy 1.22.0; gate remains FAIL"
doc_type: memory
schema_version: "1.0"
source: cycle-34-snapshot-correction-session
title: "PR #377 Cycle 34 Snapshot Correction Memory"
---

# PR #377 Cycle 34 Snapshot Correction Memory

**Date**: 2026-08-26
**Agent**: Prompt Builder
**Branch**: `chore/stage-130-s`
**PR**: #377
**Cycle**: 34
**Worktree**: `.copilot/session-state/337f2436-0fad-4797-be93-b72985d25d56/files/stage-130s-worktree`
**HEAD at session start**: `47925de28b61a39ff3dd3860f8d90a12886a298a`
**Shipment**: `130-S` (queued - not claimed or shipped)
**Scope honored**: prompt, policy, script, plan, backlog record, checkpoint, and memory artifacts
only; no Go source, subagent, push, or merge

## Baseline

Before edits:

```text
wave-scheduler-sim.ps1
WAVE_SIM_OK: 90/90 assertions PASS across 18 scenario(s)

wave-scheduler-sim.ps1 -VerifyAgainstQueue
WAVE_SIM_OK: 104/104 assertions PASS across 18 scenario(s)
```

The passing baseline exposed the contract defect. The documented `parent_id = '147-F'` SQL
returned 43 task children: frozen M's 42 tasks plus archived historical `147.010-T`. The non-SQL
path listed all tasks, the non-shipment path inferred children, status sources were fixture copies,
and `frozen_m_counterpart` compared its expected value with itself.

## Corrections

| ID | Sev | Correction |
|---|---|---|
| I1 | P1 | SQL uses `artifact_type = 'task'` plus exact frozen-M IDs with bound parameters or validated/escaped quoted CLI literals; no `parent_id` |
| I2 | P1 | Non-SQL gets each frozen ID directly exactly once at every status; task listing is forbidden; non-shipment mode requires explicit `frozen_task_ids` or halts |
| I3 | P2 | Live verification parses config statuses and registry status mapping plus `sql_query`/`shipments`, with drift mutations |
| I4 | P2 | Exact manifest-M/fallback equality replaces the tautology; archived `147.010-T` inclusion is a mutation control |
| I5 | P3 | Green-regression parsing requires an object root and array value, covered by three parser controls |

The covering feature `147-F` and archived historical sibling `147.010-T` are explicit forbidden
wave IDs. Neither may enter M, the non-shipment fallback set, or any snapshot.

## Validation

| Gate | Result |
|---|---|
| Simulator | `WAVE_SIM_OK` 93/93 across 18 scenarios |
| Live drift and 13 mutations | `WAVE_SIM_OK` 115/115 across 18 scenarios |
| Exact-M SQL | 42 distinct task IDs, 104 dependency edges, both forbidden IDs absent |
| Direct get | sample PASS; full 42/42 exact-once status/dependency parity with SQL |
| Markdown and docs lint | 0 issues / `valid: true`, 0 violations |
| Integration tests | exit 0 |
| Index sync | 0 parse failures |
| Topology | S=43, M=42, 104 executable edges, 18 waves, acyclic |
| Go source modified | none |

## Gate state

`cycle: 34`, `decision: FAIL`, `pending: independent-review-required`, `push_allowed: no`;
severity P0=0 / P1=2 / P2=2 / P3=1, all remediated in-pass. Policy
`workflow-policies.md` is **1.21.0 -> 1.22.0**.

Independent review is required before push. PR-thread reconciliation, shipment claim, and merge
remain blocked. Operator merge approval has not been requested.
