---
chunk_strategy: h1-h2-h3
description: Documents how shipment 006-S was marked ready for Ship before deliberation, exec plan, and plan review existed for harvested stash scope 834CCDB7 / 023.008-T.
doc_type: learning
docline:
    category: workflow_issue
    component: task_manager
    date: 2026-04-10T00:00:00Z
    file_path: docs/memory/2026-04-10-stage-stash-triage.md
    message: Shipment 006-S was marked ready for Ship even though harvested stash scope 834CCDB7 lacked deliberation, exec plan, and plan review gates.
    problem_type: workflow_issue
    resolution_type: documentation
    resolved: false
    root_cause: missing_tooling
    severity: high
    tags:
        - stage
        - shipment
        - stage-gating
        - pre-shipment-validation
        - stash-harvest
        - deliberation
        - exec-plan
        - plan-review
        - retroactive-gating
        - ready-for-ship
ingested_at: "2026-06-26T02:32:58Z"
schema_version: "1.0"
source: docs/compound/workflow-issues/shipment-ready-before-stage-gates-2026-04-10.md
title: Shipment ready before Stage gates completed
---

## Problem

Shipment `006-S` entered the Ship lane as if it were fully staged, even though
the repository workflow requires Stage evidence before a shipment is treated as
ready. The scope already existed as `023-F` and `023.008-T`, but the source
stash entry `834CCDB7` had no deliberation artifact, no implementation plan,
and no plan review when the shipment was declared ready in
`docs/memory/2026-04-10-stage-stash-triage.md`.

This was a workflow failure, not a feature implementation failure. In backlogit
terms, `harvest -> shipment assembly` was incorrectly treated as equivalent to
`Stage complete -> ready for Ship`.

## Symptoms

* `006-S` looked operationally ready for Ship even though no prior deliberation
  artifact existed for the scope
* There was no implementation plan for the shipment scope when it first entered
  the Ship lane
* There was no plan review artifact confirming the plan had passed a review gate
* Reviewers and later agents would have had to infer design intent from shipment
  membership instead of reading explicit Stage outputs
* The shipment, feature, and task had no explicit remediation traceability until
  comments were later appended to `006-S`, `023-F`, and `023.008-T`

## What Did Not Work

Treating harvest and shipment assembly as a readiness gate did not work.
Harvest packages scope, but it does not replace deliberation, planning, or
review.

Letting Ship proceed and planning to backfill Stage artifacts later did not
work. That approach hides missing design intent until execution is already
underway.

Recreating backlog items to fix the gap would also have been wrong. It would
have duplicated valid scope, broken continuity, and forced unnecessary shipment
churn.

Changing shipment membership was not the right remedy because the problem was
missing workflow evidence, not incorrect shipment composition.

## Solution

The fix was to retroactively apply the missing Stage gates in place for the
existing shipment instead of rebuilding scope.

1. Create the missing deliberation artifact, `011-DL`
2. Link that deliberation to the existing feature so `011-DL` informs `023-F`
3. Add the missing implementation plan at
   `docs/exec-plans/2026-04-10-event-traceability-commit-tracking-plan.md`
4. Add the advisory plan review at
   `.copilot-tracking/plan-review/2026-04-10-event-traceability-commit-tracking-plan-review.md`
5. Capture the remediation trail in
   `docs/memory/2026-04-10-stage-006-S-retroactive-gating.md`
6. Append traceability comments to `006-S`, `023-F`, and `023.008-T`
7. Leave shipment membership unchanged and avoid creating duplicate backlog
   items

### Before

```text
006-S assembled
023-F exists
023.008-T exists
No deliberation artifact
No implementation plan
No plan review
Ship sees shipment as ready
```

### After

```text
006-S assembled
023-F unchanged
023.008-T unchanged
011-DL created
011-DL informs 023-F
Implementation plan recorded in docs/exec-plans/...
Advisory plan review recorded in .copilot-tracking/plan-review/...
Memory record added in docs/memory/...
Traceability comments appended to 006-S, 023-F, and 023.008-T
No duplicate backlog items
Shipment membership unchanged
```

## Why This Works

This remedy fixes the control gap without rewriting valid backlog structure. The
shipment stays stable for execution, while the missing Stage evidence is
restored around it.

It works because backlogit's workflow is artifact-driven. Once `011-DL`, the
implementation plan, the advisory plan review, and the traceability comments
exist, future agents can follow a concrete chain of intent:

```text
deliberation -> feature -> task -> shipment
```

That preserves context, explains why the shipment is legitimate, and avoids the
operational damage that would come from duplicating items or reshuffling
shipment membership after work had already been assembled.

## Prevention

* Do not treat harvest or shipment assembly as proof that a shipment is ready
  for Ship
* Before handing a shipment to Ship, verify that its scope has a deliberation
  artifact, an implementation plan in `docs/exec-plans/`, and a plan review
  artifact, even if the review result is advisory
* Treat Stage completion as a real gate, not as documentation that can be
  safely backfilled later
* If a shipment is already assembled and the gate is missing, remediate in
  place first: add the missing artifacts, link them to the existing feature, and
  append traceability comments
* Do not create replacement backlog items unless the actual scope changed
* Add a shipment-readiness validator such as `backlogit_validate_shipment` or a
  Doctor rule that checks for deliberation, plan, and review lineage before a
  shipment is handed to Ship
* Keep Stage memory precise: backlog existence and shipment membership are not
  the same thing as gate completion

## Related Solutions

* [Stash Staleness Detection and Removal Requires Custom Scripting](stash-staleness-requires-custom-scripting-2026-04-09.md)
  shows the same class of workflow problem where agents compensate for missing
  lifecycle tooling with manual or implicit workarounds.
* [Orphaned Tasks Without Parent Features](orphaned-tasks-without-parent-features-2026-04-10.md)
  covers another enforcement gap where declared workflow invariants exist in
  configuration but are not mechanically validated by the tool surface.
* `docs/memory/2026-04-10-stage-006-S-retroactive-gating.md` records the
  remediation session that created the missing Stage lineage for `006-S`.
