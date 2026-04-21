---
title: "Closure: 037-S AdoptItem Cross-Reference Rewrite"
description: "Post-merge operational closure for shipment 037-S — data integrity fix for AdoptItem cross-artifact frontmatter reference staleness"
ms.date: 2026-04-20
---

## Context

| Field | Value |
|---|---|
| Shipment | 037-S |
| Feature | 035-F: Data Integrity — AdoptItem Cross-Reference Rewrite |
| Branch | `feat/035-adoptitem-cross-reference` |
| PR | #49 (implementation), #50 (closure state) |
| Merge SHA (implementation) | `1e2442f` |
| CI status | 3/3 green (both PRs) |
| Review gate | Passed — 0 P0/P1, 2 P2, 2 P3 findings; 5 Copilot comments resolved |
| Readiness status | **READY** |

## Summary of Change

`AdoptItem` previously rewrote the adopted artifact's hierarchical ID and renamed
its `.md`/`.jsonl` files, but left all other artifacts' Markdown frontmatter
unchanged. Any artifact referencing the old ID in `parent_id`, `dependencies`, or
`links` would have its corrected DB edges silently reverted on the next
`backlogit sync` because stale Markdown is the source of truth.

The fix adds a two-phase cross-artifact reference scanner and rewriter integrated
atomically into `AdoptItem`:

1. `findCrossArtifactReferences` — pre-transaction filesystem walk; collects every
   artifact whose frontmatter references `oldID`; produces deep-copy structs with
   `oldID` replaced by `newID`.
2. `applyCrossArtifactRewrites` — inside the adoption transaction; writes updated
   Markdown files (atomic tmp+rename), upserts DB rows, and preserves `dep_type`
   via SELECT-before-DELETE; rolls back written files via atomic tmp+rename on
   any transaction failure.

`AdoptItemResult.RewrittenArtifactIDs []string` is populated with the IDs of all
rewritten artifacts for caller traceability.

## Files Changed

| File | Change |
|---|---|
| `internal/core/artifact_references.go` | New — `crossRefUpdate`, `findCrossArtifactReferences`, `applyCrossArtifactRewrites` (280 lines) |
| `internal/core/artifact_references_test.go` | New — 11 unit tests (377 lines) |
| `internal/core/035_adoptitem_cross_ref_harness_test.go` | New — 3 integration tests incl. rehydration-consistency check (125 lines) |
| `internal/core/shipment_lifecycle.go` | Modified — `AdoptItemResult.RewrittenArtifactIDs`, integration wiring, `log/slog` import, atomic rollback (67 lines changed) |

## Invariants to Preserve

* After any `AdoptItem` call where `oldID != newID`, zero artifacts in the
  workspace should retain `oldID` in their `parent_id`, `dependencies`, or
  `links` frontmatter fields.
* A full `backlogit sync` immediately following `AdoptItem` must produce identical
  DB edges to the in-transaction state — no stale reversions.
* Adoption that fails mid-way must leave the workspace in a consistent state:
  either fully applied or fully rolled back (both Markdown files and DB rows).
* `dep_type` values on preserved dependencies must not silently default to
  `"blocks"` when an explicit type was previously recorded.

## Pre-Deploy Audits

This change is merge-only with no deployment step, schema migration, feature
flags, or external integration dependencies. The following were verified before
merge:

* `go test -race ./...` — all 14 new tests pass; full suite green
* `golangci-lint run` — zero findings
* `go vet ./...` — clean
* `gofmt -l .` — no unformatted files
* Integration test `TestAdoptItem_CrossReferenceRehydrationConsistency` explicitly
  reproduces the bug (pre-fix) and verifies the fix (post-fix)
* Atomic rollback path covered by `TestApplyCrossArtifactRewrites_RollbackOnError`

## Deployment Path

Merge-only. No deployment, migration, or feature flag needed. The fix takes effect
for any workspace on the updated binary.

## Post-Deploy Checks

For any workspace that uses `backlogit adopt` / `AdoptItem`:

1. Run `backlogit adopt <item-id> <new-parent-id>` on a test artifact.
2. Run `backlogit sync` immediately after.
3. Verify `backlogit query "SELECT id, parent_id FROM items WHERE id = '<new-id>'"` returns
   the updated parent, not the old one.
4. Verify no artifact in the workspace retains the old ID in `parent_id`:
   ```sql
   SELECT id, parent_id FROM items WHERE parent_id = '<old-id>'
   ```
   Should return zero rows.

## Risky Action Record

| Action | Risk | Approval | Result |
|---|---|---|---|
| Atomic filesystem walk inside library code (`filepath.WalkDir` over all `.backlogit/` `.md` files) | Low — read-only scan, bounded by workspace size | N/A (low risk) | Applied |
| Atomic tmp+rename for rollback writes inside transaction failure path | Moderate — file system mutation on error path | Covered by unit test | Applied |
| SELECT-before-DELETE on `item_deps` to preserve `dep_type` | Low — read + write within transaction | N/A | Applied |

## Source Artifact Cleanup

| Type | ID | Outcome |
|---|---|---|
| Stash entry | C00AA592 | Already removed (not found — removed prior to this closure) |
| Deliberation | — | No `source_deliberation_id` on 035-F |

Source artifact cleanup: 0 stash (pre-removed), 0 deliberations.

## Follow-Up Tasks

Two follow-up tasks were created during the review gate and remain in the backlog:

| ID | Title | Priority |
|---|---|---|
| 035.004-T | Propagate `context.Context` through `findCrossArtifactReferences`/`applyCrossArtifactRewrites` | Medium |
| 035.005-T | Harden `dep_type` preservation with explicit enum validation | Low |

These are non-blocking improvements and do not affect the correctness of the merged fix.

## Healthy Signals

* `AdoptItem` calls with `oldID != newID` complete without error.
* `AdoptItemResult.RewrittenArtifactIDs` is non-empty when sibling/child
  artifacts existed that referenced the old ID.
* `backlogit sync` after adoption produces identical DB edges (no stale reversions).
* No `parent_id`/`dependencies`/`links` field in any `.md` file retains the old
  adopted ID after a successful adoption.

## Failure Signals

* Adoption operations over large workspaces (>500 `.md` files) take noticeably
  longer than expected — the `filepath.WalkDir` scan is O(n) over artifacts.
  Track as a performance signal; the 035.004-T context-propagation task enables
  cancellation for long-running scans.
* Any `parent_id` in the DB still pointing to the old ID after adoption +
  `backlogit sync` indicates a regression in the rehydration path or a missed
  file in the scan.
* Disk write errors during `applyCrossArtifactRewrites` that leave partial state —
  watchable via `slog` ERROR entries from `applyCrossArtifactRewrites`.

## Monitoring Plan

This is a pure Go library change with no network surface, HTTP endpoint, or
external service dependency. Monitoring is code-path-level:

* **Signal**: `slog` ERROR entries from `findCrossArtifactReferences` or
  `applyCrossArtifactRewrites` in MCP server logs.
* **Baseline**: Zero ERROR entries from these functions in normal adoption flows.
* **Alert threshold**: Any ERROR from `applyCrossArtifactRewrites` during production
  use indicates a partial rollback occurred and manual workspace inspection is warranted.
* **Dashboard**: Not applicable — observe via local `backlogit` log output or
  MCP server stderr.
* **Owner**: softwaresalt

## Rollback Trigger

Any observed state where:
* `backlogit sync` after adoption reverts adopted-artifact edges (stale `parent_id`
  still appears in DB), OR
* a workspace `.md` file retains the old adopted ID in `parent_id` or
  `dependencies` after a successful `backlogit adopt` call

indicates the cross-reference fix is not applying correctly. Treat as rollback
trigger.

## Rollback Procedure

```bash
# Revert the implementation commit on a branch and open a PR
git checkout main
git pull origin main
git checkout -b revert/037-s-adoptitem-cross-ref
git revert 1e2442f --no-edit
git push origin revert/037-s-adoptitem-cross-ref
gh pr create --title "revert(core): roll back AdoptItem cross-reference rewrite" --base main
```

After revert, `AdoptItem` returns to pre-fix behaviour: only the adopted artifact's
own files are updated; other artifacts retain old IDs until manually corrected.
The revert does not require any data migration.

## Validation Window

48 hours post-merge. This is pure library code with no deployment step; risk is
bounded to workspaces that call `AdoptItem` with `oldID != newID`. Normal adoption
operations in the backlogit workspace itself provide the first real-world signal.

## Owner

softwaresalt
