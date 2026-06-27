---
title: "Stage Session: Shipment Grouping Triage"
description: "Stash and queue review to identify logical shipment groupings"
ms.date: 2026-04-12
---

## Session Summary

Reviewed 10 active stash entries and 2 queued deliberations to identify logical
shipment groupings based on thematic alignment, dependency relationships, and
priority.

## Stash Entries Reviewed

| ID | Priority | Kind | Summary |
|---|---|---|---|
| `042C1812` | high | task | Constrain export_command_map path to .backlogit/ |
| `6D175713` | high | feature | Fix adopt_item atomic ID rewrite + file rename |
| `2599179A` | high | feature | Internal lifecycle hooks |
| `F51BAEC0` | medium | feature | Disaster recovery for agent session termination |
| `46CC1C9D` | medium | feature | Add stash.jsonl archive for harvested entries |
| `A8F688A7` | medium | task | Add stash hygiene/pruning to Stage workflow |
| `3F71C20D` | medium | task | Tighten query-gate + section-write correctness |
| `D6E4B181` | medium | task | Fix stash harvest TOCTOU + status blocking |
| `C7550B6E` | medium | feature | External system hooks (Slack/Teams/webhooks) |
| `53EC8F4C` | medium | task | Fix instructions referencing index.db |

## Queued Deliberations

| ID | Status | Summary |
|---|---|---|
| `009-DL` | queued | External system hooks deliberation |
| `011-DL` | queued | Event traceability and commit tracking |

## Proposed Shipment Groupings

### Shipment A: Correctness and Safety Fixes (HIGH priority)

Items: `042C1812`, `6D175713`, `3F71C20D`, `D6E4B181`, `53EC8F4C`

Rationale: All correctness and safety fixes that harden existing surfaces. Two
high-priority path-safety items anchor the shipment. No cross-dependencies
between them, so they can be implemented in parallel. Small, well-scoped fixes
that ship cleanly as one unit.

### Shipment B: Stash Lifecycle Improvements (MEDIUM priority)

Items: `46CC1C9D`, `A8F688A7`

Rationale: Both items improve stash lifecycle management. Archive captures
history after harvest; hygiene adds staleness pruning. Archive logically
precedes hygiene since pruning can consider archival state. Compact shipment
with clear dependency ordering.

### Shipment C: Hooks and Event System (HIGH priority, large scope)

Items: `2599179A`, `C7550B6E`, plus deliberations `009-DL`, `011-DL`

Rationale: Internal hooks must precede external hooks since internal hook points
are the foundation external hooks consume. Event traceability connects to the
hook event model. Large scope that may warrant sub-shipments but should be
planned together. Both deliberations are already queued.

### Shipment D: Agent Session Disaster Recovery (MEDIUM priority, spike candidate)

Items: `F51BAEC0`

Rationale: Standalone exploratory item that needs investigation (spike) before
planning. Involves harness and backlogit tooling cross-cutting concerns. Keep
separate from near-term correctness work.

## Recommended Priority Order

A then B then C then D.

A first because highest urgency with smallest blast radius and unblocks trust
in existing tools. B second because stash workflow improvements benefit ongoing
Stage operations. C third because large scope but deliberations are already
queued so planning can start while A and B ship. D last because exploratory and
needs spike before any planning.

## Next Steps

Operator selects a shipment grouping to proceed with. For the selected group,
the next action is to invoke the deliberate skill (or skip to impl-plan if
entries are already well-formed) and then proceed through the standard Stage
pipeline: impl-plan, plan-review, harvest.

## Deferred Entries

All 10 stash entries remain active in stash.jsonl awaiting operator selection
of which shipment grouping to advance first.
