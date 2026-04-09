---
title: "019-F Data Quality & Tool Efficiency — Post-Merge Closure"
description: "Operational closure record for shipment 004-S, PR #13, merge commit e4b3289"
ms.date: 2026-04-08
ms.topic: reference
---

## Closure Summary

| Field | Value |
|---|---|
| Feature | 019-F — Data Quality & Tool Efficiency |
| Shipment | 004-S |
| PR | [#13](https://github.com/softwaresalt/backlogit/pull/13) |
| Merge commit | `e4b3289` |
| Fix commit | `bbb9691` (8 Copilot review comments) |
| Mode | post-merge |
| Readiness | **READY** |
| Owner | dewilliams |
| Validation window | 48 hours post-merge |

## Change Summary

Seven independent tasks shipped across four packages. All were implemented
test-first against failing harnesses and verified via full CI.

| Task | Title | Package |
|---|---|---|
| 019.001-T | Stable pagination — `ORDER BY id ASC` before `LIMIT/OFFSET` | `internal/db` |
| 019.002-T | Orphan filter — exclude items whose parent is done/archived | `internal/core` |
| 019.003-T | Compact queue view — `CompactItem` with field projection | `internal/models`, `internal/core` |
| 019.004-T | Ghost entry fix — rehydration skips JSONL-only items (no `.md`) | `internal/db` |
| 019.005-T | Stash locking — advisory lock in `HarvestStashEntry` and `LinkDeliberationToStashEntry` | `internal/core` |
| 019.006-T | Stale lock TTL — 60 s mtime-based auto-cleanup in `lockStashFile` | `internal/core` |
| 019.007-T | Duplicate detection — `FindDuplicateTitles` SQL query helper | `internal/db` |

**Net change**: 42 files, +1907 / −127 lines across implementation and harness tests.

Three review follow-up tasks (001-T, 002-T, 003-T) were resolved in the same
session as part of the Copilot comment fix commit (`bbb9691`) and have been
marked done.

## CI Gate Results

| Run | Go 1.23 | Go 1.24 |
|---|---|---|
| `bbb9691` (final commit) | ✅ Pass | ✅ Pass |
| `e4b3289` (merge commit) | N/A — PR checks applied to bbb9691 | N/A |

All 14 packages (`go test ./...`) passed. Zero `golangci-lint` findings. Zero
`go vet` findings. `gofmt -l .` clean.

## Healthy Signals

* `backlogit_get_queue` returns items in stable, repeatable order when
  `limit`/`offset` are used
* `backlogit_get_queue` with orphan filtering no longer returns items whose
  parent feature is archived or done
* `backlogit_list_items` compact mode returns only `id`, `title`, `status`,
  `type`, `parent_id` — context cost reduced from ~1.2 MB to <10 KB for a
  full queue
* `backlogit_sync_index` (rehydrate) runs atomically: any crash mid-walk
  rolls back to the previous index state, not an empty index
* Stash file (`stash.jsonl`) no longer corrupts under concurrent harvests —
  advisory lock with automatic stale-lock recovery prevents lost writes
* `FindDuplicateTitles` correctly identifies items with matching titles across
  different IDs

## Failure Signals

Investigate immediately if any of these appear in logs or agent output:

* `ERROR rehydrate: commit failed` — atomic transaction rolled back; index
  reverted to pre-rehydration state; investigate disk I/O or concurrent write
* `WARN stale stash lock detected` followed by a second lock failure — two
  processes holding overlapping locks; check for concurrent agent sessions
  writing to stash simultaneously
* `CompactItem` fields are all empty despite non-empty query results —
  struct tag mismatch or nil-safe field scan regression
* `backlogit_get_queue` returns items with `parent_id` referencing archived
  features — orphan filter `LEFT JOIN` or exclusion `WHERE` clause regressed
* Duplicate title pairs appear after a migration or manual edit — expected
  behavior; `FindDuplicateTitles` surfaces them for manual triage

## Monitoring Plan

| Surface | What to watch | How |
|---|---|---|
| Rehydration | `rehydrate:` log prefix; transaction commit/rollback ratio | `grep "rehydrate" telemetry.jsonl` |
| Stash locking | `stale stash lock` warnings | `grep "stale stash lock" logs/` |
| Pagination | Verify repeated requests with `offset` return different items | Manual spot check with `backlogit_query_sql` |
| Ghost entries | `SELECT COUNT(*) FROM items` should equal the `.md` file count in `.backlogit/` | `backlogit sync` + SQL spot check |
| Compact output | Token cost of `backlogit_get_queue` response | Agent context monitoring |

## Rollback Triggers

Roll back to the previous `main` commit (`7b8cc8b`) if:

* Rehydration consistently produces an empty index after restart
* Stash corruption is observed (missing entries, duplicate entries, parse
  errors) despite the new locking
* `backlogit_get_queue` returns zero results for an obviously non-empty queue

Rollback procedure:

```bash
git revert e4b3289 --no-edit
git push origin HEAD:chore/rollback-019-f
gh pr create --title "revert: 019-F data quality rollback" --base main
```

## Unresolved Items

None blocking. The three follow-up tasks from the review (001-T, 002-T, 003-T)
have been resolved and closed — all three root causes were addressed in fix
commit `bbb9691`.

Outstanding stash entries `64CFF524` (MCP tooling coverage) and `F51BAEC0`
(disaster recovery research) are queued for a future grooming session.

## Post-Merge State

* `004-S`: shipped and archived
* `019-F` and all 7 tasks: archived (merge commit `e4b3289` stamped)
* `001-T`, `002-T`, `003-T`: done
* Backlog index: synced; follow-up PR #14 carries archive/state changes

## Readiness: READY

All gates passed. No blocking residuals. Validation window closes 2026-04-10.
