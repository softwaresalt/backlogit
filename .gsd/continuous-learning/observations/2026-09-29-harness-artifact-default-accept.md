---
type: observation
observed_at: 2026-09-29T19:03:56-07:00
source: operator-directive
signal: harness-artifact-default-accept
---

# Harness Workflow Artifacts Are Default-Accepted

## Observation

Ship halted 154-S at the P-011 branch-creation gate because the worktree on
`main` held harness workflow artifacts: a line-ending-only
`.backlogit/stash.jsonl` modification, untracked `.backlogit/checkpoints/*.json`,
untracked `.backlogit/reconcile/*.md`, and an untracked `docs/memory/` note.

## Operator Rule

1. `.backlogit/stash.jsonl` changes are always accepted and committed. The stash
   is appended during every phase of work and is never a dirty-worktree blocker.
2. `docs/memory/**`, `.backlogit/memories.json`,
   `.backlogit/checkpoints/*.json`, `.backlogit/reconcile/*.md`, and
   `.autoharness/continuous-learning/observations/*.md` are natural outputs of
   the harness and its tooling. They are default-accepted.
3. The post-merge closure process owns these artifacts. Ship commits them into the
   post-merge closure chore branch commit by default. When leftovers remain at a
   later claim, Ship carries them forward into the next branch commit instead of
   halting.
4. These artifacts are never stashed, cleaned, or deleted.

## Promotion Candidate

Update the Ship P-011 branch-creation gate and the post-merge closure step to
treat these paths as a default-accepted allowlist. Unknown paths still halt.
