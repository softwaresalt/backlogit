---
title: Stage session — telemetry stash triage and shipment assembly
description: Staged 5 telemetry stash entries into 2 features, 8 tasks, and 2 shipments
ms.date: 05/08/2026
ms.topic: reference
---

## Session Summary

Triaged all 6 active stash entries. Identified 5 actionable telemetry items
and 1 deferred contingency item. Grouped the 5 items into 2 thematic
shipments, wrote implementation plans, gated through plan-review, and
harvested into backlogit work items.

## Stash Disposition

| Stash ID | Disposition | Target |
|---|---|---|
| E39D0A34 (high, bug) | Harvested | 053-F / 052-S |
| 5EC2B37F (high, feature) | Harvested | 053-F / 052-S |
| B68AED87 (medium, feature) | Harvested | 054-F / 053-S |
| 5F0AAB28 (medium, feature) | Harvested | 054-F / 053-S |
| 6646ACA1 (low, feature) | Harvested | 054-F / 053-S |
| 21E17BFC (low, feature) | Deferred | Remains in stash — contingency, trigger condition not met |

## Items Created

### Feature 053-F: Telemetry Accuracy & Call-Rate Metrics (high)

| Task | Title | Dependencies |
|---|---|---|
| 053.004-T | Ghost session predicate and trend filtering | — |
| 053.005-T | Ghost session visual indicator in list output | 053.004-T |
| 053.006-T | Add call-rate columns to TrendGroup and formatters | 053.004-T |

### Feature 054-F: Model-Aware Telemetry (medium)

| Task | Title | Dependencies |
|---|---|---|
| 054.001-T | Model class and reasoning level derivation functions | — |
| 054.002-T | Populate model class and reasoning level during harvest | 054.001-T |
| 054.003-T | Surface PRIMARY_MODEL in list and report session views | 054.001-T |
| 054.004-T | Add --by model and --by class report dimensions | 054.001-T, 054.002-T |
| 054.005-T | Add --by class grouping to trend report | 054.001-T, 054.002-T |

## Shipments Assembled

| Shipment | Title | Priority | Items |
|---|---|---|---|
| 052-S | Telemetry Accuracy & Call-Rate Metrics | High | 053-F + 5 tasks |
| 053-S | Model-Aware Telemetry | Medium | 054-F + 8 tasks |

## Plans Written

- `docs/exec-plans/2026-05-08-telemetry-accuracy-call-rate-plan.md` — PASS
- `docs/exec-plans/2026-05-08-model-aware-telemetry-plan.md` — PASS (P1 resolved, P2 advisory)

## Decisions

- Ghost session filtering at report time (not harvest time) preserves auditability
- Shipment A ships first — accuracy fix ensures new call-rate columns launch with correct data
- Model class derived from model name string, not external registry
- Reasoning level limited to OpenAI o-series models only

## Next Steps

- Ship agent claims 052-S first (high priority), then 053-S
- 21E17BFC (singleton MCP server) remains in stash at low priority
