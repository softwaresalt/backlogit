---
chunk_strategy: h1-h2-h3
schema_version: "1.0"
title: "Carry forward continuity files into the next staging round"
description: "Preserve recurring backlog, memory, launcher, and ignore-file changes on local main and include them in the next Stage-owned staging change."
doc_type: learning
source: docs/compound/workflow-issues/carry-forward-continuity-files-into-staging-2026-09-08.md
docline:
    problem_type: workflow_issue
    category: workflow-issues
    component: stage-ship-handoff
    root_cause: expected continuity artifacts were left on local main but treated as unrelated dirty-worktree contamination
    resolution_type: design_change
    severity: medium
    message: "Ship branch creation blocked because local main contains recurring continuity-file changes"
    file_path: .backlogit/stash.jsonl
    citations:
        - .github/agents/_stage.agent.md
        - .github/agents/_ship.agent.md
        - docs/compound/workflow-issues/ship-agent-incomplete-git-staging-pr-bypass-2026-04-14.md
        - .backlogit/queue/139-S.md
    tags:
        - stage
        - ship
        - dirty-worktree
        - session-continuity
        - staging
        - memory
---

## Problem

A new pipeline session can start on local `main` with legitimate changes left
by earlier workflow activity. The recurring paths are:

* `.backlogit/stash.jsonl`
* `docs/memory/**/*.md`
* `start.ps1`
* `.gitignore`

Ship correctly refuses to create a shipment branch from a dirty default branch.
However, treating these recurring files as arbitrary contamination causes the
same halt in each new session. Discarding or automatically stashing them would
lose backlog intake, session continuity, launcher maintenance, or repository
ignore rules.

This occurred when dark-factory execution of shipment `139-S` reached the Ship
branch-creation gate. The shipment itself was ready, but the known continuity
files on local `main` prevented the handoff.

## Root Cause

The Stage-to-Ship handoff assumed that every staging round begins from a clean
local `main`. In practice, some workflow-owned files are intentionally produced
between rounds and remain uncommitted:

* stash changes preserve newly captured or deferred work
* memory documents preserve session and closure context
* `start.ps1` carries local workflow-launcher maintenance
* `.gitignore` carries repository hygiene discovered during execution

These files belong to the next staging round. They are not Ship implementation
scope, but they also must not be discarded as unrelated local state. Without an
explicit carry-forward rule, Stage leaves them behind and Ship repeatedly
encounters the same dirty-worktree gate.

## Resolution

Always carry these known continuity-file changes into the next staging round:

1. Inspect the existing changes and confirm they are legitimate workflow,
   continuity, launcher, or repository-hygiene updates.
2. Preserve the files exactly as found. Do not reset, discard, or automatically
   stash them to make Ship's branch gate pass.
3. Have Stage validate and commit the backlog, planning, learning, and memory
   artifacts that fall within its role boundary.
4. Have the Orchestrator's staging-artifact merge gate carry any validated
   launcher or repository-hygiene changes through the staging branch and pull
   request. Stage must not create the pull request or commit files outside its
   role boundary.
5. Merge the staging pull request to `main` before invoking Ship.
6. Confirm local `main` is synchronized and clean, then hand the queued shipment
   to Ship.

This rule carries forward the listed file classes. It does not convert every
unknown dirty file into staging scope. Unexpected source-code, generated-binary,
secret-bearing, or unrelated operator changes still require provenance review
and must fail closed when their ownership is unclear.

## Prevention

* Stage session intake should validate known carry-forward paths and commit only
  the backlog, planning, learning, and memory artifacts its role permits
* The Orchestrator's staging-artifact merge gate should include legitimate
  pending changes to `.backlogit/stash.jsonl`, `docs/memory/`, `start.ps1`, and
  `.gitignore` without assigning out-of-bound mutations to Stage
* Until Orchestrator Step 1.5 formally checks every carry-forward path, treat
  this learning as the explicit path allowlist and record that continuity
  staging is required in the dark-mode activation before claiming a shipment
* Ship should continue enforcing a clean-default-branch gate; Stage owns making
  the expected continuity state durable before the Ship handoff
* Never use reset, checkout, clean, or automatic stash operations to hide these
  changes
* Review unknown dirty paths separately instead of broadening this allowlist
  implicitly
