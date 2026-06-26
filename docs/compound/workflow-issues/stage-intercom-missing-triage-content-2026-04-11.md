---
chunk_strategy: h1-h2-h3
description: ""
doc_type: learning
docline:
    category: workflow_issue
    component: cli
    date: 2026-04-11T00:00:00Z
    file_path: .github/agents/stage.agent.md
    message: Stage agent broadcasts lifecycle transitions but not triage decisions or shipment grouping content, leaving operator blind to substantive results
    problem_type: workflow_issue
    resolution_type: documentation
    resolved: true
    root_cause: missing_type_hint
    severity: high
    tags:
        - agent-intercom
        - stage
        - broadcast
        - operator-visibility
        - triage
        - shipment-grouping
        - gating
ingested_at: "2026-06-26T02:32:58Z"
schema_version: "1.0"
source: docs/compound/workflow-issues/stage-intercom-missing-triage-content-2026-04-11.md
title: Stage intercom broadcast protocol missing triage and shipment content
---

# Stage Intercom Broadcast Protocol Missing Triage and Shipment Content

## Problem

The Stage agent's intercom broadcast protocol defines messages for lifecycle
transitions (session start, deliberation handoff, plan written, review gate,
harvest complete) but has no entry for broadcasting the substantive content of
triage decisions or shipment candidate groupings. When Stage completes a triage
session and groups stash entries into shipment candidates, the operator receives
only a summary count ("4 stale entries removed, 13 entries grouped into 5
candidates") without the detail needed to make a selection decision.

## Symptoms

- Operator asked for full shipment candidate details because they had no
  visibility into what each candidate contained
- Triage analysis was produced in the chat response but not pushed to the
  intercom channel where the operator monitors progress
- The operator had to request a re-broadcast of information that should have
  been transmitted as part of the standard protocol
- Decision artifacts (which stash entries are stale, which group together, why)
  were only available in the chat transcript, not the broadcast channel

## What Did Not Work

The existing broadcast table in `stage.agent.md` covers only state transitions:

```text
| Triage start | broadcast | info | [STAGE] Reviewing stash entry: {stash_id} |
| Session complete | broadcast | success | [STAGE] Complete: stash triage finished |
```

There is no broadcast event between "reviewing entry" and "session complete"
that carries the triage outcome. The agent correctly followed the protocol but
the protocol itself was incomplete. The gap is structural, not behavioral.

## Solution

Add three broadcast events to the Stage agent's Remote Operator Integration
table that cover decision content, not just lifecycle transitions.

### Before

The broadcast table jumps from per-entry triage to deliberation handoff with no
intermediate content broadcast:

```text
| Triage start       | broadcast | info    | [STAGE] Reviewing stash entry: {stash_id}       |
| Deliberation handoff | broadcast | info  | [STAGE] Routing to deliberate skill: {stash_id} |
```

### After

Add these rows to the Stage agent's intercom broadcast table:

```text
| Stale entry removed    | broadcast | warning | [STAGE] Stale: {stash_id} — {reason}                                    |
| Shipment candidate     | broadcast | info    | [STAGE] 📦 SHIPMENT {letter}: "{name}" — {entry_count} entries: {details} |
| Triage recommendation  | broadcast | info    | [STAGE] 🔢 PRIORITY: {ordered_list}. Awaiting operator selection.         |
```

The `Shipment candidate` broadcast MUST include:

- Shipment letter and name
- Each stash entry with ID, priority, kind, and a one-line summary
- The rationale for grouping

The `Triage recommendation` broadcast MUST include the recommended priority
order with brief justification for the ordering.

## Why This Works

The root cause is a protocol design gap: the broadcast table was designed around
workflow state machine transitions (start → handoff → gate → complete) but not
around the information content that flows between those transitions. Triage
produces two types of output that the operator needs:

1. Elimination decisions (stale entries removed and why)
2. Grouping decisions (which entries form coherent shipment candidates and why)

Both are decision artifacts that the operator needs to act on. Without them in
the broadcast channel, the operator must either read the chat transcript or
ask for a re-broadcast, which breaks the async monitoring model that
agent-intercom enables.

The fix adds content-bearing broadcasts at the point where decisions are made,
following the same `[STAGE]` prefix convention. Using `warning` level for stale
removals ensures they stand out. Using `info` level for candidates and
recommendations keeps them in the normal flow.

## Prevention

- When defining agent broadcast tables, audit for both lifecycle events and
  decision content events. Every decision that requires operator action should
  have a corresponding broadcast.
- Apply the "could the operator act on this from the broadcast channel alone?"
  test to each protocol step. If the answer is no, the broadcast is missing
  substantive content.
- When a new workflow step produces output the operator needs to see (triage
  results, grouping recommendations, dependency analysis), add it to the
  broadcast table before shipping the agent update.
- Treat broadcast protocol gaps the same as missing API response fields: the
  consumer (operator) cannot function without them.

## Related Solutions

- `workflow-issues/shipment-ready-before-stage-gates-2026-04-10.md` — Stage
  gating enforcement gap where shipments were marked ready without completing
  deliberation, plan, and review gates
- `workflow-issues/pr-review-comment-reply-protocol-2026-04-10.md` — Similar
  pattern where local work existed but remote visibility was missing, requiring
  explicit protocol for bidirectional communication
- `workflow-issues/orphaned-tasks-without-parent-features-2026-04-10.md` —
  Agent instruction gap where guidance was incomplete, leading to structural
  errors
