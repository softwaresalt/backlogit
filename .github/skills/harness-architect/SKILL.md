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
* `${input:tasks}`: (Required in wave mode) Comma-separated task IDs for the
  **current P-002.6 wave** — the tasks whose dependencies are all `done`. Ship
  supplies this set from its Step 4.0 wave admission. When omitted, use all
  ready tasks under the feature whose dependencies are complete; never scaffold
  a task with an unfinished dependency.

## Workflow

### Step 1: Claim the ready task set

**This skill is invoked once per wave** (P-002.6), not once per shipment. Its
scope is the current wave's ready set and nothing else.

1. Load the feature or chore and its ready descendants through backlog
   query or queue operations.
2. If `${input:tasks}` is present, restrict the scope to that explicit
   task set — it is the wave, and it is authoritative.
3. Exclude blocked, done, or otherwise non-ready work items.
4. Exclude tasks that are already harness-satisfied: those carrying
   `harness-ready`, and those carrying `harness-exempt` that pass the
   P-002.1 static intake in Step 1a. Do **not** exclude a `covered-by` task's
   named harness owner — the owner is a scaffolding target.
5. **Exclude every task with an unfinished dependency.** Such a task belongs to
   a later wave. Its harness may not compile yet, and scaffolding it now is the
   one-pass deadlock waves exist to prevent.
6. Preserve the work-item-to-task mapping so each harness can be traced
   back to the correct backlog item.
7. Assume every declaration this wave's harnesses compile against has **already
   landed**, because the tasks that own those declarations are dependencies and
   are therefore terminal (`done` or `archived`). If a scaffold still cannot
   compile because a declared type or field is absent, the dependency graph is
   missing an edge: halt and report the missing prerequisite to Ship for a plan
   amendment. Do not fabricate the declaration, do not stub it into the test
   file to force compilation, and do not scaffold ahead of the wave.
8. **Scaffolding the whole wave in one pass is expected.** Every non-exempt
   member of the wave may be scaffolded together, which leaves them all red at
   the same time. That is the designed state, not a defect: Ship then drives
   each member green against its **own scoped selector**, and the unfiltered
   repository suite runs at Ship's Step 4.6 wave convergence gate whenever the
   open-red set is empty. Record each task's scoped selector (Step 6) so the
   build loop and Ship's Step 4.3 share one boundary and no sibling's red is
   ever attributed to the task under build.
9. **A red-deliverable task's harness is the deliverable, and it stays red past
   its own wave.** When a task carries a `red-deliverable-contract` block
   (`red_deliverable: true`), the harness this skill scaffolds is what the task
   ships: no later step in that wave drives it green, and the declared
   `green_maker_tasks` do so in later waves. Scaffold it exactly as any other
   red harness, record its `red_selector_command` as the task's scoped command,
   and record the declared green-makers in the manifest note so Ship's Step 3
   mapping and Step 4.6 open-red set can be built from artifacts rather than
   from inference. Never scaffold a green assertion into it to "balance" it.

### Step 1a: Harness-exempt static intake (P-002.1, fail-closed)

`harness-exempt` is an alternative way to satisfy P-002, never a way to skip
it. Read `.github/policies/workflow-policies.md` § P-002.1 and evaluate every
task in scope that carries the label. The label alone is not admission.

**This step evaluates static conditions only.** It runs before any harness is
generated, so no predecessor owner can have red evidence yet. Evaluating owner
red evidence here would make `covered-by` unsatisfiable by construction and
deadlock the release unit. Owner red evidence belongs to Ship's claim-time gate
(`.ship.agent.md` Step 4.1a).

A task is statically admitted as exempt and MUST NOT be scaffolded when all of
the following hold:

* it carries a harness exemption contract block delimited by
  `<!-- BEGIN:harness-exemption-contract -->` /
  `<!-- END:harness-exemption-contract -->` with the five canonical keys in
  order: `harness_exemption_class`, `harness_exemption_reason`, `harness_owner`,
  `exempt_verification_command`, `exempt_precondition` (plus
  `harness_owner_command` when the class is `covered-by`)
* `harness_exemption_class` is exactly one of `docs-only`,
  `verification-only`, or `covered-by`. `declaration-only` was withdrawn in
  cycle 29 and is `EXEMPT_CLASS_UNRECOGNIZED`
* `harness_exemption_reason` is one non-empty line
* `exempt_verification_command` is an exact runnable command and
  `exempt_precondition` is the literal `must-fail-before-deliverable`
* the governing plan, feature, or shipment contract enumerates a closed exempt
  set containing this task
* if its declared deliverable changes production behavior, its class is
  `covered-by` (P-002.4 objective test)
* when the class is `covered-by`, the task named in `harness_owner` exists in the
  same release unit, is a declared dependency of this task, does **not** itself
  carry `harness-exempt`, and `harness_owner_command` is present and exact

If any condition fails, do **not** scaffold a substitute harness. Halt and
report the P-002.2 code (`EXEMPT_CONTRACT_INCOMPLETE`,
`EXEMPT_CLASS_UNRECOGNIZED`, `EXEMPT_COMMAND_MISSING`, `EXEMPT_NOT_IN_CONTRACT`,
`EXEMPT_BEHAVIOR_NO_OWNER`, or `EXEMPT_OWNER_INVALID`) to the caller. Do **not**
raise `EXEMPT_OWNER_NOT_RED` from this skill — it is a claim-time code and
raising it here is a false halt.

**Scaffold the owners.** A `covered-by` task's named `harness_owner` is not
exempt. Unless it already carries `harness-ready`, include it in this run's
scaffolding set so its red evidence exists before the exempt task reaches its
claim-time gate. Report each owner-to-exempt-task pairing back to the caller.

Never manufacture a test to give an exempt task a red assertion. A test that can
only fail because a symbol does not yet exist is a build error, and a test that
passes the moment a declared shape lands was never red. Behavior-changing tasks
outside the exempt set still require a genuine failing harness before
implementation. An exempt task's observed failure is its
`exempt_verification_command` run at Ship Step 4.1a, not a fabricated test file
authored here.

**A declaration task is not exempt and gets a source-shape harness.** A task
whose delta lands an exported type, a serialized struct field, a function or
method signature, or a sentinel is behavior-changing production code (P-002.1,
cycle 29). It is a scaffolding target like any other. Scaffold it a harness that
asserts over the package's **own source text** rather than over the missing
symbol:

* parse the named production file with `go/parser` into a `*ast.File` and assert
  the declared shape through `go/ast` — the type exists, the struct carries a
  field with the exact `json:"…"` tag, the function is declared with the
  expected name and receiver-less form;
* import only `go/ast`, `go/parser`, `go/token`, `strings`, and `testing`, so the
  file **compiles before the declaration exists** and
  `go test -run=^$ -count=1 ./...` exits 0 (P-002 postcondition);
* fail with an assertion message naming what is missing — for example
  `RemediationIntent struct is not declared in checkpoint_schema.go` — so the
  red is a genuine P-004 red phase, not a build error;
* name the functions `TestU<unit>_<Descriptor>` like any other harness, so the
  unit's red selector matches them.

This is the correct resolution of the old tension between "a build error is not
a red" and "a test that passes the instant the shape lands was never red": the
source-shape harness is neither. Do **not** instead reference the undeclared
symbol — that is the build error P-004 rejects — and do **not** ask for an
exemption.

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
   reason. This applies **only** where the task's delta is behaviour on a symbol
   that already exists. Where the task's delta *introduces* the symbol, write no
   stub at all — see item 5.
4. Keep signatures, types, and module names aligned with the current
   codebase.
5. For a **declaration** task, use the source-shape (`go/ast`) form from Step 1a
   instead of a stub-plus-behavior harness. The declaration is the deliverable,
   so there is no behavior to stub and no signature to align against — the
   harness parses the named production file and asserts the shape is present.
   A production stub is **not** written for such a task: writing one would make
   the harness pass on the scaffolding commit, which is the "never red" failure
   mode P-004 rejects, **and** it would land production surface ahead of the
   harness that gates it, which P-002.1 forbids outright (cycle-31). The order is
   always harness first, declaration second.
6. A **seam or declaration whose body would absorb real behaviour** — reading,
   mutating, or writing — is not a single task. Its declaration is gated by a
   source-shape harness; its behaviour is gated by a separate behaviour-harness
   task in a later wave. If a task asks for both at once, halt and report the
   plan defect rather than scaffolding a behaviour-carrying stub.

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

Run each task's **own scoped selector** — `go test -count=1 -run '^TestU<unit>_' ./<pkg>` — for the
harness tests it scaffolds. ALL of that task's tests MUST fail with the expected failure marker
(// TODO: implement).

**Do not run `go test ./...` for the red-phase check.** When this skill scaffolds a whole P-002.6
wave in one pass, every non-exempt member of the wave is red simultaneously and by design, and a
red deliverable from an earlier wave may still be failing by design too; a repo-wide run cannot
tell one task's intended red from a sibling's or from an open-red deliverable. Evaluate each
scaffolded task against its own selector, and record that selector as the task's scoped harness
command so `build-feature` and Ship Step 4.3 use the same boundary.

If any test passes (false positive) or fails with an unexpected error
(compilation vs runtime), fix the harness.

For a **declaration** task's source-shape harness, the expected failure marker
is the assertion message naming the absent shape (for example
`… is not declared in <file>.go`), not a `// TODO: implement` panic. It MUST be
an assertion failure reported by `testing`, never a build error — if
`go test -run=^$ -count=1 ./...` fails for that package, the harness references
the undeclared symbol and must be rewritten in the `go/ast` form.

### Step 6: Apply harness-ready label

After both checks pass (P-004 gate satisfied):

1. Update each task with `harness-ready` label using the backlog tool's
   update operation.
2. Add an implementation note with the harness command. It MUST be the task's
   **scoped** command — an explicit package path, `-count=1`, and a `^TestU<unit>_`
   selector anchored to this task's own functions — so it can neither match nor be
   satisfied by a sibling's harness (P-002.6 task-scoped command requirements).
3. Record the harness manifest: `Compilation: PASS`,
   `Red Phase: CONFIRMED`.

Apply this label only to tasks this run scaffolded. Do not add it to
P-002.1-valid `harness-exempt` tasks — they are already harness-satisfied and
carry no scaffolded harness. When a scaffolded task is the named
`harness_owner` of an exempt `covered-by` task, record that pairing in the
manifest note so Ship's claim-time gate can find the red evidence. When a
scaffolded task carries a `red-deliverable-contract`, record its declared
`green_maker_tasks` and `green_maker_closes_wave` in the same note, so Ship's
Step 3 red-deliverable mapping is built from a recorded artifact and its Step 4.6
deferral is traceable to a stated closing wave.

## Completion Criteria

The skill is complete only when the selected tasks have:

* harness files in the correct modules
* structural stubs with intentional not-implemented behavior, **except** for
  declaration tasks, which carry a source-shape harness and no stub at all
* a successful `go test -run=^$ -count=1 ./...` result after scaffolding
* all harness tests failing with the expected marker, verified per task against
  that task's own scoped selector
* clear mapping from backlog task to its scoped harness command

Tasks excluded as P-002.1-valid `harness-exempt` are reported as
already-satisfied, with their class and reason echoed back. They are not a gap
and not a scaffolding target. Every `covered-by` owner scaffolded in this run is
reported alongside the exempt task it covers.

## Guardrails

* Do not implement production logic — stubs only, and only where the task's delta
  is behaviour on an already-declared symbol.
* Do not write a production stub for a declaration task, and never land any
  production stub ahead of the harness that gates it (P-002.1, cycle-31). The
  order is harness first, declaration second.
* Do not scaffold a behaviour-carrying stub for a seam. Report the plan defect
  and let the declaration / behaviour-harness / implementation split be made.
* Do not skip compilation verification.
* Do not use a repo-wide `go test ./...` as the red-phase check; evaluate each
  task against its own scoped selector so a sibling's designed red is never
  mistaken for this task's.
* Do not apply the harness-ready label until both compilation and red
  phase checks pass.
* Do not scaffold tests, stubs, or a `harness-ready` label for a P-002.1-valid
  `harness-exempt` task.
* Do not fabricate a failing assertion to manufacture a red phase for an exempt
  task; halt and report the P-002.2 code instead.
* Do not raise `EXEMPT_OWNER_NOT_RED` from this skill. It is Ship's claim-time
  code; owner red evidence cannot exist before this skill runs.
* Do not skip scaffolding a `covered-by` task's named harness owner. The owner
  is the red evidence the exempt task depends on.
* Keep test scenarios traceable to acceptance criteria.

## Model Routing

This skill operates at **Tier 2 (Standard)** — test scaffolding is structured but routine.
