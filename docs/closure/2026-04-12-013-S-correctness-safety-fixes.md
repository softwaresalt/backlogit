---
title: "013-S Closure: Correctness & Safety Fixes"
description: Post-merge closure record for shipment 013-S
ms.date: 2026-04-12
ms.topic: reference
---

## Shipment

**ID**: 013-S
**Feature**: 029-F — Correctness & Safety Fixes
**Branch**: `ship/013-S-correctness-safety-fixes`
**PR**: [#29](https://github.com/softwaresalt/backlogit/pull/29) — merged to `main` at `aeee58e`

## Delivered

| Task | Title | Status |
|------|-------|--------|
| 029.001-T | Constrain export_command_map path to .backlogit/ | done |
| 029.002-T | Atomic adopt item with ID rewrite | done |
| 029.003-T | Fix query gate semicolons and section-write error handling | done |
| 029.004-T | Fix stash harvest TOCTOU and terminal status gaps | done |
| 029.005-T | Fix stale index.db references in instructions | done |

## Quality Gates

| Gate | Result |
|------|--------|
| `go build ./...` | ✅ Clean |
| `go vet ./...` | ✅ Clean |
| `go test ./...` | ✅ All 16 packages pass |
| CI Go 1.23 | ✅ Pass (3 commits) |
| CI Go 1.24 | ✅ Pass (3 commits) |
| Copilot review | ✅ 16 comments across 2 waves, all addressed |

## Copilot Review Resolutions

### Wave 1 (commit 4c80ccd)

1. `UpsertItemTx` aligned with `UpsertItem`: added `hierarchy_path` column, `RFC3339Nano` timestamps, NULL-safe JSON marshaling.
2. `ValidateQuery` semicolon handling: extracted `semicolonGuard` named regex var, removed brittle index-based check.
3. `stripStringLiterals` comment: fixed Unicode smart-quote typo (U+201D → ASCII representation).
4. `validateSectionName`: added whitespace rejection (`strings.ContainsAny`) matching parser `\S+` constraint.
5. `AdoptItem` file naming: switched from hard-coded `newID+".md"` to `ResolveFileName()` respecting `artifact_types[*].file_name_format`.
6. `AdoptItem` ancillary references: added `RewriteAncillaryReferences` for `commit_links`, `stash_links`, `item_logs`, `item_log_entries`.

### Wave 2 (commit 573b8a9)

7. `writeSectionsToFile` map iteration: sorted section names for deterministic output ordering.
8. `RewriteAncillaryReferences` log_path: fixed to store `.backlogit/`-relative path (`logs/<id>.jsonl`) matching `IndexEvent` convention, not absolute filesystem path.
9. `RewriteAncillaryReferences` comment: corrected misleading "delete+insert" to describe actual UPDATE behavior; removed unused `oldLogPath` parameter.
10. `filterByResolvedDependencies` comment: updated to list full `TerminalStatuses` set (done, accepted, archived, shipped, abandoned, rejected).
11. `writeSectionsToFile` test: added `TestWriteSectionsToFile_MixedExistingAndNew` regression test for mixed existing+new section case.
12. `TestWriteCommandMap_WritesInsideBacklogit` containment: replaced `strings.HasPrefix` with `filepath.Rel` to avoid false positives.
13-15. Markdown table formatting: confirmed as false positives (tables use standard single-pipe format on disk).
16. AdoptItem Markdown frontmatter reference rewrite: acknowledged as known limitation; filed as follow-up (cache-only edge rewrite is sufficient since DB is ephemeral).

## Key Technical Decisions

- **AdoptItem transaction ordering**: DB tx → file ops → commit. Files are the source of truth; the DB is a disposable cache rebuilt via rehydration. This ordering ensures the source of truth is never left in an inconsistent state on crash.
- **Stash harvest TOCTOU fix**: `HarvestStashByPriority` now holds the lock for the entire batch instead of acquiring/releasing per entry. Extracted `harvestStashEntryLocked` internal variant to avoid deadlock.
- **`TerminalStatuses` centralization**: `"rejected"` added to `blocking_cascade.go:TerminalStatuses`. `queue.go` now uses the canonical slice instead of a hardcoded subset, preventing future divergence.
- **Query gate string-literal awareness**: `stripStringLiterals` replaces SQL string contents with placeholders before pattern matching, preventing false positives on semicolons inside string values.
- **Section-write per-section processing**: Each section is processed individually against the current body state, so a mix of existing (update) and missing (append) sections works correctly without duplication.

## Known Limitations / Follow-up

- **Cross-artifact Markdown frontmatter rewrite on adoption**: When an item is adopted with a new ID, other artifacts' Markdown frontmatter that references the old ID in `dependencies` or `links` is not updated. On next `backlogit sync`, rehydration will resurrect edges pointing to the deleted old ID. Filed as follow-up work.
- **Crash-consistency window in AdoptItem**: Between file ops and DB commit, a crash could leave files renamed but the DB transaction uncommitted. This is acceptable because the DB is rebuilt from Markdown on rehydration — the renamed files (source of truth) will produce the correct state.

## Rollback Plan

If issues arise from this shipment:

1. **Revert merge commit**: `git revert aeee58e` on `main` and push. This reverses all code changes.
2. **Rehydrate DB**: `backlogit sync` to rebuild the DB from the reverted Markdown state.
3. **Instructions files**: The `index.db` → `backlogit.db` renames in `.github/instructions/` are cosmetic documentation — reverting restores the old (incorrect) references but has no runtime impact.
4. **No data migration needed**: All changes are code-level fixes with no schema changes or data transformations.
