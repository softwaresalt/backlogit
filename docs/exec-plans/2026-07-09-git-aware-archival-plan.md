---
chunk_strategy: h1-h2-h3
description: 'Plan git-aware archive and restore moves so tracked backlog artifacts preserve file history while untracked or non-git workspaces keep the current filesystem move path.'
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-07-09-git-aware-archival-plan.md
title: 'Git-aware archival for tracked backlog artifacts'
---

## Source

* Stash: `44E2B442` (kind=feature, priority=high)
* Scope hint: `internal/core/archive.go` (`ArchiveItem`, `UnarchiveItem`, and
  `canonicalRestorePath`)
* Prior learning: `docs/compound/db-reliability/archived-from-invertible-unarchive-2026-06-27.md`
  confirms archive and restore paths are data-integrity surfaces and must fail
  closed instead of clobbering or stranding records

## Problem Frame

`core.ArchiveItem` currently rewrites archive frontmatter into a temporary file,
renames it to `.backlogit/archive/<item>.md`, and removes the original queue
file with filesystem operations. `core.UnarchiveItem` mirrors that path by
writing restored frontmatter to a temporary file, renaming it back to the
recorded `archived_from` path, then removing the archive copy.

Those moves are correct for untracked files and non-git workspaces. When
`.backlogit/` artifacts are tracked by Git, plain filesystem rename plus remove
does not intentionally update Git's index. Git does not store explicit rename
metadata: `git mv` stages a delete/add pair, and rename or `--follow` behavior
is inferred later from committed snapshots and file similarity. The archive and
restore lifecycle should use `git mv` only when the workspace is inside a Git
repository and the specific artifact path is tracked, then fail closed if the
selected `git mv` itself fails.

## Requirements Trace

| Requirement | Implementation action |
|---|---|
| Detect git repository and per-file tracked state | Add a small helper around bounded `exec.CommandContext` calls to `git rev-parse --show-toplevel` and `git ls-files --error-unmatch -- <path>` with Windows-safe paths and a minimal environment |
| Tracked artifact in git repo uses `git mv` for archive | Replace the final tracked-file move step in `ArchiveItem` with `git mv` after archive frontmatter is prepared, preserving existing destination collision checks and treating `currentPath == archivePath` as an in-place frontmatter update, not `git mv path path` |
| Non-git, untracked, or missing git keeps current behavior | Make the helper return an explicit filesystem-move strategy for these non-error states |
| Restore is symmetric | Reuse the same strategy for `UnarchiveItem` so tracked archive files move back through `git mv` |
| Fail closed on unexpected `git mv` errors | Return contextual errors instead of silently falling back after a selected `git mv` strategy fails; DB-failure rollback must restore both worktree files and Git index state |
| Decide staged-vs-unstaged behavior | Document that `git mv` stages the delete/add pair in Git, while content/frontmatter edits remain normal working-tree changes; tests assert the observable status |

## Implementation Units

### T1 - Git move strategy detection helper

* Changes: add a core helper that classifies a source path as `git mv`
  eligible or filesystem-only using bounded Git subprocesses
* Files: `internal/core/archive.go`, colocated unit test file
* Tests: table-driven unit tests for non-git workspace, git unavailable, git
  repo with untracked file, git repo with tracked file, timeout/canceled Git
  command, and same-path ineligibility
* Posture: test-first

### T2 - Git-aware archival path

* Changes: use the helper inside `ArchiveItem` after destination safety checks
  and before DB synchronization; keep pre-archived same-path records on the
  existing in-place update path
* Files: `internal/core/archive.go`, archive tests
* Tests: integration-style temp git repo test that commits the archived result
  before asserting `git log --follow` can see pre-archive history; regression
  tests for non-git filesystem moves and tracked same-path re-archival
* Posture: test-first with characterization of current fallback behavior

### T3 - Git-aware restore path

* Changes: apply the same move strategy in `UnarchiveItem`, including the
  canonical restore path and clobber-refusal guard; keep same-path self-heal or
  in-place updates away from `git mv path path`
* Files: `internal/core/archive.go`, archive restore tests
* Tests: tracked archive restore via `git mv`, untracked archive restore via
  filesystem move, and self-heal path remains contained
* Posture: test-first

### T4 - Error and staging semantics documentation

* Changes: document selected strategy, expected staged rename behavior, and
  fail-closed `git mv` error handling in comments or package-level test names
* Files: `internal/core/archive.go`, archive tests
* Tests: force a selected `git mv` failure and assert the error includes move
  context without falling back to filesystem rename; force a DB sync failure
  after `git mv` and assert both worktree and index return to the pre-call state
* Posture: test-first

## Dependency Graph

1. T1 must land before T2 or T3 because both use the strategy helper.
2. T2 and T3 can proceed independently after T1.
3. T4 depends on the final helper and move-call shape from T2/T3.

## Decisions and Rationale

* Use the Git CLI rather than a new Go dependency. The repository already uses
  Git operationally, and a dependency is unnecessary for two porcelain commands;
  every invocation must be bounded with `exec.CommandContext` and a small
  timeout to avoid indefinite archive or restore stalls.
* Treat git-unavailable, non-repository, and untracked-file states as fallback
  states, not hard errors, to preserve current behavior outside tracked repos.
* Treat `git mv` failure after strategy selection as fatal. Falling back after
  Git accepted eligibility would risk losing the history-preservation guarantee.
* Keep `git mv` staged behavior. `git mv` intentionally updates the index with
  a delete/add pair; Git history continuity is then inferred after the archive
  or restore result is committed. Commit discipline stays with the surrounding
  workflow.

## Risks and Caveats

* Git path handling differs on Windows. Mitigation: pass command arguments
  without shell quoting and derive repository-relative paths with `filepath.Rel`.
* The current archive path writes modified frontmatter before removing the
  source. Ship must extend existing rollback behavior so a post-`git mv` DB
  failure restores both filesystem content and index state.
* `git log --follow` assertions require a real temp Git repository and a commit
  after the archived move. Tests should skip only when `git` is genuinely
  unavailable, not when backlogit behavior is incorrect.

## Plan Hardening Signals

* Public API, schema, or contract change: absent. The CLI surface and artifact
  schema stay the same.
* Security, auth, permission, or compliance-sensitive behavior: absent. The work
  touches local file movement only.
* Migration, backfill, destructive data/config action, or irreversible step:
  absent. No existing artifacts are migrated by this task.
* External integration, operator checkpoint, or external dependency: present.
  The implementation invokes a local `git` binary when available, with explicit
  fallback and fail-closed behavior.
* High runtime, rollout, or rollback risk: absent. The changed runtime surface is
  archive and restore file movement, covered by temp workspace tests and existing
  rollback checks.

Requires plan hardening: no

## Runtime Verification and Closure

* Runtime surface: backlogit archive and restore flows for CLI and MCP callers,
  because both route through `internal/core/archive.go`
* Verification: `go test ./internal/core/...` must include git and non-git
  archive/restore cases; full `go test ./...`, `go vet ./...`, lint, and format
  remain Ship gates
* Closure: Ship should record that tracked backlog artifacts now use `git mv`
  and note the staged-rename behavior in the closure artifact; rollback is to
  revert the implementation commit and return to filesystem moves

## Plan Review

Gate decision: PASS.

Plan hardening requirement: satisfied. The plan names the only notable
hardening signal, a local Git CLI dependency, and bounds it with fallback states
plus fail-closed behavior for selected `git mv` failures. It does not change a
public schema, auth surface, migration, or rollout path.

Findings by severity:

* P0: none
* P1: none
* P2: none
* P3: none

Reviewer notes:

* Constitution review: the plan preserves Stage scope by staging backlog and
  plan artifacts only; Ship owns all Go implementation and TDD execution
* Go review: units are test-first, keep changes inside `internal/core`, and
  call out contextual error wrapping and Windows-safe process arguments
* Scope review: the plan maps directly to stash `44E2B442` and avoids unrelated
  archive lifecycle refactors
* Learnings review: the cited archived-from learning is relevant and reinforces
  fail-closed restore and clobber-refusal behavior
* Architecture review: keeping archive and restore strategy in core avoids CLI
  or MCP divergence because both surfaces share `internal/core/archive.go`

Runtime verification and closure are adequate for harvest: Ship must prove git
and non-git archive/restore behavior in tests and record the staged-rename
semantics in closure.
