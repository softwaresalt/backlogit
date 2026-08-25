---
name: harness-architect
description: "Scaffolds compilable but failing test harnesses for feature and chore tasks"
argument-hint: "feature=001-{SUFFIX_FEATURE} tasks=001.001-{SUFFIX_TASK},001.002-{SUFFIX_TASK}"
input:
  properties:
    feature:
      type: string
      description: "Feature or chore ID to scaffold harnesses for"
    tasks:
      type: string
      description: "Comma-separated task IDs to scaffold (optional, defaults to all ready tasks)"
  required:
    - feature
---

# Harness Architect Skill

Scaffold strict test harnesses for a feature's or chore's ready work items.
The output must compile cleanly, fail for the intended not-yet-implemented
behavior, and leave clear harness commands for downstream build execution.

## Purpose

Use this skill when a release unit needs executable test boundaries before
implementation starts. The skill prepares the red phase and stops there. It
does not implement production logic.

## Agent-Intercom Communication

When the `agent-intercom` capability pack is installed, call `ping` at
session start. If reachable, broadcast at every step. If unreachable,
warn the operator that visibility is degraded and continue locally.

| Event | Level | Message prefix |
|---|---|---|
| Session start | info | `[HARNESS] Starting: feature={input.feature}` |
| Tasks loaded | info | `[HARNESS] Ready tasks: {task_count}` |
| Exempt skipped | info | `[HARNESS] Exempt (P-002.1): {task_id} — {class}` |
| Codebase analyzed | info | `[HARNESS] Context gathered: {module_count} modules` |
| Harness generated | info | `[HARNESS] Generated: {test_file} ({scenario_count} scenarios)` |
| Compilation check | info | `[HARNESS] Compilation: {result}` |
| Red phase check | info | `[HARNESS] Red phase: {result}` |
| Label applied | success | `[HARNESS] harness-ready: {task_id}` |
| Complete | success | `[HARNESS] Complete: {task_count} tasks harnessed` |

## Inputs

* `${input:feature}`: (Required) Feature or chore ID such as `001-F`
* `${input:tasks}`: (Optional) Comma-separated task IDs to scaffold.
  When omitted, use all ready tasks under the feature.

## Workflow

### Step 1: Claim the ready task set

1. Load the feature or chore and its ready descendants through backlog
   query or queue operations.
2. If `${input:tasks}` is present, restrict the scope to that explicit
   task set.
3. Exclude blocked, done, or otherwise non-ready work items.
4. Exclude tasks that are already harness-satisfied: those carrying
   `harness-ready`, and those carrying `harness-exempt` that pass the
   P-002.1 evaluation in Step 1a.
5. Preserve the work-item-to-task mapping so each harness can be traced
   back to the correct backlog item.

### Step 1a: Harness-exempt evaluation (P-002.1, fail-closed)

`harness-exempt` is an alternative way to satisfy P-002, never a way to skip
it. Read `.github/policies/workflow-policies.md` § P-002.1 and evaluate every
task in scope that carries the label. The label alone is not admission.

A task is exempt and MUST NOT be scaffolded when all of the following hold:

* its declared exemption class is one of `declaration-only`, `docs-only`,
  `verification-only`, or `covered-by <owner-id>`
* it declares a one-line reason for carrying no red harness
* the governing plan, feature, or shipment contract enumerates a closed exempt
  set containing this task
* if it changes behavior, its class is `covered-by <owner-id>`, and that owner
  exists in the same release unit, is a declared dependency of this task, is
  itself `harness-ready` rather than exempt, and has confirmed red evidence
  (`Compilation: PASS`, `Red Phase: CONFIRMED`) with its harness commit landed

If any condition fails, do **not** scaffold a substitute harness. Halt and
report the P-002.2 code (`EXEMPT_CLASS_UNRECOGNIZED`, `EXEMPT_NOT_IN_CONTRACT`,
`EXEMPT_BEHAVIOR_NO_OWNER`, `EXEMPT_OWNER_INVALID`, or `EXEMPT_OWNER_NOT_RED`)
to the caller.

Never manufacture a test to give an exempt task a red assertion. A test that can
only fail because a symbol does not yet exist is a build error, and a test that
passes the moment a declared shape lands was never red. Behavior-changing tasks
outside the exempt set still require a genuine failing harness before
implementation.

### Step 2: Read task intent

1. Read each selected task's title, description, acceptance criteria,
   and file references.
2. Pull in feature-level acceptance criteria when task text depends on
   broader feature behavior.
3. Translate acceptance criteria into named test scenarios before writing
   code.
4. Identify the correct module, test tier, and affected files for each
   harness.

When the `agent-engram` capability pack is installed, prefer indexed
symbol lookup and code-graph tools over broad grep when surveying
existing modules, test patterns, and import paths.

### Step 3: Determine execution posture

For each task, select the appropriate harness strategy:

| Posture | When to use | Harness pattern |
|---|---|---|
| **test-first** | New functionality with clear inputs/outputs | Write failing tests for expected behavior |
| **characterization-first** | Modifying existing behavior | Write tests that capture current behavior then modify |
| **migration-first** | Moving code between modules | Write tests at the destination, verify source behavior |
| **spike** | Exploratory with uncertain approach | Write minimal integration test, implement spike, expand tests |

### Step 4: Generate failing harness skeletons

1. Create test files that express the task intent as compilable tests.
2. Prefer table-driven or parameterized tests when the task describes
   multiple scenarios.
3. Create matching production stubs with // TODO: implement bodies
   so the module compiles while the tests still fail for the intended
   reason.
4. Keep signatures, types, and module names aligned with the current
   codebase.

### File placement rules

Write harness files into the module that matches the work item's scope:

* **Unit harnesses**: colocated with the production code in the
  appropriate cmd/, internal/, tests/ subdirectory
* **Integration harnesses**: in `tests//integration/` when the
  task spans modules or runtime boundaries
* **Contract harnesses**: in `tests//contract/` when the task
  defines API, CLI, or schema behavior

Write companion stub files into the production module that the tests
exercise. Do not place scaffolding in unrelated modules.

### Step 5: Verify harness

#### Step 5.1: Compilation check

Run `go test -run=^$ -count=1 ./...` including tests. The harness MUST compile.

If compilation fails, fix the harness until it compiles. Do not proceed
with a non-compiling harness.

#### Step 5.2: Red phase check

Run `go test ./...` for the harness tests. ALL tests MUST fail with
the expected failure marker (// TODO: implement).

If any test passes (false positive) or fails with an unexpected error
(compilation vs runtime), fix the harness.

### Step 6: Apply harness-ready label

After both checks pass (P-004 gate satisfied):

1. Update each task with `harness-ready` label using the backlog tool's
   update operation.
2. Add an implementation note with the harness command.
3. Record the harness manifest: `Compilation: PASS`,
   `Red Phase: CONFIRMED`.

Apply this label only to tasks this run scaffolded. Do not add it to
P-002.1-valid `harness-exempt` tasks — they are already harness-satisfied and
carry no scaffolded harness.

## Completion Criteria

The skill is complete only when the selected tasks have:

* harness files in the correct modules
* structural stubs with intentional not-implemented behavior
* a successful `go test -run=^$ -count=1 ./...` result after scaffolding
* all harness tests failing with the expected marker
* clear mapping from backlog task to harness command

Tasks excluded as P-002.1-valid `harness-exempt` are reported as
already-satisfied, with their class and reason echoed back. They are not a gap
and not a scaffolding target.

## Guardrails

* Do not implement production logic — stubs only.
* Do not skip compilation verification.
* Do not apply the harness-ready label until both compilation and red
  phase checks pass.
* Do not scaffold tests, stubs, or a `harness-ready` label for a P-002.1-valid
  `harness-exempt` task.
* Do not fabricate a failing assertion to manufacture a red phase for an exempt
  task; halt and report the P-002.2 code instead.
* Keep test scenarios traceable to acceptance criteria.

## Model Routing

This skill operates at **Tier 2 (Standard)** — test scaffolding is structured but routine.
