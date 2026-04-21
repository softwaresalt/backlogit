---
title: "Ship 038-S: PR #52 ready, awaiting merge approval"
description: "Session memory for 038-S shipment — backlogit_merge_sync MCP tool. PR open, all CI green, zero review comments."
ms.date: 2026-04-21
---

## Shipment State

| Field | Value |
|---|---|
| Shipment | 038-S |
| Feature | 037-F (backlogit_merge_sync incremental cache-refresh) |
| Branch | `feat/038-mcp-merge-sync` |
| PR | [#52](https://github.com/softwaresalt/backlogit/pull/52) |
| Status | **Active — awaiting user merge approval** |

## CI Status (all green)

| Check | Result |
|---|---|
| `test (1.23)` | SUCCESS |
| `test (1.24)` | SUCCESS |
| `CLI Reference Drift` | SUCCESS |

Review comments: **None**

## Completed Items

| ID | Title | Status |
|---|---|---|
| 037.001-T | Manifest data types and ClassifyFile logic | done |
| 037.002-T | BuildManifest and ComputeDiff | done |
| 037.003-T | ShouldFallback and RehydrateWithManifest | done |
| 037.004-T | MergeSync orchestrator (merge_sync.go) | done |
| 037.005-T | MCP handleMergeSync tool handler | done (archived) |

## Key Technical Decisions

- **BuildManifest hidden-dir fix**: Guard is `path != workspacePath && strings.HasPrefix(d.Name(), ".")` — never skip workspace root even when it is a hidden directory.
- **ShouldFallback `total > 1` guard**: Percentage threshold only applies when ≥2 files changed; single-file changes can't trigger percentage fallback.
- **Lock ordering**: `workspaceMu` before `manifestMu`; locks NOT held simultaneously — RLock released before `MergeSync()` call.
- **nil-slice normalization**: Fallback path result slices normalized to `[]` before JSON serialization for contract compliance.

## Review Gate (037.001-R — PASS)

- **P2-001** — Delete ghost entry: `result.Deleted` populated unconditionally even when `DeleteItemCascade` fails. Follow-up: **037.006-T** (queued, medium).
- **P2-002** — Concurrent manifest regression: no version-check CAS when writing manifest snapshot. Follow-up: **037.007-T** (queued, medium).
- **P3-001** — Double file read in BuildManifest + parseMarkdownArtifact (advisory).
- **P3-002** — Delete operations not batched in a single transaction (advisory).

No P0/P1 findings — gate PASSED.

## Backlog Artifacts Created This Session

- `037.001-R` — review artifact (status: review)
- `037.006-T` — P2 follow-up: stale delete (queued)
- `037.007-T` — P2 follow-up: concurrent manifest regression (queued)

## Next Steps (pending merge approval)

1. **User approves merge** → `gh pr merge 52 --squash` (or merge strategy of choice)
2. Record merge SHA → `backlogit_track_commit 038-S` and `037-F`
3. Ship shipment → `backlogit shipment ship 038-S --sha <merge-sha>`
4. Invoke `operational-closure` skill
5. Archive `037-DL` deliberation artifact
6. Update `docs/ARCHITECTURE.md` and `README.md` for new `backlogit_merge_sync` tool surface
7. Write compound learnings: hidden-dir bug, ShouldFallback semantics
8. Compact `.copilot-tracking/` if needed
