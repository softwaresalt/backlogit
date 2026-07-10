---
chunk_strategy: h1-h2-h3
description: Use git-aware moves for tracked backlog artifacts so archive and restore operations stage rename intent, avoid commit-a omission of new archive files, and support index-aware rollback.
doc_type: learning
docline:
    category: best-practices
    component: archive-lifecycle
    date: 2026-07-10T00:00:00Z
    file_path: internal/core/archive.go
    message: Tracked backlog artifacts archived with plain filesystem moves lose staged rename intent and can leave the new archive file untracked if committed carelessly.
    problem_type: workflow_issue
    resolution_type: code_fix
    resolved: true
    root_cause: archive and restore code treated Git-tracked Markdown artifacts like ordinary files instead of staging VCS rename intent.
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
title: Git-aware backlog artifact archival stages tracked renames safely
---

## Problem

Backlog work items are Markdown files that move between queue and archive paths.
Before `088-S`, archive and restore paths used filesystem writes and removals
even when an artifact was already tracked by Git. The content was preserved, but
the Git index did not show an intentional rename, which made operator review and
rollback less reliable.

The larger footgun is commit discipline: a filesystem move creates a deleted
source and a new archive file. A careless `git commit -a` can stage the deletion
and modified tracked files while omitting the new untracked archive file. That is
where history and traceability are genuinely lost.

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

The guaranteed benefits are staged rename state, protection from the untracked
new-file loss footgun, and index-aware rollback. Git is not required solely for
history retention: once both paths are committed, an equivalent filesystem move
can also be followed by Git when content similarity is sufficient.

Tests in `internal/core/archive_git_test.go` cover tracked archive/restore
rename staging, `git log --follow` regression evidence, untracked fallbacks,
nested worktrees, fail-closed probe errors, timeout handling, and rollback.

## Guarantee scope and caveat

`git mv` guarantees the **rename is staged**: the delete/add pair is staged
atomically and the index shows intentional rename state. It also lets rollback
reverse the staged move through Git if a later content rewrite or DB update
fails.

`git mv` does **not** by itself guarantee `git log --follow` history. Git stores
no rename metadata, so `--follow` relies on a content-similarity heuristic
computed at diff time. An equivalent filesystem move can be followed too once
both paths are committed and similarity is sufficient. For short or
heavily-rewritten artifacts, committing the move together with a large
frontmatter/content rewrite can push similarity below Git's rename threshold and
break `--follow`.

Use the `--follow` tests as regression evidence that the implementation can
produce a committed shape Git follows, not as proof that `git mv` uniquely
retains history. For short or heavily-edited artifacts, keep the rename and the
content rewrite in **separate commits**: commit the pure move first, then commit
the frontmatter/content rewrite. The short-artifact case in
`archive_git_test.go` depends on exactly this ordering.

## Prevention

When a code path relocates repository-managed Markdown artifacts, do not treat it
as a plain file move by default. First decide whether staged rename intent,
commit-a safety, and index-aware rollback matter for review or traceability. If
they do, use Git-aware move planning for tracked files and keep explicit
filesystem fallbacks for contexts where Git is unavailable or the artifact is not
tracked.

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
