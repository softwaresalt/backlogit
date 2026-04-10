---
title: "008-S Workspace Governance and Archival Policies: Post-Merge Closure"
description: "Operational closure record for shipment 008-S, PR #19, merge commit a058efe"
ms.date: 2026-04-10
ms.topic: reference
---

## Closure Summary

| Field | Value |
|---|---|
| Feature | 025-F: Workspace Governance and Archival Policies |
| Shipment | 008-S |
| PR | [#19](https://github.com/softwaresalt/backlogit/pull/19) |
| Merge commit | `a058efe` |
| Fix commits | `f942b2e` (round 1), `9faa03f` (round 2), `5d8891e` (round 3), `9ed13fb` (round 4) |
| Mode | post-merge |
| Readiness | **READY** |
| Owner | dewilliams |
| Validation window | 48 hours post-merge |

## Change Summary

Eight tasks shipped across `internal/core`, `internal/mcp`, `tests/contract`, and
`tests/integration`. All implemented test-first against failing harnesses, verified
via full CI on both Go 1.23 and 1.24.

| Task | Title | Package(s) |
|---|---|---|
| 025.013-T | Hierarchy enforcement for level-2+ artifacts | `internal/core` |
| 025.014-T | Archive lifecycle (ArchiveItem / UnarchiveItem) | `internal/core` |
| 025.015-T | Stash harvest safety: single-lock + best-effort cleanup | `internal/core` |
| 025.016-T | Doctor: orphan and duplicate-ID diagnostics | `internal/core` |
| 025.017-T | `backlogit_doctor` MCP tool | `internal/mcp` |
| 025.018-T | `VerifyPostShipConsistency` post-ship check | `internal/core` |
| 025.011-T | ShipShipment lifecycle with archive integration | `internal/core` |
| 025.012-T | Integration harness for full lifecycle | `tests/integration`, `tests/contract` |

**Net change:** 64 files, +2779 / -158 lines.

## CI Status

| Check | Go 1.23 | Go 1.24 |
|---|---|---|
| `go test ./...` | ✅ pass | ✅ pass |
| `golangci-lint run` | ✅ pass | ✅ pass |
| `gofmt -l .` | ✅ pass | ✅ pass |
| `go vet ./...` | ✅ pass | ✅ pass |

All four CI rounds completed green. 22 Copilot review comments resolved across 4
review rounds (commits `f942b2e`, `9faa03f`, `5d8891e`, `9ed13fb`).

## Healthy Signals

* `go test ./...` passes on both Go 1.23 and 1.24 with no skipped tests.
* `backlogit_doctor` MCP tool responds with `{"findings":[],"checked_at":"..."}` on
  a clean workspace.
* `ShipShipment` archives all shipment items and returns
  `VerifyPostShipConsistency` error-free (no stale IDs in queue).
* `backlogit add` with a level-2 artifact type (`task`, `subtask`) requires
  `parent_id`; creates successfully when `parent_id` is supplied.
* `HarvestStashEntry` runs to completion without leaving orphaned artifacts when
  invoked concurrently.

## Failure Signals

* **Orphaned artifacts:** `backlogit_doctor` returns findings with
  `type: "orphaned_artifact"` (indicates level-2+ items were created without a
  `parent_id` and no returned-to-backlog event exists).
* **Duplicate IDs:** `backlogit_doctor` returns findings with
  `type: "duplicate_id"` (indicates the same artifact ID appears in multiple
  directories, regression in file routing or archive logic).
* **Stale post-ship files:** `ShipShipment` returns a non-nil error containing
  `"stale artifact IDs"`: archive operation did not complete cleanly.
* **Hierarchy rejection:** `backlogit add --type task` with no `--parent`
  returns an error; if it does NOT return an error, hierarchy enforcement
  regressed.
* **CI failures:** Any red check on `go test ./...` or `golangci-lint run`
  indicates a regression.

## Monitoring Plan

| Surface | Check | Frequency |
|---|---|---|
| Workspace integrity | Run `backlogit_doctor` (orphans + duplicates) | After each shipment |
| Post-ship verification | `VerifyPostShipConsistency` runs automatically inside `ShipShipment` | Every ship operation |
| Hierarchy enforcement | Review creation errors for unexpected `parent_id` rejections | On agent task creation |
| Archive audit | Verify archive/ dir grows after each `backlogit shipment ship` | After each ship |

## Rollback Plan

**Rollback trigger:** `backlogit_doctor` returns unexpected findings on a clean
workspace, or `ShipShipment` begins erroring on valid inputs in production.

**Rollback steps:**
1. Revert to the prior release tag via `git revert a058efe` or by creating a
   hotfix branch from `bb71b3d` (the pre-008-S main HEAD).
2. Rebuild and redeploy the binary.
3. The stash and artifact Markdown files are not altered by this change.
   No data migration reversal is required.
4. File an incident item in the backlog with the specific error + reproduction
   steps before reverting.

**Blast radius:** Workspace governance checks are additive. No existing artifacts,
CLI commands, or MCP tools were removed. The only breaking surface is
`backlogit add --type task` without `parent_id`, which now rejects instead of
silently creating an orphan.

## Validation Window

48 hours post-merge. Evaluate:

* No unexpected `parent_id` validation rejections in agent workflows.
* No `VerifyPostShipConsistency` errors during routine ship operations.
* `backlogit_doctor` returns empty findings on the current workspace.

## Follow-up Items

* Stash 1393A037 (staleness detection) still open (not part of this shipment).
* `Doctor` currently scans all registry-routed directories; a future improvement
  could parallelize the walk for large workspaces.
* `VerifyPostShipConsistency` is called inside `ShipShipment` but not yet
  exposed as a standalone CLI command. Add `backlogit shipment verify` in a
  future iteration if operators need manual invocation.

## Learnings

Captured in:
* `docs/compound/workflow-issues/pr-review-comment-reply-protocol-2026-04-10.md`:
  PR review comment reply protocol (hard gate in `fix-ci` Step 4c)

Key pattern: Registry-awareness matters for archive path exclusion. Hardcoded
`"archive"` directory paths create a subtle correctness gap when users configure
custom registry paths. Always derive excluded directories from registry rules
rather than assuming conventional layout.
