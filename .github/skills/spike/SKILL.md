---
name: spike
description: "Time-boxed investigation of a technical question or feasibility concern, producing a findings artifact and optional planning handoff"
argument-hint: "goal=... [time_box={1h|2h|4h|8h}] [promote_to={plan|queue|learnings|none|ask}]"
---

# Spike

Investigate an unknown before the team commits to implementation. A spike produces findings, not production code.

## When to use

Use this skill when you need evidence for a technical question, migration risk, prototype path, performance concern, or external dependency choice.

Do not use it for feature decisions that can be resolved through conversation alone. Use `deliberate` for those.

## Inputs

* `goal`: Required, the specific question or hypothesis to investigate.
* `time_box`: Optional, defaults to `4h`. Accepted values are `1h`, `2h`, `4h`, and `8h`.
* `scope_constraints`: Optional boundary such as read-only, sandbox-only, or branch-only.
* `linked_feature`: Optional feature ID or path this spike informs.
* `promote_to`: Optional destination: `plan`, `queue`, `learnings`, `none`, or `ask`.

## Output

Write the findings artifact to `docs/decisions/{YYYY-MM-DD}-{slug}-spike.md`.

In this workspace, queue promotion should append or update a stash entry in `.backlogit/queue/.stash.md`. Backlogit currently provides first-class deliberation artifacts here, but not a first-class `spike` artifact type in the live workspace configuration.

## Required protocol

### Phase 1: Scope the investigation

1. Restate the goal as a precise question.
2. Define success criteria.
3. Record out-of-scope areas.
4. Confirm the time box and any scope constraints.

### Phase 2: Check prior work

Before investigating, search:

* `docs/compound/` for prior learnings
* `docs/decisions/` for earlier spike or decision artifacts
* `.backlogit/queue/.stash.md` and related backlog items for linked work

### Phase 3: Investigate

1. Outline a short investigation approach.
2. Read the relevant code and configuration.
3. Prototype, benchmark, or research only as needed to answer the question.
4. Record evidence as you go.
5. Stop exploring new branches once the time box is effectively spent.

### Phase 4: Synthesize the result

Return one conclusion:

* `proceed`
* `pivot`
* `defer`
* `abandon`

Also rate confidence as `high`, `medium`, or `low`.

### Phase 5: Route the outcome

When `promote_to` includes `plan`, invoke `impl-plan` with the spike artifact as source context. If the operator wants backlog creation after review, route the resulting plan through `harvest` or the Groomer workflow.

When `promote_to` includes `queue`, record the follow-up in `.backlogit/queue/.stash.md` using a concise stash entry that references the spike artifact path.

When `promote_to` includes `learnings`, invoke `compound` with the spike artifact as source material.

### Phase 6: Write the artifact

Use this structure:

```markdown
---
title: "Short spike title"
type: spike
date: YYYY-MM-DD
time_box: "4h"
conclusion: "proceed"
confidence: "medium"
linked_feature: null
promoted_to:
  - plan
tags:
  - domain
  - technology
---

## Goal

## Success Criteria

## Scope Constraints

## Investigation Approach

## Findings

### What Was Discovered

### What Was Tried and Failed

### Remaining Unknowns

## Recommendation

## Next Steps

## References
```

## Quality criteria

* The goal is specific and answerable.
* Findings are evidence-based.
* Failed approaches are documented.
* The conclusion and confidence are explicit.
* Queue promotion reflects the real workspace behavior and does not assume a non-existent spike artifact type.
