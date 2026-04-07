---
name: harvest
description: "Decomposes a reviewed implementation plan into backlogit feature/task/subtask hierarchy"
argument-hint: "plan=docs/exec-plans/{YYYY-MM-DD}-{slug}-plan.md"
input:
  properties:
    plan:
      type: string
      description: "Path to the reviewed implementation plan"
    dry_run:
      type: boolean
      description: "When true, output the planned structure without creating entries"
  required:
    - plan
---

# Harvest Skill

The `harvest` skill turns a reviewed implementation plan into backlogit
feature, task, and subtask items. It is the reusable decomposition step for the
Groomer workflow and replaces the embedded harvest phase inside the legacy
`backlog-harvester` agent.

This skill does not perform planning or review. It assumes the incoming plan is
already reviewed or otherwise approved for decomposition.

## Agent-Intercom Communication (NON-NEGOTIABLE)

Call `ping` at session start. If `agent-intercom` is reachable, broadcast at
every step. If unreachable, warn the operator that visibility is degraded and
continue locally.

| Event | Level | Message prefix |
|---|---|---|
| Session start | info | `[HARVEST] Starting: plan={input.plan}` |
| Plan accepted | info | `[HARVEST] Using reviewed plan: {plan_path}` |
| Structure parsed | info | `[HARVEST] Parsed implementation units: {unit_count}` |
| Dry run | info | `[HARVEST] Dry run: {feature_count} features, {task_count} tasks, {subtask_count} subtasks` |
| Feature created | info | `[HARVEST] Created feature: {feature_id} - {title}` |
| Task created | info | `[HARVEST] Created task: {task_id} - {title}` |
| Dependency wired | info | `[HARVEST] Dependency: {item_id} blocked by {depends_on}` |
| Complete | success | `[HARVEST] Complete: {feature_count} features, {task_count} tasks, {subtask_count} subtasks` |

## Inputs

* `${input:plan}`: (Required) Path to the reviewed implementation plan.
* `${input:dry_run:false}`: (Optional, defaults to `false`) Preview the planned
  hierarchy without creating backlogit entries.

## Workflow

### Phase 1: Validate the reviewed plan

1. Read `${input:plan}` in full.
2. Confirm the file exists and represents an implementation plan.
3. Confirm the plan has already cleared the review gate or is explicitly marked
   ready for harvesting.
4. Broadcast the accepted plan path.
5. Halt if the plan is missing, unreadable, or clearly not ready for backlog
   creation. Recommend running `plan-review` first when the review state is
   unclear.

### Phase 2: Parse the plan structure

Extract the planning data needed for decomposition:

1. Root feature title from frontmatter and top-level headings.
2. Problem frame, requirements trace, decisions, and standards check for the
   root feature description.
3. Task candidates from each implementation unit.
4. Subtask candidates from file lists, acceptance criteria, verification steps,
   and test surfaces inside each implementation unit.
5. Dependency edges from the plan's dependency graph.

Use repository search tools to validate file references or symbols when the plan
mentions existing code locations that need confirmation.

### Phase 3: Build the hierarchy model

Map the plan into the backlogit hierarchy:

* one feature representing the whole reviewed plan
* one task per implementation unit
* one or more subtasks per file group, verification slice, or explicit
  execution step inside the unit

Before creating anything, apply the same backlog shaping rules that governed the
legacy decomposition path:

1. Keep tasks small enough to fit a focused implementation session.
2. Keep each task within a single skill domain.
3. Require a verifiable exit state for every task and subtask.
4. Preserve plan references so downstream shipment assembly can trace work back
   to the plan.

### Phase 4: Execute or preview

If `${input:dry_run}` is `true`:

1. Produce the proposed feature, task, subtask, and dependency structure.
2. Broadcast the dry-run counts.
3. Do not call backlogit mutation tools.

If `${input:dry_run}` is `false`:

1. Query backlogit first to avoid duplicate root features.
2. Create the root feature.
3. Create one task per implementation unit under that feature.
4. Create granular subtasks under each task.
5. Wire dependencies with backlogit dependency operations.
6. Broadcast each created feature, task, and dependency edge as it is written.

### Phase 5: Verify and report

1. Confirm the created hierarchy through backlogit read operations.
2. Report the created IDs, counts, and dependency summary.
3. Return the ready backlog as the output of the Groomer-side workflow.
4. Recommend handing the resulting backlog to the Shipper workflow for harness,
   build, review, CI, and pull request execution.

## Guardrails

* Do not modify the plan file.
* Do not skip duplicate checks.
* Do not create shipment artifacts from this skill. Shipment is a downstream
  concern handled after backlog grooming.
* Keep descriptions self-contained enough for the next executor to act without
  reopening the plan for basic context.
