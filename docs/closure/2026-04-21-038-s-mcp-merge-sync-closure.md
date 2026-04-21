---
title: "Closure: 038-S MCP Merge Sync — Incremental Cache Refresh"
description: "Post-merge operational closure for shipment 038-S — backlogit_merge_sync MCP tool for incremental SQLite cache refresh from .backlogit/ file diffs"
ms.date: 2026-04-21
---

## Context

| Field | Value |
|---|---|
| Shipment | 038-S |
| Feature | 037-F: MCP Merge Sync — Incremental Cache Refresh |
| Branch | `feat/038-mcp-merge-sync` |
| PR | [#52](https://github.com/softwaresalt/backlogit/pull/52) |
| Merge SHA | `f84e3812dc9058ea313f103575939d9656208cbb` |
| CI status | 3/3 green (test 1.23, test 1.24, CLI Reference Drift) |
| Copilot review | 10 comments — all fixed in commit `0697f4f`, all replied |
| Readiness status | **READY** |

## Summary of Change

`backlogit_merge_sync` is a new MCP tool that provides incremental SQLite cache
refresh driven by file-manifest diffing. Where `backlogit_sync_index` performs a
full DELETE-all + WalkDir + batch-insert cycle, `backlogit_merge_sync` computes
a diff against a snapshot of `.backlogit/` file paths and modification times,
upserts only changed artifacts, and deletes only removed artifacts. A fallback
path triggers full rehydration when the diff is too large or the manifest is
stale.

Key surfaces added:

* `internal/db/manifest.go` — `BuildManifest`, `ComputeDiff`, `ShouldFallback`,
  `RehydrateWithManifest`
* `internal/db/merge_sync.go` — `MergeSync` orchestrator with upsert/delete loop
  mirroring full rehydration (deps, links, level/hierarchy_path)
* `internal/mcp/tools.go` — `handleMergeSync` handler with CAS version scheme
* `internal/mcp/server.go` — `manifestVersion uint64` field; manifest seeded in
  `ensureWorkspace` via `BuildManifest`

## Invariants to Preserve

* `backlogit_sync_index` full-rehydration behavior is unchanged; `merge_sync` is
  additive and does not modify the rehydration path.
* Manifest state is never visible to callers — it is internal to the `Server`
  struct, protected by `manifestMu`.
* A failed `MergeSync` leaves the manifest in a state that will retry only the
  failed files on the next call (failed paths have their manifest entry restored
  to the prior snapshot value).
* Contract: `merge_sync` response always includes `upserted`, `deleted`,
  `failed_paths`, and `fallback_triggered`; nil slices normalized to `[]`.

## Pre-Deploy Audits

* CLI Reference Drift CI check validates that CLI docs match the binary — passed.
* No migrations, feature flags, or schema changes were required.
* The SQLite schema is unchanged; `merge_sync` uses existing tables.
* No external service integrations affected.

## Deployment Path

Merge-only. `backlogit_merge_sync` ships as part of the binary. No deployment,
canary, or rollout gate is required. Agents using the MCP server will see the
new tool in the tool list on their next connection.

## Post-Deploy Checks

* Run `backlogit mcp` and confirm `backlogit_merge_sync` appears in the tool list
  via any MCP client.
* Invoke `backlogit_merge_sync` on a live workspace; verify `upserted` and
  `deleted` counts reflect actual file changes.
* Confirm `backlogit_sync_index` still works normally (no regression).

## Risky Action Record

| Action | Risk | Result |
|--------|------|--------|
| Merge commit to main with new MCP tool | low | applied — SHA `f84e3812` |
| CAS version scheme protecting manifest state | moderate | applied — prevents concurrent regression |

## Source Artifact Cleanup

| Type | ID | Result |
|------|----|--------|
| Stash | `source_stash_id` | not set on 037-F — no stash entry to remove |
| Deliberation | `source_deliberation_id` | not set on 037-F; `037-DL` archived by ship command |

## Healthy Signals

* `backlogit_merge_sync` returns `fallback_triggered: false` for normal
  incremental syncs with small diff sets.
* `upserted` count equals the number of `.backlogit/` files touched since the
  last sync.
* `deleted` count equals the number of `.backlogit/` files removed since the
  last sync.
* `failed_paths` is empty on a healthy workspace.

## Failure Signals

* `fallback_triggered: true` on every call — indicates the manifest is not
  persisting between calls or the diff threshold is misconfigured.
* Non-empty `failed_paths` — indicates a file could not be parsed or upserted;
  check the artifact's Markdown frontmatter for validity.
* Stale item counts in `backlogit_query_sql` after a `merge_sync` call — may
  indicate the CAS version update was dropped; follow up with `backlogit_sync_index`.

## Monitoring Plan

`backlogit_merge_sync` is a CLI/MCP tool, not a running service. Monitoring is
call-site observational:

* Watch `failed_paths` in tool responses during active agent sessions.
* Watch `fallback_triggered` frequency: occasional fallbacks are expected for
  large batch changes; frequent fallbacks on small diffs suggest a manifest
  initialization bug.
* Structured logs via `log/slog` are emitted for each `MergeSync` call at
  `INFO` level including `upserted`, `deleted`, `failed`, and `fallback` fields.

## Rollback Trigger

Any of these conditions warrants rollback or follow-up:

* `MergeSync` panics or returns an internal error on a healthy workspace.
* Item counts in `backlogit.db` diverge from `.backlogit/queue/` file counts
  after repeated `merge_sync` calls without intervening `sync_index`.
* Concurrent `handleMergeSync` calls cause observable data loss in the index.

## Rollback Procedure

`backlogit_merge_sync` is stateless beyond the `Server.manifest` field. Rolling
back is:

1. Remove the `backlogit_merge_sync` tool registration from `RegisterTools` in
   `internal/mcp/tools.go` and rebuild the binary.
2. Alternatively, call `backlogit_sync_index` to restore a known-good index state
   without rolling back the binary.

Full regression requires reverting commits `0697f4f` and `f84e3812` and building
from the prior `main` state.

## Validation Window

48 hours from merge. The change is an additive MCP tool with no external
dependencies. If no `failed_paths` or panic reports appear from agent sessions
within that window, the change is considered stable.

## Owner

@softwaresalt — primary maintainer. Any `failed_paths` reports or tool errors
should be filed as bugs against the `backlogit_merge_sync` tool.

## Follow-up Backlog Items

The review gate (037.001-R) surfaced two P2 findings that were archived with
the feature scope. They should be re-harvested for the next sprint:

| Original ID | Title | Priority |
|-------------|-------|----------|
| 037.006-T | MergeSync reports deleted items even when DeleteItemCascade fails | medium |
| 037.007-T | Concurrent handleMergeSync calls can regress manifest to staler snapshot | medium |

These are documented in the prior session memory at
`docs/memory/2026-04-21-ship-038-s-pr-ready.md`.
