---
chunk_strategy: h1-h2-h3
description: Closure record for shipment 088-S git-aware archival.
doc_type: closure
docline:
    ms.date: 2026-07-09T00:00:00Z
    ms.topic: reference
schema_version: "1.0"
source: docs/closure/2026-07-09-088-S-git-aware-archival-closure.md
title: 088-S Git-Aware Archival Closure
---

## Summary

Shipment `088-S` delivered git-aware backlog artifact archival for feature
`089-F` and tasks `089.001-T` through `089.004-T`. Tracked artifacts now move
with `git mv` during archive and restore operations when the workspace is inside
a Git worktree. Non-git workspaces, untracked artifacts, and missing git binaries
continue to use filesystem moves.

## Changed surfaces

| Surface | Closure note |
|---|---|
| `internal/core/archive.go` | Adds Git worktree detection, tracked-file detection, `git mv`, fallback, and rollback behavior |
| `internal/core/archive_git_test.go` | Covers tracked git moves, fallback cases, restore symmetry, fail-closed errors, rollback, and environment hardening |
| `.backlogit/` | Shipment `088-S`, feature `089-F`, and all scoped tasks shipped and archived |

## Validation

* Red phase confirmed with `go test ./internal/core/...` before implementation
* `go test ./internal/core/...` passed after implementation
* `go test ./...` passed
* `go vet ./...` passed
* `golangci-lint run` passed
* `gofmt -l internal/core/archive.go internal/core/archive_git_test.go` returned empty output
* PR #198 CI passed:
  * `test (1.23)`
  * `test (1.24)`
  * `CLI Reference Drift`
  * `Detect CLI-affecting changes`
  * `Detect code changes`
  * `Docline frontmatter gate`
* `LOCAL_REVIEW_READY`: local review found no unresolved P0/P1 findings
* Copilot review raised comments across multiple cycles; all valid comments were
  fixed, replied to, and resolved. The final Copilot review on
  `fbda87c8202f4d60cdfb904c1750713cd1f20f94` raised no new comments.

## Merge and release record

* Feature branch: `feat/088-S-git-aware-archival`
* PR: #198
* Reviewed HEAD: `fbda87c8202f4d60cdfb904c1750713cd1f20f94`
* Merge commit: `e370af1d737a2fcf881f865a92a354874487b766`
* Merge path: normal `gh pr merge 198 --merge` was blocked by branch policy:
  `X Pull request softwaresalt/backlogit#198 is not mergeable: the base branch policy prohibits the merge.`
* Dark-mode admin fallback used `gh pr merge 198 --merge --admin`
* P-009 preserved: no squash or rebase merge was used

## Operational notes

`git mv` stages the rename in the Git index. Subsequent backlog artifact
metadata rewrites remain normal worktree edits unless the caller stages them.
Rollback attempts restore file content and reverse staged git moves before
returning wrapped errors.

Runtime monitoring is limited to local and CI quality gates for the CLI and MCP
archive surfaces. Rollback is a normal revert of merge commit
`e370af1d737a2fcf881f865a92a354874487b766` if archive behavior regresses.
