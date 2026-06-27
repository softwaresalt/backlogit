---
title: Stage Session — Audit Harvest and Shipment Grouping
description: Session memory for repo-wide audit stash harvest into 5 feature groups with 12 bug tasks
ms.date: 2026-04-22
---

## Session Summary

Comprehensive repo-wide audit identified 12 verified bugs across all major
backlogit subsystems. All 12 were stashed, then grouped into 5 thematic
features and harvested into the backlog as queued tasks.

## Stash Entries Processed

All 12 bug stash entries harvested. 2 feature entries deferred.

| Stash ID | Harvested As | Feature | Summary |
|---|---|---|---|
| BDF63CA2 | 040.001-T | 040-F | Hook event queue fsync |
| 6898DB32 | 040.002-T | 040-F | Hook checkpoint fsync |
| 65941061 | 040.003-T | 040-F | Telemetry harvest fsync |
| ED8C5707 | 040.004-T | 040-F | Hook stale-lock TOCTOU race |
| 1AFACAEF | 041.001-T | 041-F | CLI delete file/index ordering |
| B27D7F1D | 041.002-T | 041-F | Stash harvest atomicity |
| 367F1231 | 041.003-T | 041-F | Archive file/index consistency |
| CCB4B96C | 042.001-T | 042-F | Link type validation on rehydration |
| E2E17135 | 042.002-T | 042-F | Manifest ID extraction lossy scan |
| CCAEF17B | 043.001-T | 043-F | Doctor false orphan positives |
| F75A5B42 | 043.002-T | 043-F | Wire CLI doctor command |
| 9F5ACF94 | 044.001-T | 044-F | Telemetry parser current log format |

## Features Created

| ID | Title | Tasks | Priority |
|---|---|---|---|
| 040-F | Write Durability and Hook Reliability | 4 | high |
| 041-F | File and Index Consistency | 3 | high |
| 042-F | Data Integrity Hardening | 2 | high-medium |
| 043-F | Doctor Feature Completion | 2 | medium |
| 044-F | Telemetry Parser Update | 1 | medium |

## Dependencies Wired

- 043.002-T (Wire CLI doctor command) blocks on 043.001-T (Fix false orphan positives)

## Deferred Stash Entries

| Stash ID | Kind | Reason Deferred |
|---|---|---|
| F51BAEC0 | feature | Needs deliberation and research before planning |
| 21E17BFC | feature | Contingency item — only if prior concurrency fixes prove insufficient |

## Decisions

- Grouped bugs by shared root cause pattern rather than by subsystem, enabling
  single-PR shipments with coherent diffs
- Crash-durability bugs (fsync pattern) bundled with the stale-lock race since
  both are in internal/events/ and both affect hook reliability
- Did not create shipments from Stage (Ship agent owns that transition)
- Skipped deliberation/planning gates for these bugs — they are well-defined
  defects with clear fix directions already documented in stash descriptions

## Recommended Shipment Priority

1. 040-F — Write Durability (crash risk, 3 high-priority tasks)
2. 041-F — File/Index Consistency (data integrity risk, 2 high-priority tasks)
3. 042-F — Data Integrity Hardening (1 high-priority task)
4. 043-F — Doctor Feature Completion (medium, small scope)
5. 044-F — Telemetry Parser Update (medium, standalone)

## Next Steps

- Ship agent can claim any of the 5 features for shipment
- Features 040-F and 041-F are highest priority and could ship in parallel
- Deferred stash entries need deliberation before entering the pipeline
