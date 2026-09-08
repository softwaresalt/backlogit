---
name: pr-lifecycle
description: "Manages the full PR lifecycle: creation, Copilot comment handling, CI remediation, and user-approved merge"
argument-hint: "branch=015-feature-branch"
input:
  properties:
    branch:
      type: string
      description: "Feature branch to create PR for"
    title:
      type: string
      description: "PR title (optional, defaults to branch name)"
  required:
    - branch
---

# PR Lifecycle Skill

Manage the branch-to-merged workflow for a shipment or feature branch. This
skill creates or updates the pull request, responds to review feedback, keeps CI
healthy, and stops at the user merge gate unless the user explicitly approves
the merge.

## Purpose

Use this skill when implementation work is ready to move through pull request
execution. It centralizes the PR control loop so higher-level agents can treat
review, CI follow-up, and merge approval as one bounded workflow.

## Inputs

* `${input:branch}`: (Required) Branch name to ship
* `${input:title}`: (Optional) PR title override. When omitted, derive the
  title from the branch name or prepared PR description

## Workflow

### Step 1: Prepare the branch

1. Confirm the branch exists locally and is ready to push.
2. Gather or generate the PR title and body before calling GitHub tooling.
3. Push the branch if it is not already available on the remote.

### Step 2: Create or update the pull request

1. Use the GitHub CLI, such as `gh pr create` or `gh pr edit`, to create or
   refresh the pull request.
2. Capture the PR URL, branch, and base branch as the active shipment context.
3. Reuse an existing PR when the branch already has one instead of creating
   duplicates.

### Step 3: Handle Copilot and review feedback

1. Monitor PR review comments, especially Copilot review comments.
2. Apply bounded fixes directly when they are clearly actionable.
3. Re-run the relevant validation after each fix cycle.
4. Keep the PR description and review context aligned with the latest branch
   state.

### Step 4: Handle CI failures

1. If CI fails, invoke the `fix-ci` skill with the active PR or branch context.
2. Let `fix-ci` own the remediation loop for failing checks and unresolved
   review comments.
3. Return to PR monitoring once CI and review status are clean again.

### Step 5: Merge approval gate

1. When the PR is reviewable and checks are green, present the status to the
   user.
2. Wait for explicit user approval before any merge action.
3. Never auto-merge and never treat silence as approval.
4. If the user does not approve merge, leave the PR open and report the ready
   state.
5. **Branch retention (NON-NEGOTIABLE)**: Do NOT checkout `main` or delete the feature branch
   while the merge gate is open. The branch is the active working context for
   any follow-up CI or review fixes.

### Step 6: Post-merge synchronization and cleanup

After a user-approved merge that reached `MERGE_SUCCEEDED`:

1. Report the merge result and resulting default-branch state.
2. **Return the existing worktree to synchronized `main` (NON-NEGOTIABLE).**
   This runs immediately after `MERGE_SUCCEEDED`, before any post-merge closure
   branch or session completion, so the operator never has to ask for it. The
   invariant is
   `MERGE_SUCCEEDED -> safe switch main -> ff-only sync -> SHA equality verification -> optional post-merge branch`.
   Run each step sequentially; never stash, reset, rebase, or discard to force
   it:
   a. Inspect the working tree (`git status --porcelain`) and preserve any
      unrelated tracked or untracked local state.
   b. Fetch the default branch (`git fetch origin main`).
   c. Verify switching branches will not overwrite local modifications. If it
      would, fail closed with a BLOCKED result — never auto-stash or discard.
   d. Switch the existing worktree to local `main` (`git checkout main`).
   e. Fast-forward only (`git pull --ff-only origin main`).
   f. Verify `HEAD == origin/main` and record the synchronized SHA.
   g. Confirm the unrelated local state from step 2a is still present and
      unstaged.
   h. Only after this may a `post-merge/{feature_slug}` closure branch be
      created from synchronized `main`. If no closure branch is needed, end on
      synchronized local `main`.
3. If the safe switch or fast-forward-only update cannot complete, treat
   post-merge cleanup as BLOCKED and surface it. Do not report the merge
   workflow as fully complete.
4. Delete the feature branch only when that cleanup is explicitly requested or
   already part of the chosen PR flow. Do NOT delete automatically.
5. Summarize any follow-up items, release notes, or residual risks that remain
   after merge.

## Completion Criteria

The skill is complete only when one of these outcomes is explicit:

* the PR is open and ready, waiting on user merge approval
* the PR feedback and CI loop is blocked with a clear reason
* the PR was merged after explicit user approval
