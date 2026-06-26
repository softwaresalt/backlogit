---
chunk_strategy: h1-h2-h3
description: ""
doc_type: learning
docline:
    category: workflow_issue
    component: cli
    date: 2026-04-23T00:00:00Z
    file_path: .github/skills/pr-lifecycle/SKILL.md
    message: branch-protection conversation-resolution rule is not bypassable by --admin; use resolveReviewThread GraphQL mutation to bulk-resolve threads before merge
    problem_type: workflow_issue
    resolution_type: documentation
    resolved: true
    root_cause: incorrect_error_type
    severity: medium
    tags:
        - github
        - graphql
        - pr-review
        - conversation-resolution
        - branch-protection
        - merge-blocked
        - copilot-review
        - resolveReviewThread
        - gh-cli
ingested_at: "2026-06-26T02:32:58Z"
schema_version: "1.0"
source: docs/compound/workflow-issues/github-pr-bulk-resolve-review-threads-graphql-2026-04-23.md
title: 'GitHub PR Review Threads: Bulk Resolve via GraphQL When Branch Protection Blocks Merge'
---

# GitHub PR Review Threads: Bulk Resolve via GraphQL When Branch Protection Blocks Merge

## Problem

When the GitHub branch protection rule "Require conversation resolution before
merging" is enabled, every review thread must be `isResolved: true` before merge
can proceed. Pushing fix commits does NOT auto-resolve threads. The `gh pr merge
--admin` flag bypasses most protections but **NOT** the conversation resolution
requirement.

## Symptoms

- Merge command fails: `GraphQL: Repository rule violations found — A conversation
  must be resolved before this pull request can be merged.`
- `gh pr merge <number> --merge --admin` returns the same error.
- Review threads remain `isResolved: false` in the GitHub UI even after pushing
  commits that address the code feedback.

## What Did Not Work

- `gh pr merge 60 --merge --admin` — rejected; `--admin` does not override
  conversation resolution.
- Pushing additional fix commits — threads stay unresolved regardless of commit
  content.

## Solution

**Step 1 — Query all thread IDs**

```bash
gh api graphql -f query='query {
  repository(owner:"OWNER", name:"REPO") {
    pullRequest(number:60) {
      reviewThreads(first:30) {
        nodes { id isResolved }
      }
    }
  }
}'
```

**Step 2 — Resolve each thread via mutation**

Single thread:

```bash
gh api graphql -f query='mutation {
  resolveReviewThread(input:{threadId:"PRRT_kwDO..."}) {
    thread { id isResolved }
  }
}'
```

Bulk resolution in PowerShell:

```powershell
$threads = @(
    "PRRT_kwDOAB_id1",
    "PRRT_kwDOAB_id2"
)
foreach ($id in $threads) {
    gh api graphql -f query="mutation { resolveReviewThread(input:{threadId:`"$id`"}) { thread { id isResolved } } }"
}
```

**Step 3 — Merge**

```bash
gh pr merge 60 --merge
```

## Why This Works

The branch protection rule checks the `isResolved` field on each `ReviewThread`
node in the GitHub data model. The `resolveReviewThread` mutation sets that field
directly. This is a data state, not a permission gate — `--admin` has no authority
to change it because there is nothing to override; the data simply must be in the
right state.

## Prevention

- After pushing a fix for each Copilot review comment, immediately resolve the
  corresponding thread rather than batching all resolutions at merge time.
- When many threads accumulate (e.g., after 2+ Copilot review passes), use the
  bulk PowerShell loop above rather than resolving threads one-by-one in the UI.
- See [pr-review-comment-reply-protocol-2026-04-10.md](pr-review-comment-reply-protocol-2026-04-10.md)
  for the complementary pattern: always reply to a thread with the fix summary
  before resolving it.

## Related Solutions

- [pr-review-comment-reply-protocol-2026-04-10.md](pr-review-comment-reply-protocol-2026-04-10.md) —
  reply to each thread after pushing a fix; this doc covers how to resolve the
  thread after replying.
- [ship-agent-incomplete-git-staging-pr-bypass-2026-04-14.md](../workflow-issues/ship-agent-incomplete-git-staging-pr-bypass-2026-04-14.md) —
  related PR workflow gotcha about incomplete staging bypassing the PR cycle.
