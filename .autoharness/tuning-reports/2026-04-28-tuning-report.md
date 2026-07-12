---
title: Harness Tuning Report — 2026-04-28
date: 2026-04-28
status: applied
---

# Harness Tuning Report — 2026-04-28

## Context

Post-merge tuning after PR #79 (legacy Copilot CLI plugin distribution for backlogit). The autoharness
templates have evolved since the 2026-04-26 tuning session, causing two previously "fixed" targeted
checks to fail again. A new targeted check (`ship_source_artifact_cleanup`) was also failing.

## Drift Summary

| Category | Count |
|---|---|
| Breaking (P0) | 0 |
| Degrading (P1 structural) | 3 |
| Degrading (P1 learning-driven) | 2 |
| Growth (P2) | 4 |
| Cosmetic (P3) | 0 |

## Composition

| Item | Value |
|---|---|
| Installed preset | full |
| Schema contracts | All current at 1.0.0 |
| Targeted checks pre-tune | 3 failing |
| Targeted checks post-tune | 0 failing (19/19 pass) |

## Checksum Scan

| Status | Count |
|---|---|
| Unchanged | 36 |
| User-modified | 1 (go.instructions.md — intentional tune modification) |
| Missing | 0 |

## Regression Explanation

The 2026-04-26 tuning run (PR #77) applied patches to `stage.agent.md` and `ship.agent.md`
but the autoharness templates continued to evolve afterward. The 2026-04-28 verify-workspace uses
updated targeted-check patterns that look for more specific guardrail text than was added in
the prior run. This is structural drift through template evolution, not a revert of committed changes.

## Proposals Applied

### TUNE-001 (re-issue, P1 Degrading) — stage.agent.md: Step Sequence Contract regression

**Check:** `stage_shipment_determinism`
**Issue:** Missing `Step Sequence Contract (NON-NEGOTIABLE)`, `Shipment Assembly (NON-NEGOTIABLE when shipments are
supported)`, `Pre-Summary Verification Gate (NON-NEGOTIABLE)`, and "Never skip shipment assembly"
**Prior fix:** 2026-04-26 TUNE-001 — marked Applied/Verified but templates evolved
**Resolution:** Added `## Step Sequence Contract (NON-NEGOTIABLE)` section with step-completion
checklist, renamed Step 5 with NON-NEGOTIABLE designation and guardrail text, added
`Pre-Summary Verification Gate (NON-NEGOTIABLE)` to Step 6.
**Status:** Applied ✓ | Verified ✓ (targeted check `stage_shipment_determinism`: ok)

### TUNE-002 (re-issue, P1 Degrading) — ship.agent.md: Branch management regression

**Check:** `ship_branch_management`
**Issue:** Missing `Branch retention (NON-NEGOTIABLE)`, `Post-Merge Branch Protocol (NON-NEGOTIABLE)`,
`Branch Management Rules (NON-NEGOTIABLE)`, `post-merge/{feature_slug}`
**Prior fix:** 2026-04-26 TUNE-002 — marked Applied/Verified but templates evolved
**Resolution:** Added `## Branch Management Rules (NON-NEGOTIABLE)` section covering branch retention,
`post-merge/{feature_slug}` branch convention, PR-per-branch, and no-auto-merge requirements.
Added `#### Step 5.0: Post-Merge Branch Protocol (NON-NEGOTIABLE)` sub-step in Step 5.
**Status:** Applied ✓ | Verified ✓ (targeted check `ship_branch_management`: ok)

### TUNE-003 (new, P1 Degrading) — ship.agent.md: Source artifact cleanup

**Check:** `ship_source_artifact_cleanup`
**Issue:** Missing `source_stash_id`, `source_deliberation_id`, `backlogit_stash_remove`,
`backlogit_archive_item` in ship.agent.md
**Resolution:** Added `#### Step 5.2: Source Artifact Cleanup (backlogit only)` sub-step in Step 5
with explicit guidance on reading `source_stash_id`/`source_deliberation_id` from `custom_fields`
and calling `backlogit_stash_remove`/`backlogit_archive_item` to retire consumed artifacts.
**Status:** Applied ✓ | Verified ✓ (targeted check `ship_source_artifact_cleanup`: ok)

### TUNE-004 (P1 Degrading, learning-driven) — go.instructions.md: Error type selection

**Pattern:** `incorrect_error_type` × 5 compound entries (evidence_count ≥ 5 → P1 escalation)
**Evidence refs:**
- `docs/compound/best-practices/crash-safe-delete-rename-rollback-go-2026-04-23.md`
- `docs/compound/best-practices/go-file-write-short-write-guard-2026-04-23.md`
- `docs/compound/runtime-errors/nodejs-https-redirect-missing-location-2026-04-28.md`
- `docs/compound/workflow-issues/github-pr-bulk-resolve-review-threads-graphql-2026-04-23.md`
- `docs/compound/workflow-issues/pr-review-comment-reply-protocol-2026-04-10.md`
**Issue:** Instructions lacked guidance on when to choose sentinel vs typed struct vs wrapped error
types, leading to recurring use of the wrong type for a given situation.
**Resolution:** Added `### Error Type Selection` sub-section to `## Error Handling` with a decision
table mapping situation → error type → example.
**Status:** Applied ✓ (source: learning-driven)

### TUNE-005 (P1 Degrading, learning-driven) — go.instructions.md: Schema synchronization

**Pattern:** `schema_mismatch` × 5 compound entries (evidence_count ≥ 5 → P1 escalation)
**Evidence refs:**
- `docs/compound/config-issues/go-xterm-unpinned-dep-bumps-go-directive-2026-04-20.md`
- `docs/compound/go-patterns/f015-shipment-stash-patterns.md`
- `docs/compound/workflow-issues/cli-reference-drift-check-manual-edits-bypass-gen-docs-2026-04-25.md`
- `docs/compound/workflow-issues/orphaned-tasks-without-parent-features-2026-04-10.md`
- `docs/compound/workflow-issues/stable-contract-before-two-agent-adoption-2026-04-05.md`
**Issue:** No guidance on keeping Go struct tags synchronized with YAML frontmatter schemas; silent
mismatches cause rehydration failures and query errors.
**Resolution:** Added `### Schema Synchronization` sub-section to `## Patterns to Follow` with
explicit rules for adding, renaming, and removing struct fields relative to YAML frontmatter.
**Status:** Applied ✓ (source: learning-driven)

## Deferred Proposals (P2 Growth — learning-driven)

These patterns were detected but not actioned in this tuning session. Recommend revisiting in the
next tuning cycle if compound library continues to grow in these areas.

| ID | Pattern | Evidence | Suggested Action |
|---|---|---|---|
| TUNE-006 | `cli` component hotspot | 8 entries | Review harness coverage for CLI patterns; consider CLI-specific review checklist |
| TUNE-007 | `task_manager` component hotspot | 6 entries | Consider workflow-patterns instruction or review persona addition |
| TUNE-008 | `go` cross-cutting tag | 6 entries | go.instructions.md already updated; re-evaluate after next compound refresh |
| TUNE-009 | `sqlite`/`rehydration` cross-cutting tags | 4 entries each | Consider a db-patterns or rehydration-reliability instruction file |

## New Surface (legacy plugin distribution — informational)

PR #79 introduced a retired JavaScript wrapper and `plugin/` (Copilot CLI plugin). These are
application code surfaces, not harness artifacts. No harness instruction file exists for Node.js.
If the npm surface grows (additional Node.js tooling, CI jobs for npm publishing), consider adding a
`nodejs.instructions.md` or `npm-distribution.instructions.md` to cover Node.js coding conventions
and npm publishing patterns.

## Severity Trend

16 of 30 compound entries are high/critical severity (53%). This is elevated and warrants attention.
The `workflow_issue` category dominates (12/30 = 40%), suggesting the harness's workflow guidance
continues to be the highest-leverage area for improvement.

## Verification Results (post-tune)

```
autoharness verify-workspace: 0 strict_schema_blockers, 0 warnings
Checksum scan: 36 unchanged, 1 user-modified (go.instructions.md)
Targeted checks: 19/19 pass (0 failing)
Schema contracts: manifest, config, profile — all current at 1.0.0
```

## Files Modified

| File | Change |
|---|---|
| `.github/agents/stage.agent.md` | TUNE-001: Step Sequence Contract + Shipment Assembly NON-NEGOTIABLE + Pre-Summary Verification Gate |
| `.github/agents/ship.agent.md` | TUNE-002: Branch Management Rules NON-NEGOTIABLE + Post-Merge Branch Protocol; TUNE-003: Source Artifact Cleanup |
| `.github/instructions/go.instructions.md` | TUNE-004: Error Type Selection table; TUNE-005: Schema Synchronization rules |
| `.autoharness/harness-manifest.yaml` | tuned_at bump to 2026-04-28, go.instructions.md checksum updated, session entry added |

## Backups

Pre-tune backups at `.autoharness/backups/2026-04-28/`:
- `stage.agent.md`
- `ship.agent.md`
- `go.instructions.md`
