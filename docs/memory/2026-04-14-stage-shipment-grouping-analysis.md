---
title: Stage Shipment Grouping Analysis
description: Triage analysis of 7 active stash entries grouped into 3 shipment candidates with 2 deferred items
ms.date: 2026-04-14
---

## Context

All queue items are done/archived. Clean slate for new shipments. 7 active stash entries analyzed for logical grouping.

## Stash Hygiene

No entries at or above 30-day age threshold. One entry (F51BAEC0) has no `age_days` field (predates `created_at` support), surfaced to operator as unknown age. No removals needed.

## Shipment Candidates

### Shipment A: Hooks System (recommended first)

| Stash ID | Priority | Kind | Summary | Deliberation |
|---|---|---|---|---|
| 2599179A | high | feature | Internal lifecycle hooks for validation, cascade, and enforcement | 007-DL (archived, fully resolved) |
| C7550B6E | medium | feature | External system hooks with prioritized notification push | 032-DL (queued, fully resolved) |

Rationale: 032-DL explicitly depends on 007-DL for the hook firing mechanism. Internal hooks provide the foundation (pre/post callbacks on CreateArtifact, MoveArtifactStatus, ArchiveItem, ShipShipment, AdoptItem). External hooks build on that with webhook dispatch. Both deliberations have chosen directions and detailed notes. This is the highest-priority grouping (contains the only high-priority stash entry) and both items are planning-ready.

Dependency order: 2599179A (internal hooks) must ship before C7550B6E (external hooks).

### Shipment B: Stash Lifecycle

| Stash ID | Priority | Kind | Summary | Deliberation |
|---|---|---|---|---|
| 46CC1C9D | medium | feature | Stash archive for removed entries in .backlogit/archive/ | None |
| A8F688A7 | medium | task | Stash hygiene in Stage workflow (agent protocol + compound doc refresh) | None |

Rationale: Complementary stash lifecycle improvements. Archive preserves durable history of removed stash entries (currently they disappear entirely). Hygiene formalizes the pruning step. Small, self-contained scope. Neither has a deliberation artifact yet, so they need deliberation before planning.

Note: A8F688A7 may be partially addressed already (Stage agent file now includes Step 0 stash hygiene), but the stash entry mentions compound doc refresh and protocol formalization that may still be outstanding.

### Shipment C: Data Integrity

| Stash ID | Priority | Kind | Summary | Deliberation |
|---|---|---|---|---|
| C00AA592 | medium | task | AdoptItem cross-artifact reference rewrite (stale parent_id/dependencies after rename) | None |

Rationale: Standalone bug fix. When AdoptItem rewrites an artifact's ID and renames its file, other artifacts with frontmatter references to the old ID are not updated. Stale edges reappear on rehydration. Could optionally fold into Shipment A since `post_adopt` is an identified hook point in 007-DL, but the fix itself is independent of the hooks system. Needs deliberation.

## Deferred Items

| Stash ID | Priority | Kind | Summary | Reason |
|---|---|---|---|---|
| F51BAEC0 | medium | feature | Disaster recovery for agent sessions | Needs spike or deliberation first. No age_days (unknown age, predates created_at). Research and analysis required before staging. |
| 21E17BFC | medium | feature | Singleton MCP server with multiplexed transport | Explicitly marked as contingency. Only pursue if concurrent-access fixes (busy_timeout, write-retry, BEGIN IMMEDIATE, batched rehydration) prove insufficient. Keep in stash. |

## Recommended Priority

A → B → C

Shipment A has the only high-priority entry and both deliberations are fully resolved (ready for impl-plan). Shipment B is small and complementary but needs deliberation first. Shipment C is a focused fix that also needs deliberation.

## Next Steps

For Shipment A: invoke impl-plan on 007-DL and 032-DL (both have chosen directions), then plan-review, then harvest.

For Shipments B and C: invoke deliberate skill before planning.

For deferred items: F51BAEC0 should be routed to spike when capacity allows. 21E17BFC stays in stash as contingency.
