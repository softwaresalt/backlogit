---
title: "Groom Group G: Data Quality & Tool Efficiency"
description: "Session memory capturing Group G grooming from stash triage through harvest"
ms.date: 2026-04-08
---

## Outcome

Completed the full Groomer pipeline for Group G (Data Quality & Tool Efficiency):
deliberate → impl-plan → plan-review → harvest.

Created feature 019-F with 7 independent tasks ready for Shipper workflow.

## Pipeline Steps Completed

1. **Housekeeping**: Retired 7 stale stash entries already shipped. Reconciled
   015.009-T and 015.011-T to done. Shipped 002-S with SHA 311c82f.
2. **Stash triage**: Reduced 22 entries to 15 active. Grouped into 5 candidate
   sets (C, G, H, I, J).
3. **Group selection**: Group G (Data Quality) selected for highest operational
   urgency (2 high-priority items, experienced during grooming).
4. **Deliberation**: Created 004-DL linking stash 64CFF524. Harvested stash
   entry.
5. **Implementation plan**: Wrote 7-unit plan to
   `docs/exec-plans/2026-04-08-data-quality-tool-efficiency-plan.md`.
6. **Plan review**: 4-persona gate passed with advisories (2 P1 findings
   addressed via plan amendments, 3 P2 noted for implementation).
7. **Harvest**: Created 019-F + 7 tasks (019.001-T through 019.007-T).

## Feature 019-F Task Summary

| Task ID | Title | Labels |
|---|---|---|
| 019.001-T | Pagination for list_items | pagination, mcp-tools |
| 019.002-T | Pagination for fetch_stash | pagination, mcp-tools |
| 019.003-T | Compact projection mode | projection, mcp-tools |
| 019.004-T | Fix rehydration ghost entries | bug-fix, rehydration |
| 019.005-T | Orphan detection filter | orphan-detection |
| 019.006-T | Duplicate detection tool | duplicate-detection |
| 019.007-T | Stash harvest advisory file locking | concurrency, hardening |

All tasks are independent with no inter-dependencies. Recommended execution
order: 019.004-T (bug fix) → 019.003-T (highest impact) → 019.001-T + 019.002-T
(pagination) → 019.005-T → 019.006-T → 019.007-T.

## Key Decisions

- limit/offset pagination, not cursors (D1)
- Clear items/item_deps at rehydration, NOT item_links (D2, D3)
- Sidecar .lock file, not OS file locking (D4)
- Compact as boolean, not field selection (D5)
- Orphan filter on get_queue, not separate tool (D6)
- Duplicate detection as new tool (D7)

## Review Findings Applied

- F-1 (P1): CompactQueueView/CompactQueueGroup needed for get_queue compact mode
- F-3 (P1): unlockStashFile must handle Windows remove failure gracefully

## Remaining Stash Entries (10 active)

Group H (Stash CLI): 174A4EB9, 078E58F2, 8699071E
Group I (Agent Operations): C50CB316, 2CDA43BF, 60EF697D, 834CCDB7
Group J (Lifecycle & Recovery): BA3DB37B, 93A77D46, F51BAEC0

Group C (Spike Type, F014) needs re-grooming for legacy ID format.

## Next Steps

1. Shipper assembles shipment for 019-F (all 7 tasks)
2. Groomer selects next candidate group (H, I, or J) for triage
3. F014 (Group C) deferred until ID re-grooming is addressed
