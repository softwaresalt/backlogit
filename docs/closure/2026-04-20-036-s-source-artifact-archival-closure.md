---
chunk_strategy: h1-h2-h3
description: 'Operational closure for shipment 036-S after merge of PR #47 to main at 9a7af9f.'
doc_type: closure
docline:
    ms.date: 2026-04-20T00:00:00Z
ingested_at: "2026-06-26T02:32:32Z"
schema_version: "1.0"
source: docs/closure/2026-04-20-036-s-source-artifact-archival-closure.md
title: '036-S Post-Merge Closure: Workflow Hygiene — Source Artifact Archival Pattern'
---

## Summary

Shipment 036-S added a mandatory source artifact archival step to the Ship agent's
post-merge closure protocol. When a shipment ships, the agent now traces each
feature back to its originating stash entry and deliberation artifact, removes the
stash entry via `backlogit_stash_remove`, archives the deliberation via
`backlogit_archive_item`, and logs the results in this closure section.

The operational-closure skill checklist was also updated to include a **Source
Artifact Cleanup** section for consistent traceability across all future shipments.

**Merge commit**: `9a7af9f275657a37251864f1d28308d0364c5ce4`
**PR**: [#47](https://github.com/softwaresalt/backlogit/pull/47)
**Branch**: `feat/036-s-source-artifact-archival` (deleted post-merge)
**Shipped by**: softwaresalt

## Items Shipped

| ID | Title | Status |
|---|---|---|
| 034-F | Workflow Hygiene: Source Artifact Archival Pattern | archived |
| 034.001-T | Update ship.agent.md post-merge closure: source stash removal | archived |
| 034.002-T | Update ship.agent.md post-merge closure: deliberation archival | archived |
| 034.003-T | Update operational-closure skill: Source Artifact Cleanup section | archived |
| 034.004-T | One-Time Cleanup: Archive Stale Harvested Stash Entries | archived |

## Source Artifact Cleanup

### 034-F

* `custom_fields.source_stash_id: B155D9DA` — `backlogit_stash_remove` returned `not_found`;
  `stash.jsonl` was already clean. Stale DB cache records existed for 15+ harvested entries
  from previously shipped work but no active entries remained. Superseded by shipment `036-S`.
* `custom_fields.source_deliberation_id`: not set — 034-F was harvested directly from the
  stash without a deliberation artifact. Nothing to archive.

**Totals**: 0 stash entries removed (1 skipped — already clean), 0 deliberations archived (none linked).

## CI Status

| Check | Result |
|---|---|
| CI/test (Go 1.23) | ✅ Passing |
| CI/test (Go 1.24) | ✅ Passing |
| CLI Reference Drift Check | ✅ Passing |

21 Copilot review threads across 3 review waves — all replied to and resolved.

## Invariants to Preserve

* `backlogit_stash_remove` accepts only `stash_id` — no `reason` parameter. Superseded-by
  linkage is recorded here in the closure artifact, not as a tool parameter.
* `backlogit_archive_item` always stamps `archived_from` on the archived file.
  Any file archived outside the MCP tool surface (workspace repair) must have
  `archived_from` added manually and `backlogit_sync_index` called afterward.
* Ship agent write-only discipline: all `.backlogit/` mutations go through MCP tools or CLI.
  Direct file edits are permitted only for workspace repair (and must be followed by sync).

## Pre-Deploy Audit

This change affects agent protocol files only — no binary changes, no Go code, no database
schema, no API contracts.

* [x] No migration required
* [x] No feature flags — change is protocol-level and takes effect immediately on next agent session
* [x] No external service dependencies introduced
* [x] Backward-compatible: existing closure workflows are unaffected if `source_stash_id` or
  `source_deliberation_id` are not set on a feature

## Deployment / Rollout Path

Merge-only. Agent protocol files in `.github/` take effect on the next Ship agent session.
No deployment or infrastructure steps required.

## Post-Deploy Checks

On the next Ship agent post-merge closure run:

1. Confirm the agent reads `custom_fields.source_stash_id` and calls `backlogit_stash_remove`
   with only the stash ID (no reason parameter).
2. Confirm the agent reads `custom_fields.source_deliberation_id` and calls
   `backlogit_archive_item` when the deliberation exists and is not already archived.
3. Confirm the broadcast message matches:
   `[SHIP] Source artifacts archived: {stash_count} stash, {delib_count} deliberations`
4. Confirm archived source IDs are logged in the closure artifact's **Source Artifact Cleanup**
   section.

## Risky Action Record

| Action | Risk | Approval | Result |
|---|---|---|---|
| Merge PR #47 to main | low | User-approved | applied — `9a7af9f` |
| Workspace repair: create `.backlogit/archive/033-F.md` | low | Self-approved (tool surface gap) | applied — `archived_from` stamped, sync run |
| Workspace repair: archive 033.002-R review artifact | low | Self-approved (tool surface gap) | applied — moved to archive with parent_id and body |

## Healthy Signals

* Post-merge closure runs complete without errors on features that have `source_stash_id` set.
* `backlogit_stash_remove` returns success or `not_found` (both are valid — `not_found` means
  already clean).
* `backlogit_archive_item` returns the archived artifact ID for linked deliberations.
* Closure artifacts include a populated **Source Artifact Cleanup** section.
* `backlogit doctor` reports no new orphaned artifacts from the source archival step.

## Failure Signals

* Ship agent calls `backlogit_stash_remove` with a `reason` parameter — tool will error.
* Ship agent attempts to archive an already-archived deliberation without the idempotency
  guard (skip if already archived or not found).
* `archived_from` frontmatter missing from a newly archived artifact — `UnarchiveItem` will
  fail if reversal is ever needed.
* New orphaned tasks appearing in `backlogit doctor` output after a closure run.

## Monitoring Plan

No runtime metrics surface — change is agent protocol only. Monitoring is behavioral:

* Spot-check the next Ship closure session for correct tool calls and correct closure artifact
  structure.
* Run `backlogit doctor` after each post-merge closure to catch orphaned artifacts.
* Review the **Source Artifact Cleanup** section in future closure artifacts to verify the
  pattern is being applied consistently.

## Rollback Trigger

If the source archival step causes the Ship agent to error out or corrupt `.backlogit/` state,
revert the additions to `ship.agent.md` and `operational-closure/SKILL.md`.

## Rollback Procedure

```bash
git revert 9a7af9f  # Revert the 036-S merge commit
git push origin main
```

The revert restores the prior protocol without affecting any already-archived artifacts
(those are safe — archive is additive and reversible via `backlogit_archive_item`'s
`archived_from` field).

## Validation Window

One full Ship agent execution cycle (next shipment that triggers post-merge closure).
Owner: softwaresalt.

## Readiness Status

**READY** — merge complete, CI clean, no runtime surfaces affected, rollback path is a
clean revert. Protocol change takes effect immediately on the next Ship session.
