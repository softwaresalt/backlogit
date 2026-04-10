---
title: "PR Review Comment Reply Protocol: Always Reply After Fixing"
problem_type: workflow_issue
category: workflow_issue
component: cli
root_cause: incorrect_error_type
resolution_type: documentation
severity: medium
message: "After fixing PR review comments, always post a reply to each comment thread explaining the fix — a commit alone is not enough."
file_path: ".github/skills/fix-ci/SKILL.md"
resolved: true
tags: [pr-review, github, fix-ci, comment-reply, workflow, protocol, review-comments]
date: 2026-04-10
---

## Problem

When fixing GitHub Copilot (or human) review comments on a pull request, simply
pushing a fix commit is insufficient. Comment threads remain unresolved in the
GitHub UI, and the PR author cannot trace which commit addressed which comment
without digging through git history. This creates a poor review experience and
leaves the PR in an ambiguous state.

## Symptoms

* Copilot review comments show no responses in the PR thread
* PR author asks "have you worked the issue?" even though commits were pushed
* Comment threads remain unresolved (not marked resolved)
* No traceability between comment and fix commit from the reviewer's perspective

## What Did Not Work

* Pushing a fix commit that addresses the comments — the commit is visible in
  history but there is no connection between it and the specific comment thread
* Summarizing fixes in the conversation (local to the chat session) — this is
  visible to the operator but not to GitHub PR reviewers

## Solution

After pushing all fixes for review comments, iterate through every comment and
post a reply using the GitHub API. The reply should:

1. Reference the fix commit (short hash)
2. Explain what was changed and why
3. Be specific enough that a reviewer can verify without hunting through diffs

### API Command

```powershell
gh api repos/{owner}/{repo}/pulls/{pr_number}/comments/{comment_id}/replies `
    --method POST `
    -f body="Fixed in commit {sha}. {explanation of what was changed and why.}"
```

### Discovery: Get All Comment IDs

```powershell
$c = (gh api repos/{owner}/{repo}/pulls/{pr_number}/comments | ConvertFrom-Json)
$c | ForEach-Object { "$($_.id) $($_.path):$($_.line)" }
```

### Check Which Comments Already Have Replies

```powershell
$c | Where-Object { $_.in_reply_to_id -eq $null } | Select-Object id, path, line
```

Comments with `in_reply_to_id` populated are replies; comments without it are
top-level review comments needing a response.

### Example Reply Format

```text
Fixed in commit f942b2e. Updated the GoDoc comment on `VerifyPostShipConsistency`
from 'listing any stale queue paths found' to 'listing any stale artifact IDs
found' to match the implementation, which appends artifact IDs (not file paths)
to the error message.
```

## Why This Works

GitHub PR review comment threads are separate from commit history. A fix commit
closes the code gap, but the comment thread only shows as addressed when a reply
is posted. Posting a reply:

* Creates a direct link between the comment and the resolution
* Allows the reviewer to mark the thread as resolved
* Documents the rationale for each change in the thread, not just the diff

## Prevention

The fix-ci skill's Step 4b instructs to "resolve the comment thread if the
GitHub API supports it" — but this is insufficient guidance. Update the protocol
to **require a reply** with what was fixed before attempting to resolve.

Baked-in protocol rules to add to the agent harness:

* After applying fixes for review comments, always call the GitHub API to post
  a reply to each addressed comment thread before the session ends.
* Reply format: `"Fixed in commit {sha}. {what changed and why.}"`
* Never consider review comment remediation complete until all threads have
  replies and the fix commit is pushed with CI green.
* Add this step explicitly to the fix-ci skill Step 4b and to the
  ship.agent.md post-fix checklist.

## Related Solutions

* `docs/compound/workflow-issues/unstaged-mcp-tool-registrations-caused-ci-only-failure-2026-04-07.md`
  — another case where local state appeared resolved but remote/reviewer state was not
