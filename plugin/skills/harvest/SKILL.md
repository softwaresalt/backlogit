---
name: harvest
description: "Decomposes a reviewed implementation plan into a configured backlogit hierarchy"
argument-hint: "plan=docs/plans/{YYYY-MM-DD}-{slug}-plan.md"
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
feature or chore, task, and optional subtask items. It is the reusable decomposition step for the
Stage workflow and replaces the embedded harvest phase inside the legacy
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

1. Covering feature or chore title from frontmatter and top-level headings.
2. Problem frame, requirements trace, decisions, and standards check for the
   covering parent description.
3. Task candidates from each implementation unit.
4. Optional subtask candidates from file lists, acceptance criteria, verification steps,
   and test surfaces inside each implementation unit.
5. Dependency edges from the plan's dependency graph.

Use repository search tools to validate file references or symbols when the plan
mentions existing code locations that need confirmation.

### Phase 3: Build the hierarchy model

Map the plan into the backlogit hierarchy:

* one covering feature or chore representing the whole reviewed plan
* one or more atomic tasks per implementation unit directly under that parent
* optional subtasks under tasks only when configured metadata permits them and
  granularity requires them

Plan headings are conceptual groups, not required work items. Do not invent
an epic or sub-epic type.

Before creating anything, apply the same backlog shaping rules that governed the
legacy decomposition path:

1. Keep tasks small enough to fit a focused implementation session.
2. Keep each task within a single skill domain.
3. Require a verifiable exit state for every task and subtask.
4. Preserve plan references so downstream shipment assembly can trace work back
   to the plan.

**Parent-first ordering (NON-NEGOTIABLE):** Tasks require a `parent_id`
referencing an existing covering feature or chore whose configured allowed
children include task. Create or identify that parent before creating tasks.
Validate optional subtask parent edges against the configured types as well.
Add the covering parent to a shipment before its child tasks. Omitting
`parent_id` for task-kind artifacts violates the required hierarchy.

### Phase 4: Execute or preview

If `${input:dry_run}` is `true`:

1. Produce the proposed parent, task, optional subtask, and dependency structure.
2. Broadcast the dry-run counts.
3. Do not call backlogit mutation tools.

If `${input:dry_run}` is `false`:

1. Query backlogit first to avoid duplicate covering parents.
2. Create or identify the covering feature or chore.
3. Create atomic tasks directly under that parent for each implementation unit.
4. Create subtasks only when allowed and needed for granularity.
5. Wire dependencies with backlogit dependency operations.
6. Broadcast each created parent, task, and dependency edge as it is written.

### Phase 5: Verify and report

1. Confirm the created hierarchy through backlogit read operations.
2. Report the created IDs, counts, and dependency summary.
3. Return the ready backlog as the output of the Stage-side workflow.
4. Recommend handing the resulting backlog to the Ship workflow for harness,
   build, review, CI, and pull request execution.

## Guardrails

* Do not modify the plan file.
* Do not skip duplicate checks.
* Do not create shipment artifacts from this skill. Shipment is a downstream
  concern handled after backlog staging.
* Keep descriptions self-contained enough for the next executor to act without
  reopening the plan for basic context.
