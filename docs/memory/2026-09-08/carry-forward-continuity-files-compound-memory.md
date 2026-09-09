---
title: "Carry-forward continuity files compound learning"
date: 2026-09-08
---

## Outcome

Captured a workflow learning that recurring changes to
`.backlogit/stash.jsonl`, `docs/memory/`, `start.ps1`, and `.gitignore` must be
preserved and included in the next Stage-owned staging round.

## Files modified

* `docs/compound/workflow-issues/carry-forward-continuity-files-into-staging-2026-09-08.md`
* `docs/memory/2026-09-08/carry-forward-continuity-files-compound-memory.md`

## Decisions

* Known continuity-file changes are not disposable dirty-worktree noise
* Stage validates and commits only the backlog, planning, learning, and memory
  artifacts within its role boundary; Orchestrator Step 1.5 has a narrow
  carve-out to commit and carry the four allowlisted continuity paths through a
  staging branch and pull request
* Ship retains its clean-default-branch gate
* Unknown or secret-bearing changes remain fail-closed and are not implicitly
  included by this rule

## Next step

Apply this learning during the next Stage round before resuming shipment
`139-S`.
