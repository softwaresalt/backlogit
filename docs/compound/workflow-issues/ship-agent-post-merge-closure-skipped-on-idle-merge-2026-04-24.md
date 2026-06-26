---
chunk_strategy: h1-h2-h3
description: Ship agent post-merge closure silently skipped when PR merged via GitHub UI while Ship agent was idle; ship_shipment fires but the 6-step closure workflow has no execution context to run.
doc_type: learning
docline:
    component: task_manager
    date: 2026-04-24T00:00:00Z
    file_path: .github/agents/ship.agent.md
    message: 'Ship agent post-merge closure (6 steps) never executed after shipment 044-S was archived. PR #66 merged via GitHub UI while Ship agent was idle; ship_shipment fired but closure protocol had no execution context.'
    problem_type: workflow_issue
    resolution_type: documentation
    resolved: true
    root_cause: timeout
    severity: medium
    tags:
        - ship-agent
        - post-merge-closure
        - session-continuity
        - shipment-lifecycle
        - idle-merge
        - hook-gap
ingested_at: "2026-06-26T02:32:58Z"
schema_version: "1.0"
source: docs/compound/workflow-issues/ship-agent-post-merge-closure-skipped-on-idle-merge-2026-04-24.md
title: Post-Merge Closure Silently Skipped When PR Merged Outside Active Ship Session
---

## Problem

The Ship agent's mandatory post-merge closure protocol only executes when the agent is **actively running** at the moment the user approves the merge. When a PR is merged via GitHub UI while the Ship agent is idle, `ship_shipment` fires and archives the shipment, but the six-step closure workflow never initiates — no execution context is present to run it.

## Symptoms

- `docs/closure/` has no closure artifact for the shipped shipment
- No memory file in `docs/memory/` for the shipment
- Source stash entry (e.g., `F51BAEC0`) remains unremoved
- Source deliberation artifact still in `archived` state without confirmed cleanup
- No `[SHIP] Post-merge closure complete` broadcast in the session log
- Hook event `ship_shipment` (seq 240) recorded at 07:31 — but no subsequent closure events
- Merged PR commit visible on `origin/main`; local branch still checked out on the feature branch

## What Did Not Work

- **Relying on `ship_shipment` alone**: The state transition archives the shipment artifact but does not embed or trigger the six-step closure workflow. The two are decoupled by design.
- **Session Start Recovery Protocol**: The protocol looks for `active` checkpoints. If the previous Ship session ended cleanly after calling `ship_shipment` (or a minimal session ran only that call), no closure-pending checkpoint is written, so recovery on the next session start does not detect the gap.
- **Hook event polling**: The `ship_shipment` hook event does not carry a `post_merge_closure` signal payload, so a new Ship session has no programmatic way to discover that closure is pending.

## Solution

### Immediate — Operator Re-entry Protocol

When reconnecting to a shipped or recently-merged branch, the Ship agent must perform a closure audit before doing anything else:

1. Check `docs/closure/` for a closure artifact matching the shipment.
2. Check `docs/memory/` for a final memory file for the shipment.
3. If either is missing, **run the full post-merge closure protocol now**:
   - Invoke `operational-closure` skill in `mode=post-merge`
   - Remove the source stash entry (`backlogit_stash_remove`)
   - Archive the source deliberation (`backlogit_archive_item`)
   - Evaluate and update product documentation
   - Write a memory file to `docs/memory/`
   - Broadcast `[SHIP] Post-merge closure complete`

### Future Enhancement — Closure Checkpoint at `ship_shipment`

When `ship_shipment` is called, write a V1 checkpoint with `phase: "closure-pending"` and `status: "active"` before the state transition completes. This ensures the next Ship session's Recovery Protocol finds the pending closure and re-enters it automatically, even without an explicit operator prompt.

```json
{
  "schema_version": 1,
  "agent": "ship",
  "phase": "closure-pending",
  "status": "active",
  "context": {
    "shipment_id": "044-S",
    "merge_sha": "71e392a6dc0f99a74e1b1c695251404014a56c7d"
  },
  "resume_hint": "Post-merge closure not yet executed. Run operational-closure, archive source artifacts, update docs, write memory."
}
```

## Why This Works

`ship_shipment` is a **terminal state mutation** — it marks the shipment artifact as archived and records the merge SHA. Post-merge closure is a **distinct multi-step workflow** (artifact archival, documentation review, context compaction, memory persistence). These are intentionally separate: closure cannot be embedded in `ship_shipment` because it calls external skills (`operational-closure`, `compact-context`) that require a running agent context.

The gap emerges when the **execution context disappears between the state mutation and the closure workflow**. A persistent `closure-pending` checkpoint bridges the gap by making the outstanding closure obligation discoverable to the next session.

## Prevention

1. **Closure audit at session start**: Every Ship session resuming on a feature branch with an `archived` shipment must check for a closure artifact before claiming new work. Add this audit to the Session Start Recovery Protocol.

2. **Write a `closure-pending` checkpoint before `ship_shipment`**: Ensures recovery finds the gap even if the merge happened outside an active session. Resolve the checkpoint only after `[SHIP] Post-merge closure complete` is broadcast.

3. **Treat closure as part of the shipment state machine**: Do not consider a shipment fully terminal until the closure broadcast is recorded. The shipped state is `archived`; add a logical `closed` step (even if not a separate status value) that confirms closure is done.

4. **Operator checklist on branch re-entry**: When the user or agent notices the working tree is still on a feature branch after a merge (local branch not switched to `main`), treat this as a signal to check whether post-merge closure ran.

## Related Solutions

- [`source-artifact-archival-pattern-2026-04-20.md`](../workflow-issues/source-artifact-archival-pattern-2026-04-20.md) — Post-merge closure must archive source stash entries and deliberations; covers the archival sub-steps in detail.
- [`ship-agent-unrealized-follow-up-stash-2026-04-12.md`](../workflow-issues/ship-agent-unrealized-follow-up-stash-2026-04-12.md) — Ship agent closure violations where follow-ups were reported without tool invocation.
- [`shipment-ready-before-stage-gates-2026-04-10.md`](../workflow-issues/shipment-ready-before-stage-gates-2026-04-10.md) — Shipment lifecycle gaps from premature state transitions.
