---
chunk_strategy: h1-h2-h3
description: Use git-aware moves for tracked backlog artifacts so archive and restore operations preserve Git rename staging (and, under Git's similarity heuristic, follow-history) while keeping filesystem fallbacks for untracked and non-git workspaces. Note the single-commit caveat for short/heavily-rewritten artifacts.
doc_type: learning
docline:
    category: best-practices
    component: archive-lifecycle
    date: 2026-07-10T00:00:00Z
    file_path: internal/core/archive.go
    message: Tracked backlog artifacts archived with plain filesystem moves lose Git rename staging and make follow-history harder to audit.
    problem_type: workflow_issue
    resolution_type: code_fix
    resolved: true
    root_cause: archive and restore code treated Git-tracked Markdown artifacts like ordinary files instead of preserving VCS rename semantics.
    severity: medium
    tags:
        - archive-safety
        - git
        - git-mv
        - follow-history
        - traceability
        - rollback
    shipped_in: 088-S
    merge_commit: e370af1d737a2fcf881f865a92a354874487b766
ingested_at: "2026-07-10T00:00:00Z"
schema_version: "1.0"
source: docs/compound/best-practices/git-aware-backlog-artifact-archival-preserves-follow-history-2026-07-10.md
title: Git-aware backlog artifact archival preserves follow-history for tracked work items
---

# Git-aware backlog artifact archival preserves follow-history

## Problem

Backlog work items are Markdown files that move between queue and archive paths.
Before `088-S`, archive and restore paths used filesystem writes and removals
even when an artifact was already tracked by Git. The content was preserved, but
the working tree did not show an intentional rename, which made follow-history
queries and operator review less reliable for long-lived backlog artifacts.

## Root Cause

The archive lifecycle did not distinguish tracked Git artifacts from untracked or
non-git files. It also needed to preserve existing safety behavior: non-git
workspaces, missing `git`, untracked artifacts, same-path archives, occupied
archive destinations, and rollback paths must continue to work without requiring
Git.

## Resolution

`internal/core/archive.go` now plans artifact moves before archive and restore:

1. Detect whether `git` is available and whether the workspace belongs to a
   valid worktree.
2. Convert source and destination paths to repository-relative paths.
3. Use `git ls-files --error-unmatch` to require that the source artifact is
   tracked before selecting a Git move.
4. Use `git mv` for tracked archive and restore moves.
5. Fall back to filesystem moves for non-git workspaces, missing `git`, and
   untracked artifacts.
6. Fail closed for unexpected Git probe errors and roll back both content and
   staged Git moves if the content rewrite or DB update fails.

Tests in `internal/core/archive_git_test.go` cover tracked archive/restore
rename staging, `git log --follow` preservation, untracked fallbacks, nested
worktrees, fail-closed probe errors, timeout handling, and rollback.

## Guarantee scope and caveat

`git mv` guarantees the **rename is staged** — the delete/add pair is staged
atomically and the working tree shows an intentional rename. It does **not** by
itself guarantee `git log --follow` history: Git stores no rename metadata, so
`--follow` relies on a content-**similarity heuristic** computed at diff time. For
short or heavily-rewritten artifacts, staging the move together with a large
frontmatter/content rewrite in a **single commit** can push similarity below Git's
rename threshold and break follow-history.

To guarantee follow-history for such artifacts, keep the rename and the content
rewrite in **separate commits**: commit the pure move first (high similarity → the
rename is detected), then commit the frontmatter/content rewrite. The short-artifact
case in `archive_git_test.go` depends on exactly this ordering. Stated precisely:
git-aware archival guarantees **rename staging**; follow-history is preserved when
the rename is committed with sufficient similarity (use the two-commit pattern for
short or heavily-edited artifacts).

## Prevention

When a code path relocates repository-managed Markdown artifacts, do not treat it
as a plain file move by default. First decide whether preserving Git rename
semantics matters for review, history, or traceability. If it does, use Git-aware
move planning for tracked files and keep explicit filesystem fallbacks for
contexts where Git is unavailable or the artifact is not tracked.

## Evidence

- `docs/closure/2026-07-09-088-S-git-aware-archival-closure.md` records shipment
  `088-S`, PR #198, and merge commit
  `e370af1d737a2fcf881f865a92a354874487b766`.
- `internal/core/archive.go` implements `planArtifactMove`, `performArtifactMove`,
  and `rollbackGitArtifactMove`.
- `internal/core/archive_git_test.go` includes
  `TestArchiveItem_TrackedGitArtifactUsesGitMoveAndPreservesFollowHistory`,
  `TestArchiveItem_UntrackedGitArtifactUsesFilesystemFallback`, and
  `TestUnarchiveItem_TrackedGitArtifactUsesGitMoveBack`.

## Related

- `docs/compound/db-reliability/archived-from-invertible-unarchive-2026-06-27.md`
  covers canonical restore paths and unarchive invertibility.
- `docs/compound/workflow-issues/source-artifact-archival-pattern-2026-04-20.md`
  covers post-merge cleanup of source stash entries and deliberations.
