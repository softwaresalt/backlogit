---
chunk_strategy: h1-h2-h3
description: Ship agent merged PRs without committing all locally-modified files, bypassing the PR review cycle for those changes.
doc_type: learning
docline:
    keywords:
        - ship agent
        - git staging
        - pr cycle
        - working tree
        - gofmt
        - line endings
    ms.date: 2026-04-14T00:00:00Z
    ms.topic: troubleshooting
ingested_at: "2026-06-26T02:32:58Z"
schema_version: "1.0"
source: docs/compound/workflow-issues/ship-agent-incomplete-git-staging-pr-bypass-2026-04-14.md
title: 'Ship Agent: Incomplete Git Staging Caused PR to Bypass Working-Tree Changes'
---

## Problem

During shipment 033-S (hooks system), the Ship agent opened and merged PR #36 and PR #37
to `main` without first verifying the working tree was clean. Approximately 44 files showed
as modified in `git status` after the merge, none of which had gone through code review.

The changes that escaped the PR included:

* `internal/db/gate.go` — small comment fix
* `internal/telemetry/harvest.go` — godoc blank line fix
* `internal/telemetry/harvest_since_test.go` — trailing newline fix
* `docs/exec-plans/2026-04-14-hooks-system-plan.md` — 1,344-line implementation plan (untracked)
* `docs/memory/*.md` — session memory artifacts (untracked)
* `.backlogit/` state changes from `backlogit_ship_shipment` running locally after merge
* `gofmt` reformatting needed on 28 source files (CRLF → LF normalization)

## Root Cause

The Ship agent staged changes using path-specific `git add` calls (e.g., `git add internal/hooks/
internal/core/`) rather than `git add -A`. Files modified outside those explicit paths were never
staged. Critically, the agent did not run `git status --porcelain` after staging to confirm the
working tree was clean before proceeding to PR creation.

The CI gates (`go test ./...`, `go vet ./...`, `golangci-lint run`) operate on on-disk files
regardless of what is staged. Passing CI gates locally does NOT prove all changes are committed —
it only proves the on-disk state is correct.

A secondary issue: `gofmt -l .` reported 28 source files as needing reformatting after the
PR merged. The Ship agent ran on Windows with `core.autocrlf=true`. Files that `gofmt` wrote
with LF endings were staged correctly, but the working directory checkout converted them back
to CRLF. Subsequent `gofmt -l .` runs reported them as unformatted. Adding a `.gitattributes`
entry (`*.go text eol=lf`) would resolve this permanently.

## Fix Applied

1. Created branch `feat/033-hooks-system-fix` from `main` after the errant merge.
2. Staged all missing changes with `git add -A` followed by `git rm --cached backlogit`
   (to exclude the build binary).
3. Ran `gofmt -w` on all 28 flagged files to normalize LF endings.
4. Verified all CI gates pass: `go test ./...`, `go vet ./...`, `golangci-lint run`, `gofmt -l .`.
5. Opened PR for review and put the missed changes through the proper review cycle.
6. Added `backlogit` (no-extension binary) to `.gitignore`.

## Required Harness Fixes

### 1. Clean Working Tree Gate Before PR Creation

The `pr-lifecycle` skill and the Ship agent's pre-PR checklist MUST include:

```bash
# Fail loudly if working tree is dirty
DIRTY=$(git status --porcelain)
if [ -n "$DIRTY" ]; then
  echo "ERROR: working tree is not clean. Stage all changes before opening a PR."
  git status --short
  exit 1
fi
```

This check MUST run AFTER all `go generate`, `gofmt -w`, and build steps, and BEFORE
`git push` / `gh pr create`.

### 2. Use `git add -A` (or Verified Path Coverage)

The Ship agent MUST use `git add -A` when staging all changes for a commit. Path-specific
`git add` is only safe when the explicit paths are verified to cover all modified files.
After any `git add`, run `git status --porcelain` and assert it returns no unstaged entries
for relevant paths.

### 3. Separate "Local Tool State" Changes from Code Changes

After running `backlogit_ship_shipment` or similar MCP tools, the `.backlogit/` directory
may acquire new state (archived artifacts, stash updates). These changes are legitimate and
should be committed on the fix branch, but they will NOT appear in the original PR since
they happen post-merge. This is expected behavior. Document this distinction to avoid
confusion during post-merge cleanup.

### 4. Fix `gofmt` Line-Ending False Positives on Windows

Add a `.gitattributes` file with:

```gitattributes
*.go text eol=lf
```

This ensures Go source files always use LF line endings in the repository, eliminating
false `gofmt -l .` failures on Windows checkouts where `core.autocrlf=true`.

## Prevention Checklist for Ship Agent

Before `git push` and PR creation, the Ship agent MUST verify:

- [ ] `git status --porcelain` returns empty output (or explicitly accounts for expected dirty files)
- [ ] `git add -A` was used (not path-specific adds) OR all modified paths are verified complete
- [ ] `gofmt -l .` returns no output
- [ ] `go test ./...` passes
- [ ] `go vet ./...` passes
- [ ] `golangci-lint run` passes

These gates work on the staged/committed state. CI gates passing locally without a clean working
tree is a false positive — always confirm staging completeness independently.

## Signals That This Mistake Occurred

* After a PR merges, `git status` on `main` shows modified or untracked files in `internal/`,
  `docs/`, or `tests/` directories.
* `gofmt -l .` on `main` reports files needing reformatting immediately after merge.
* Session memory or execution plan documents that were authored during the Ship session appear
  as untracked files post-merge.

## Related

* Shipment 033-S: hooks system two-layer implementation
* PR #36: original implementation (committed changes only)
* PR #38 / `feat/033-hooks-system-fix`: remediation branch for missed changes
* `docs/compound/workflow-issues/unstaged-mcp-tool-registrations-caused-ci-only-failure-2026-04-07.md` — related: a prior instance of incomplete staging causing CI issues
