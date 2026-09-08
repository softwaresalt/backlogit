---
description: "Required protocol for Git merge, rebase, and rebase --onto workflows with conflict handling and stop controls"
applyTo: '**'
---

# Git Merge and Rebase Instructions

## Required Protocol

### 1. Prepare the Workspace

* Confirm the working tree is clean with `git status --short`
* Stash local changes before proceeding **only when preparing a new merge/rebase
  operation**. This preparation step does NOT apply after a merge — post-merge
  synchronization is governed by the Post-Merge Local Main Synchronization rule
  below and never stashes, resets, or discards.
* Fetch latest remote refs

### 2. Select the Operation Path

* For merge: `git merge --no-edit {branch}`
* For rebase: `git rebase --empty=drop --reapply-cherry-picks {branch}`
* For rebase-onto: `git rebase --onto {onto} {upstream} {branch}`

### 3. Execute the Operation

* Run the planned Git command and capture output
* When Git reports conflicts, highlight the files listed by `git status --short`
* If no conflicts, jump to Step 6

### 4. Resolve Conflicts

* Inspect each conflicted file individually
* Apply focused edits to resolve markers, then stage changes
* After fixes, describe the rationale

### 5. Honor Review Pauses

* Pause after summarizing conflict fixes when requested
* Await explicit user confirmation before continuing

### 6. Continue or Complete

* Resume with `git merge --continue`, `git rebase --continue`, or abort if needed
* Confirm a clean tree after completion

### 7. Summarize Results

* Provide a summary of the operation, conflicts, and resolutions
* Remind the user that this local merge/rebase conflict-resolution operation
  performed no pushes

## Guardrails

These guardrails govern the **local merge/rebase conflict-resolution operation**
described in Steps 1–7. They do not forbid the normal, non-force pushes that
downstream merge-capable workflows legitimately perform (for example, Ship
pushing a `post-merge/` closure branch, or any workflow pushing a feature branch
before opening its PR). Force-pushing and history rewrites remain forbidden
everywhere.

* Never push, force-push, or rewrite remote history **as part of this local
  merge/rebase conflict-resolution operation**. (Force-push and history rewrite
  stay forbidden in all workflows; a normal push by a downstream workflow that
  owns that step is allowed.)
* Do not proceed with unrelated staged changes
* Document every conflict fix with justification
* When unsure, consult official Git documentation

## Merge Strategy Policy (NON-NEGOTIABLE)

All pull request merges MUST use merge commits. Squash merge and rebase merge are
expressly forbidden (Constitution Principle XI, P-009).

**Required repository settings** (GitHub):
1. Navigate to Settings → General → Pull Requests
2. Ensure **"Allow squash merging"** is **unchecked**
3. Ensure **"Allow rebase merging"** is **unchecked**
4. Ensure **"Allow merge commits"** is **checked**

**Rationale**: Merge commits preserve the full development history, individual commit
attribution, and bisect-friendly history. Squash merge destroys commit granularity.
Rebase merge rewrites history, breaking backlog commit-traceability links.

**Ship agent enforcement**: Before executing any merge, verify the merge strategy
is `merge commit`. If squash or rebase merge is detected, halt and report a P-009
violation. Do not proceed until the operator corrects the repository settings.

## Post-Merge Local Main Synchronization (NON-NEGOTIABLE)

This is the authoritative, cross-workflow rule for what happens after *any*
successful merge into the default branch `main`. It applies to **every** harness
workflow, agent, and skill that performs or confirms a merge to `main`,
**regardless of role or PR class** — Ship, PR lifecycle, staging/planning merges,
corrective PRs, closure PRs, elective/harness PRs, and any future merge-capable
workflow. Ship and the pr-lifecycle skill *operationalize* this rule; they do not
replace, narrow, or contradict it.

**Global invariant:**

```text
ANY MERGE_SUCCEEDED to main -> safe local main switch -> ff-only origin/main sync -> SHA equality -> next steps
```

### Trigger — MERGE_SUCCEEDED only

This sequence runs **only after a confirmed `MERGE_SUCCEEDED`** — a merge that
actually landed on `main`. A merge failure, a merely approved-but-unmerged PR, or
an aborted/blocked merge MUST NOT trigger it. Confirm the merge landed (for
example `git merge-base --is-ancestor <feature> origin/main`) before starting.

### Ordered sequence — run before any next step

After `MERGE_SUCCEEDED`, and **before any next step** — including creating a
post-merge closure/follow-up branch, selecting the next queue item, or ending the
workflow — run these steps in order. Never stash, reset, rebase, discard,
`git clean`, or force-push to force the switch:

1. Record and preserve unrelated local tracked and untracked state
   (`git status --porcelain`).
2. Fetch the merged remote tip into the `origin/main` tracking ref that step 6
   compares against — use an explicit destination refspec so
   `refs/remotes/origin/main` is updated, not only `FETCH_HEAD`
   (`git fetch origin main:refs/remotes/origin/main`).
3. Prove the switch will not overwrite local modifications. If it would, fail
   closed — never auto-stash, reset, or discard.
4. Switch the existing worktree to local `main` (`git checkout main`). Advance the
   existing worktree only — never create a second worktree (P-016).
5. Fast-forward only: `git pull --ff-only origin main` (or the exact safe
   equivalent). Never a merge, rebase, or forced update.
6. Verify local `main` equals the remote tip — `HEAD == origin/main` — and record
   the synchronized SHA.
7. Verify the unrelated local state recorded in step 1 is still present and
   unchanged — the same tracked, untracked, and index (staged) status captured in
   step 1. The sync MUST NOT have staged, modified, or dropped it; a pre-existing
   staged entry must remain staged, an unstaged one must remain unstaged.
8. Only then proceed to any next step.

### Fail-closed blocked state

If any step cannot complete safely, halt with `POST_MERGE_SYNC_BLOCKED` and a
concrete reason. A workflow in `POST_MERGE_SYNC_BLOCKED` MUST NOT claim overall
completion and MUST NOT move on to next work until the operator resolves it. Do
not stash, reset, rebase, discard, or force-push to clear the block automatically.

### Preserved rules

* Precedence over pre-merge prep: the general "Stash local changes before
  proceeding" step (Required Protocol §1) applies only to preparing a new
  merge/rebase operation. It never overrides this section — post-merge
  synchronization never stashes, resets, or discards.
* No automatic source-branch deletion — branch deletion stays explicit (see
  Guardrails and the merge-commit-only policy below).
* P-009 (merge-commit-only) and P-016 (single-worktree) are preserved: the ff-only
  pull advances the existing worktree; it never rewrites history or spawns a
  parallel worktree.

Generated by autoharness | Template: git-merge.instructions.md.tmpl
