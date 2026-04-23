---
title: "Operational Closure — 042-S Data Integrity and Crash-Safety Consistency"
description: "Post-merge closure artifact for shipment 042-S covering release readiness, monitoring, and rollback plan"
author: backlogit ship agent
ms.date: 2026-04-23
ms.topic: reference
---

## Release Summary

| Field | Value |
|---|---|
| Shipment | 042-S — Data Integrity and Crash-Safety Consistency |
| Features | 041-F (File and Index Consistency), 042-F (Data Integrity Hardening) |
| Merge SHA | `9b58927a6380ffb5e79da0de5a925fb0182aedd7` |
| PR | [#60](https://github.com/softwaresalt/backlogit/pull/60) |
| Merged | 2026-04-23 |
| Mode | Merge-only (single binary; no migration, no infra change) |
| Review Gate | PASS — 041.001-R; 0 P0/P1; 2 P2 advisories; 12 Copilot comments resolved across 2 rounds |
| CI | ✅ test (1.23), test (1.24), CLI Reference Drift |

## Change Summary

Five tasks shipped across two features targeting crash-safety and data integrity in the
workspace write layer. No public API, CLI surface, MCP tool schema, or database schema changed.
All changes are confined to internal implementation paths.

### Affected files

| File | Change |
|---|---|
| `internal/core/delete_crashsafe_042.go` | NEW — crash-safe `DeleteArtifact` (rename→DB delete→os.Remove; rollback on each failure stage) |
| `internal/cli/delete.go` | Delegates to `core.DeleteArtifact`; removed inline db+os ops |
| `internal/core/stash.go` | DB-before-JSONL reorder in `harvestStashEntryLocked`; best-effort `db.DeleteItemCascade` rollback on failure paths |
| `internal/core/archive.go` | Unconditional raw-bytes restore on DB failure (removed path-equality guard) |
| `internal/db/rehydration.go` | `isValidLinkType` gate before `item_links` INSERT |
| `internal/db/merge_sync.go` | `isValidLinkType` gate before `item_links` INSERT |
| `internal/db/manifest.go` | `extractItemIDFromFrontmatter` now uses full-file read + `models.ParseFrontmatter` |

### Harness files added

- `internal/core/delete_crashsafe_harness_042_test.go`
- `internal/core/stash_harvest_atomicity_harness_042_test.go`
- `internal/core/archive_consistency_harness_042_test.go`
- `internal/db/rehydration_link_validation_harness_042_test.go`
- `internal/db/manifest_id_extraction_harness_042_test.go`

## Invariants to Preserve

1. After any partial `DeleteArtifact` failure, the artifact file MUST remain discoverable at a
   `.md`-extension path so that rehydration and `FindArtifactPath` can recover it.
2. After a `harvestStashEntryLocked` failure, the DB state and the JSONL state MUST be
   reconcilable via `backlogit sync` — the DB may be ahead of JSONL transiently, never the reverse.
3. Archive restore MUST write original bytes unconditionally when a DB update fails, regardless of
   whether the current path equals the archive path.
4. `item_links` in SQLite MUST only contain rows with a `link_type` value in the set
   `{related_to, duplicate_of, informs, supersedes, spike_ref}`. Invalid values must be logged and
   skipped, never inserted.
5. `extractItemIDFromFrontmatter` MUST return the correct `id` for any valid artifact regardless of
   file size — the former 512-byte scan limit no longer applies.

## Pre-Deploy Audit Checklist

All items below were verified before merge:

- [x] No migration required — SQLite cache is ephemeral and self-healing via `sync`
- [x] No schema changes — `item_links` validation is enforced at INSERT, not via schema ALTER
- [x] No feature flags or rollout gates required — internal implementation only
- [x] No cross-service boundaries involved — single binary, workspace-local
- [x] Rollback procedure documented below
- [x] All harness tests pass (`go test ./...` green on 1.23 and 1.24)

## Deployment Path

Merge-only. No deployment step beyond merging to `main`. The binary is distributed via
`go install github.com/softwaresalt/backlogit/cmd/backlogit@latest`. Users get the fix on their
next install or upgrade.

## Post-Deploy Checks

These smoke checks apply to the local development environment after upgrading the binary:

1. **Delete round-trip** — create an artifact, delete it, confirm it is no longer listed by
   `backlogit list` and the file is absent from `.backlogit/queue/`.
2. **Stash harvest** — add a stash entry and harvest it; confirm the entry moves to `harvested`
   state in both `backlogit stash list` and the SQLite `stash_entries` table.
3. **Archive round-trip** — archive a task and unarchive it; confirm the file lands back in
   `.backlogit/queue/` with correct frontmatter.
4. **Rehydration with links** — run `backlogit sync`; confirm no `WARN` entries about invalid
   `link_type` values in the structured log output (assuming no pre-existing corrupt data).

## Risky Action Record

| Action | Risk | Approval | Result |
|---|---|---|---|
| Reorder DB-before-JSONL in stash harvest | Moderate — transient DB-ahead-of-JSONL inconsistency | Implicit (operator running shipment) | Applied: `9b58927` |
| Unconditional archive restore (removes path-equality guard) | Moderate — changes existing branch behavior on DB failure | Implicit | Applied: `9b58927` |
| os.Remove failure → rename temp back to original | Low — best-effort recovery; DB row is already gone | Implicit | Applied: `f2d7af0` → `9b58927` |

## Source Artifact Cleanup

Features 041-F and 042-F have no `source_stash_id` or `source_deliberation_id` in their
`custom_fields`. No source artifact cleanup required.

## Healthy Signals

- `go test ./...` passes on all future PRs with no regression in the `internal/core` and
  `internal/db` packages.
- No `.deleting.md` orphan files accumulate in `.backlogit/queue/` during normal use.
- `backlogit sync` completes without `WARN` entries about invalid link types on a clean workspace.
- Archive and stash harvest operations complete without errors in the structured log.

## Failure Signals

- `.deleting.md` files accumulating in `.backlogit/` across sessions — indicates partial delete
  operations that crash recovery is not cleaning up.
- Stash entries showing `harvested` in DB but `active` in `stash.jsonl` after multiple sync cycles
  — indicates the JSONL rewrite is failing silently.
- `item_links` growing with invalid `link_type` values — would appear as FTS or query errors.
- `extractItemIDFromFrontmatter` returning empty IDs for large files — would cause manifest build
  to silently drop artifacts from the index.

## Monitoring Plan

backlogit is a CLI tool distributed as a static binary — no live server to instrument. Monitoring
is passive:

| Signal | Observation method |
|---|---|
| Test regressions | CI gates on future PRs (`go test ./...` on 1.23 and 1.24) |
| Orphaned `.deleting.md` files | Manual workspace inspection or `backlogit sync` output |
| Stash consistency | `backlogit query "SELECT state FROM stash_entries"` post-harvest |
| Invalid link types in DB | `backlogit query "SELECT DISTINCT link_type FROM item_links"` |
| Large-file manifest extraction | Review `extractItemIDFromFrontmatter` test coverage for >4KB files |

No external dashboard, alert rule, or SLI is applicable for a local CLI tool.

## Rollback Trigger

Any of the following should prompt investigation and possible revert:

- User reports that `backlogit delete` leaves orphaned files that survive `backlogit sync`
- User reports that stash entries re-appear as active after harvest following a `backlogit sync`
- `backlogit sync` emits errors about invalid link types on a previously clean workspace
- `backlogit list` is missing artifacts that exist as valid `.md` files in `.backlogit/queue/`

## Rollback Procedure

1. `git revert 9b58927` on a new branch, open a PR with the regression description.
2. Run `backlogit sync` on affected workspaces to rehydrate DB from Markdown source.
3. For any orphaned `.deleting.md` files: rename `<name>.deleting.md` → `<name>.md` and run
   `backlogit sync` to restore the artifact to the index.

## Validation Window

**Duration**: 1 week of normal development use.
**Owner**: Next engineer to use `backlogit delete`, `backlogit stash harvest`, or
`backlogit archive` commands on a real workspace.

## Follow-up Backlog

| ID | Title | Status |
|---|---|---|
| 042.003-T | Add CHECK constraint on `item_links.link_type` | Queued |

The runtime validation gate (isValidLinkType) was shipped; the schema-level CHECK constraint
is follow-up work in 042.003-T.

## Readiness Status

**READY** — all invariants verified by passing harness tests, CI green on both Go versions,
12 Copilot review comments resolved, no P0/P1 findings. Internal-only changes with no migration
or deployment risk. Follow-up 042.003-T is tracked in the backlog.
