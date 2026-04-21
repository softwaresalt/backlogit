---
title: "PR #47 Copilot Comments Resolved — 036-S Ready for Merge"
description: "7 Copilot review threads on PR #47 resolved; CI 3/3 passing; awaiting user merge approval for 036-S"
ms.date: 2026-04-20
---

## Session Summary

Resolved all 7 Copilot review comment threads on PR #47
(`feat/036-s-source-artifact-archival`). Commit `cb4b34c` addresses the 5 real
issues; 2 false positives replied to and resolved without code change.

## Fixes Applied (commit cb4b34c)

### 1. `ship.agent.md` — Invalid `reason` param in `backlogit_stash_remove`

Removed "with the stash ID and reason `superseded by {shipment_id}`" from the
`backlogit_stash_remove` call in Step 2 of Post-merge closure. Tool only accepts
`stash_id`. The `superseded by {shipment_id}` linkage is now recorded in the
closure artifact's `Source artifact cleanup` section instead.

### 2. `ship.agent.md` — Broadcast/log bullets inside per-feature loop

Moved the broadcast and logging bullets outside the "For each feature" loop.
Added "After processing all features in scope:" heading. Also updated the
reference from the generic "follow-up section" to the specific `Source artifact
cleanup` section to match the operational-closure skill.

### 3. `docs/compound/workflow-issues/source-artifact-archival-pattern-2026-04-20.md`

Same invalid `reason` parameter fix in the compound learning doc (line 79).
Updated item 4 to clarify that the shipment linkage is recorded in the closure
artifact, not passed as a `backlogit_stash_remove` parameter.

### 4. `.backlogit/archive/036-DL.md` — Missing `archived_from` frontmatter

Added `archived_from: .backlogit/deliberations/036-DL.md` to the frontmatter.
This is required by `UnarchiveItem`; `backlogit_archive_item` always stamps it,
but this file was archived without going through the tool surface. Workspace
repair exception applied; `backlogit_sync_index` called immediately after.

### 5. `.backlogit/queue/033.013-T.md` — Missing `parent_id` (orphan)

Added `parent_id: 033-F` directly (workspace repair). `backlogit_adopt_item`
failed because 033-F is archived and cannot be located by the MCP tool's lookup.
`backlogit_sync_index` called after to keep DB consistent.

## False Positives (no code change)

- Thread 4: `docs/memory/20260420-150100-stage-036-s-memory.md` — `||` claim;
  verified zero matches with `Select-String '\|\|'`
- Thread 5: `docs/memory/20260420-143220-ship-035-s-final-memory.md` — same claim;
  same verification, zero matches

## Thread Resolution Status

| Thread ID | File | Status |
|---|---|---|
| PRRT_kwDORzozKM58WFWZ | ship.agent.md (stash reason param) | ✅ Resolved |
| PRRT_kwDORzozKM58WFWw | source-artifact-archival compound doc | ✅ Resolved |
| PRRT_kwDORzozKM58WFW4 | .backlogit/archive/036-DL.md | ✅ Resolved |
| PRRT_kwDORzozKM58WFXB | stage memory false positive | ✅ Resolved |
| PRRT_kwDORzozKM58WFXJ | ship memory false positive | ✅ Resolved |
| PRRT_kwDORzozKM58WFXU | ship.agent.md (broadcast outside loop) | ✅ Resolved |
| PRRT_kwDORzozKM58WFXd | 033.013-T orphan | ✅ Resolved |

## Current State

* **Branch**: `feat/036-s-source-artifact-archival` at `cb4b34c`
* **PR**: [#47](https://github.com/softwaresalt/backlogit/pull/47) — all
  threads resolved, CI 3/3 passing
* **Shipment**: 036-S — active, awaiting user merge approval
* **Post-merge closure pending**: After merge, must execute:
  - `backlogit_stash_remove(stash_id="B155D9DA")` (source_stash_id for 034-F)
  - `backlogit_ship_shipment(id="036-S")`
  - Invoke `operational-closure` skill
  - Write `docs/closure/` artifact

## Next Steps

1. **User approves merge of PR #47** → merge to main
2. Post-merge closure for 036-S:
   - `backlogit_stash_remove(stash_id="B155D9DA")`
   - `backlogit_ship_shipment(id="036-S")`
   - Operational closure + docs/closure artifact
