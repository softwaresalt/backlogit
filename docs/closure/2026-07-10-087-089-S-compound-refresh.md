---
chunk_strategy: h1-h2-h3
description: Compound-refresh report for shipped work 087-S, 088-S, and 089-S.
doc_type: closure
docline:
    ms.date: 2026-07-10T00:00:00Z
    ms.topic: reference
schema_version: "1.0"
source: docs/closure/2026-07-10-087-089-S-compound-refresh.md
title: 087-S through 089-S Compound Refresh
---

## Scope

This refresh reconciled existing compound learnings against shipped work:

- `087-S` docs install-path clarity.
- `088-S` git-aware backlog artifact archival.
- `089-S` GitHub Actions CI cost reduction.

Evidence came from the shipped closure artifacts, current workflow files,
archive implementation/tests, and the existing compound library.

## Reviewed entries

| Entry | Classification | Evidence | Action |
|---|---|---|---|
| `docs/compound/runtime-errors/stale-binary-sqlite-out-of-memory-after-schema-merge-2026-04-13.md` | update | `088-S` exposed the same root pattern: stale installed v1.4.1 `C:\Tools\backlogit.exe` hid current v1.4.2 git-aware archival behavior. | Broadened from SQLite-only symptom to stale-binary trap and added 088-S evidence. |
| `docs/compound/github-actions/dorny-paths-filter-every-quantifier-semantics-2026-07-04.md` | update | `.github/workflows/ci.yml` now ships the anchor-plus-negations `every` pattern and job-level gating. | Updated the previously deferred learning to cite 089-S shipped behavior and the current filter shape. |
| `docs/compound/workflow-issues/cli-reference-drift-check-manual-edits-bypass-gen-docs-2026-04-25.md` | update | `089-S` consolidated `CLI Reference Drift` into `.github/workflows/ci.yml`. | Updated workflow location and current generator command while preserving the core generated-docs guidance. |
| `docs/compound/github-actions/F013-workflow-sha-pinning.md` | update | 089-S reduced CI to one Go 1.24 `test` context and kept SHA-pin requirements. | Updated the Go-version guidance and added a 2026-07-10 refresh note for PR-only CI and required contexts. |
| `docs/compound/db-reliability/archived-from-invertible-unarchive-2026-06-27.md` | keep | 088-S changes move artifacts with Git when tracked; they do not alter canonical `archived_from` semantics. | Kept unchanged; related but not superseded. |
| `docs/compound/workflow-issues/source-artifact-archival-pattern-2026-04-20.md` | keep | 088-S changes file move mechanics; this entry covers post-merge source stash/deliberation cleanup. | Kept unchanged; distinct workflow learning. |

## New learning

Created
`docs/compound/best-practices/git-aware-backlog-artifact-archival-preserves-follow-history-2026-07-10.md`
for the durable 088-S pattern: tracked repository artifacts should use
Git-aware move planning to preserve rename staging and follow-history, while
retaining explicit fallbacks for untracked and non-git contexts.

## Stale, superseded, or consolidated entries

No entries were marked stale, superseded, consolidated, or archived. The reviewed
archive-related entries are adjacent but not duplicates, and the CI-related
entries remained valuable after in-place refresh.

## Deliberate non-additions

- No new 087-S install-path learning was added. The shipment clarified user-facing
  docs, but no recurring implementation gotcha or durable troubleshooting pattern
  beyond the shipped docs was identified.
- No separate 089-S cost-reduction learning was added. The durable pieces were
  already represented by the dorny filter, CLI drift, and F013 workflow learnings,
  which were updated in place instead.
- No duplicate stale-binary learning was added. The existing stale-binary entry
  was the right home and was updated with the v1.4.1 versus v1.4.2 evidence.

## Follow-up

None.
