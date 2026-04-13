---
title: "Follow-up Items Must Be Stashed, Not Just Reported"
problem_type: workflow_issue
category: workflow_issue
component: task_manager
root_cause: missing_test_fixture
resolution_type: documentation
severity: high
message: "Agent closure summaries that say 'Follow-up filed' without calling backlogit_stash create silent data loss — no stash entry, no follow-up."
file_path: "internal/mcp/tools.go"
resolved: true
tags: [stash, ship-cycle, follow-up, agent-discipline, closure, backlogit_stash]
date: 2026-04-12
---

## Follow-up Items Must Be Stashed, Not Just Reported

## Problem

When an agent discovers a follow-up item during a ship cycle or any workflow phase, it must actually call `backlogit_stash` to persist it. During shipment 013-S, the Ship agent reported "Follow-up filed" in its closure summary but never invoked the tool. The stash was empty and the follow-up was silently lost.

## Symptoms

- Closure summary states "Follow-up filed" or "Stashed for follow-up" but `backlogit_fetch_stash` returns no matching entry.
- Follow-up items discovered mid-workflow disappear after the session ends.
- Future Stage agents find no queued work even though prior summaries reference filed follow-ups.
- Stash entry IDs cited in closure docs cannot be verified.

## What Did Not Work

- **Trusting agent-reported "filed" status without verification.** Prose that claims "Follow-up filed" is not proof — only a stash entry is.
- **Relying on closure artifact prose to preserve actionable follow-ups.** Closure documents are human-readable summaries, not machine-executable queues. They do not appear in Stage pipelines or `backlogit_fetch_stash` results.
- **Assuming tool calls happened because they were described.** Agents can describe actions in natural language without actually invoking the underlying tools.

## Solution

After any workflow phase that identifies a follow-up item, the agent MUST call `backlogit_stash` before reporting completion. The stash entry ID returned by the tool is proof the action occurred.

### Before (hallucinated)

```text
Closure Summary:
  "Follow-up filed: When AdoptItem rewrites IDs, cross-artifact
   frontmatter references need to be rewritten too."

[No backlogit_stash call was made. No stash entry exists.]
```

### After (correct)

```text
# During shipment closure, before marking shipment done:

backlogit_stash(
  kind="task",
  priority="medium",
  text="When AdoptItem rewrites an artifact's ID, other artifacts whose
        frontmatter references the old ID are not updated. Stale edges
        reappear on next rehydration. Need cross-artifact reference rewrite
        in AdoptItem. Discovered during shipment 013-S."
)
# → returns { id: "C00AA592", ... }

# NOW report completion with proof:
Closure Summary:
  "Follow-up stashed (C00AA592): AdoptItem cross-artifact
   frontmatter reference rewrite."
```

## Why This Works

The stash is the system of record for deferred work. Only stash entries surface in `backlogit_fetch_stash` results and Stage agent queue views. Prose in summaries and closure docs is never consumed by automated pipelines. The stash entry ID is verifiable evidence that the action executed.

## Prevention

- Before marking any shipment `done`, include an explicit "stash all discovered follow-ups" step that calls `backlogit_stash` for each one and captures the returned entry ID.
- Include the stash entry ID in the closure summary — it is proof, not decoration.
- After reporting "follow-up stashed," immediately verify with `backlogit_fetch_stash` that the entry exists before proceeding.
- Closure artifact templates should include a `Follow-ups stashed` section with ID and description for each entry.

**Ship agent checklist addition:**

```text
[ ] All discovered follow-ups have a corresponding backlogit_stash call
[ ] Stash IDs are included in the closure artifact
[ ] backlogit_fetch_stash confirms all IDs exist
```

## Related Solutions

- [`docs/compound/workflow-issues/stash-staleness-requires-custom-scripting-2026-04-09.md`](../workflow-issues/stash-staleness-requires-custom-scripting-2026-04-09.md) — stash lifecycle gaps and deletion tooling
- [`docs/compound/workflow-issues/shipment-ready-before-stage-gates-2026-04-10.md`](../workflow-issues/shipment-ready-before-stage-gates-2026-04-10.md) — ship cycle workflow discipline and gate validation
- [`docs/compound/workflow-issues/stable-contract-before-two-agent-adoption-2026-04-05.md`](../workflow-issues/stable-contract-before-two-agent-adoption-2026-04-05.md) — two-agent workflow boundaries and role separation
