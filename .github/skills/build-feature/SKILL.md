---
description: "Execute a harness loop — iteratively run tests, capture failures, fix code, and repeat until the harness passes or the circuit breaker trips"
---

# Build Feature

Implement a requested feature by continuously looping against a strict, compiling, but failing test harness until all tests pass.

## When to Use

Invoked by the ship agent when a task is harness-satisfied — it carries the `harness-ready` label, or it carries `harness-exempt`, passed the ship agent's P-002.1 static intake, and cleared the Step 4.1a claim-time gate. Not invoked directly by users.

## Inputs

* `task_id`: (Required) The backlog task ID to implement.
* `harness_cmd`: (Required) The **task-scoped** test command to run. Under P-002.6 it MUST be executable as written, name an explicit package path (never a bare `./...`), carry `-count=1`, be anchored to the task's own `^TestU<unit>_` selector, fail closed on a vacuous pass, and carry no `-short`, added build tag, `t.Skip`, or `|| true`. For a `harness-exempt` task there is no scaffolded red harness; the caller passes the command the loop must drive from failing to passing — the task's `exempt_verification_command` for `docs-only` and `verification-only`, or the predecessor owner's `harness_owner_command` for `covered-by`. The loop below runs against that command unchanged.
* `wave_scoped`: (Optional, default `false`) `true` when Ship dispatches this task from inside a P-002.6 wave. When `true`, the post-loop suite is **task-scoped** and the full-repository suite is Ship's Step 4.6 wave convergence gate, not this skill's. When `false` or absent, the post-loop suite is the full repository suite as before — this input relocates a gate inside a wave, it never removes one.
* `green_regression_cmds`: (Optional; meaningful only when `wave_scoped` is `true`; default `[]`)
  The exact array Ship parsed and froze from the task's canonical optional
  `green-regression-contract` at Step 3. The contract format is defined by P-002.6; an absent block
  means exactly `[]`. Run the supplied array unchanged. Never infer a command from task prose,
  discover a package by judgement, or reinterpret an omitted input as permission to choose.
* `sibling_red_selectors`: (Optional; meaningful only when `wave_scoped` is `true`) The closed, explicit list of `-run` selectors belonging to the other non-exempt members of the current wave that have not been built yet.
* `open_red_selectors`: (Optional; meaningful only when `wave_scoped` is `true`) The closed, explicit list of `-run` selectors in `open_red_deliverables_k` — red harnesses that were the *declared deliverable* of a task completed in an **earlier** wave and whose declared green-makers have not all closed. Together with `sibling_red_selectors` this is the **entire** tolerated-red set. Both are supplied by Ship, neither may be widened, inferred, or extended by this skill, and any failure outside their union is a real failure that fails the gate.
* `red_deliverable`: (Optional, default `false`) `true` when this task's declared deliverable **is** a red harness. Ship sets it from the task's canonical `red-deliverable-contract` and passes `harness_cmd` as the declared `red_selector_command`. It selects **Step 0.5**, the fail-closed red-deliverable execution branch, which **replaces** the generic harness loop rather than modifying it: the harness was already scaffolded with the wave by `harness-architect`, so Step 0.5 consumes and validates it — repo-wide compile check, assertion RED on the anchored selector, an empty changed-file set — reports the red evidence Ship needs for open-red accounting, and enters the inverted quality gates with no fix iteration. Do not iterate toward green, and never report success on a `harness_cmd` that passes — a red deliverable that passes at dispatch was never red.
* `red_baseline_sha`: (Required for `red_deliverable` tasks) The commit SHA Ship captured at its Step 4.1a item 5, **after** the wave's harness scaffolding commit and before this task was claimed. Use this exact value as the left side of the Step 0.5b zero-delta check. Do not re-derive it from `HEAD`: a self-derived range would either measure nothing or sweep in the wave's sibling harnesses and reject valid work. An absent baseline halts with `WAVE_RED_MAPPING_UNRESOLVED`.
* `exempt_gate_cmd`: (Required for `harness-exempt` tasks) The task's `exempt_verification_command`. This is the completion gate that must pass and match declared evidence after the deliverable lands. For every class except `covered-by` it is the same command as `harness_cmd`.
* `exempt_class`: (Required for `harness-exempt` tasks) The task's `harness_exemption_class` — `docs-only`, `verification-only`, or `covered-by`. Determines the allowed changed-file surface (P-002.4). `declaration-only` is **not** a valid value; the class was withdrawn in cycle 29 and a declaration task arrives here as a normal `harness-ready` task with a source-shape harness.
* `exempt_baseline_sha`: (Required for `harness-exempt` tasks) The commit SHA Ship captured at its Step 4.1a, immediately before claiming the task and before any mutation. Use this exact value as the left side of every P-002.4 diff. Do not re-derive it from `HEAD`, and do not proceed without it — an absent or re-derived baseline is `EXEMPT_DELTA_EXCEEDS_CLASS`, because the gate would then measure a different range than Ship's. This skill's completion gate runs **before** its own commit, so it pairs this SHA with the working-tree diff form, never with `..HEAD`.

## Output

* All harness tests passing
* For a `harness-exempt` task: `exempt_gate_cmd` observed failing (no marker) before the work and
  passing — exit 0 plus the exact `EXEMPT_VERIFY_OK:{task_id}` marker, non-vacuously — after it
* For a `red_deliverable` task: `go test -run=^$ -count=1 ./...` passing, `harness_cmd` observed
  failing on named assertions, an empty changed-file set against `red_baseline_sha`, and the
  Step 0.5c red-evidence report returned to Ship
* Code changes committed
* Task marked complete in backlog

## Required Protocol

When the `agent-intercom` capability pack is installed, follow
`.github/instructions/agent-intercom.instructions.md` throughout the loop: establish heartbeat /
ping visibility up front, broadcast meaningful attempt transitions, and route any destructive
actions through the intercom approval path rather than improvising local-only approval.

When the `agent-engram` capability pack is installed, follow
`.github/instructions/agent-engram.instructions.md` throughout the loop: prefer indexed symbol and
impact lookup while diagnosing failures, verify the workspace is bound before trusting engram
results, and refresh stale indexes before concluding the code graph is wrong.

### Step 0: Harness-exempt pre-work precondition (P-002.3 / P-002.5)

Run this step first for a `harness-exempt` task. Skip it when the task carries `harness-ready`.

1. **Screen before executing (P-002.5).** Statically check `exempt_gate_cmd` — and `harness_cmd`
   too, when `exempt_class` is `covered-by` and `harness_cmd` is therefore the distinct
   `harness_owner_command` — for a Constitution Principle VII destructive-operation pattern
   (file/directory deletion, force-overwrite, config or history mutation, data drops, package
   install/removal, untrusted code execution) before running anything.
   * A match on either command → **do not execute it**. Stop and report
     `EXEMPT_COMMAND_DESTRUCTIVE` to the caller; route the command through Principle VII operator
     approval instead of running it here.
   * No match → each command is read-only; proceed.
2. Run `exempt_gate_cmd` **exactly once** against the pre-work tree, before writing anything.
3. It MUST fail, and failure means **non-zero exit**, or exit 0 carrying one of these false-green
   signals from a *well-formed* command:
   * `[no tests to run]` — `go test -run <selector>` exits 0 when the selector matches nothing
   * `testing: warning: no tests to run` — the same case under `-v`
   * `no test files` — the package has no `_test.go` files
   * zero matching `--- PASS: <name>` lines for a named-selector command
4. Classify an **exit 0** run before any work is done by whether the declared
   `EXEMPT_VERIFY_OK:{task_id}` marker is present in stdout. Both branches halt:
   * Exit 0 **with** the marker → stop and report `EXEMPT_FALSE_GREEN` to the caller. Do not
     implement, do not weaken or replace the command, and do not proceed on the theory that the
     deliverable already exists.
   * Exit 0 **without** the marker → the command is malformed: P-002.3 requires every
     `exempt_verification_command` to print its marker as the last statement before its own exit 0,
     so an unmarked exit 0 is not evidence of anything. Stop and report `EXEMPT_MARKER_MISSING` to
     the caller. It is **not** the expected failed precondition and **not** a green light to enter
     Step 1 — this matches Ship Step 4.1a and the P-002.2 taxonomy, where `EXEMPT_MARKER_MISSING`
     is a halt code rather than a failure observation. Return the task for a contract amendment.
5. Record the observed pre-work output. It is this task's red-equivalent evidence.

Ship runs the same probe at its Step 4.1a claim-time gate. Running it again here is intentional
defense in depth: this skill must never begin a loop it has not seen fail.

### Step 0.5: Red-deliverable execution branch (P-002.6, fail-closed)

Run this step when `red_deliverable` is `true`. It **replaces** the generic harness loop below — a
red-deliverable task never enters that loop. When `red_deliverable` is `false` or absent, skip this
step entirely.

The deliverable of such a task **is** the landed, compiling, failing harness. Its declared
The deliverable of such a task **is** a compiling, failing harness. `harness-architect` scaffolded
that harness with the rest of the wave at Ship Step 4.0, so this branch **consumes and validates it
— it does not land it**. Its declared `green_maker_tasks` turn it green in a later wave, and Ship
carries its selector in `open_red_deliverables` until they do. Driving it green here destroys the
ordering contract it exists to pin.

**This branch writes nothing.** The harness already exists at dispatch; the task completes when its
RED is confirmed and recorded. Its changed-file set is therefore required to be **empty**, and that
is checked, not assumed.

#### Dispatch preconditions (halt, never repair)

1. `red_deliverable: true` and `harness-exempt` are mutually exclusive. An exempt task has no
   scaffolded harness and may never author a failing assertion (P-002.1). If `exempt_class` or
   `exempt_gate_cmd` arrives alongside `red_deliverable: true`, halt with
   `WAVE_RED_MAPPING_UNRESOLVED` and report both inputs.
2. `harness_cmd` MUST be the task's declared `red_selector_command` verbatim, and MUST satisfy
   every P-002.6 task-scoped requirement — explicit package path, `-count=1`, anchored to the
   task's own `^TestU<unit>_` selector, and no `-short`, added build tag, `t.Skip`, or `|| true`. A
   mismatch or a weakening is a contract defect: halt with `WAVE_RED_MAPPING_UNRESOLVED`. Do not
   substitute a command of your own.
3. `red_baseline_sha` MUST be present — the SHA Ship captured at Step 4.1a item 5, after the wave's
   harnesses were scaffolded and committed and before this task was claimed. It is the left side of
   the zero-delta check in Step 0.5b. An absent or self-derived baseline halts with
   `WAVE_RED_MAPPING_UNRESOLVED`; a gate that picks its own range measures nothing.

#### Step 0.5a: Consume and validate the scaffolded harness

The expected state at dispatch is **already RED**. That is the whole point: Ship Step 4.0 item 10
leaves every non-exempt member of the wave red at once, and `harness-architect` records
`Compilation: PASS` / `Red Phase: CONFIRMED` for each. This step re-confirms that evidence
independently rather than trusting the handoff.

1. Run the repo-wide compile check `go test -run=^$ -count=1 ./...`. It executes no test, so no
   sibling's red harness can redden it. A failure means the scaffolded harness or the tree does not
   compile, which contradicts the harness contract P-002 requires: halt with
   `RED_DELIVERABLE_HARNESS_UNCOMPILABLE` and return the task to `harness-architect`. Do not repair
   the harness here — this branch does not write.
2. Run `harness_cmd` **exactly once** and classify the result.

| Observed | Meaning | Action |
|---|---|---|
| Non-zero exit with `--- FAIL:` lines matching the anchored selector | The deliverable, confirmed | **Success** — record the evidence and go to Step 0.5b |
| Exit 0 in any form | The selector is green before any declared green-maker has closed | Halt with `WAVE_RED_DELIVERABLE_EARLY_GREEN` |
| A no-tests-to-run signal at any exit code | No test matches the declared selector, so the harness was never scaffolded under it and the red is vacuous | Halt with `WAVE_RED_DELIVERABLE_VACUOUS` |
| Any other non-zero exit with no matching `--- FAIL:` line — panic, timeout, package abort, harness runtime error | The harness ran but never asserted, so its red is not assertion RED | Halt with `RED_DELIVERABLE_NOT_ASSERTION_RED` and return the harness for repair |

A vacuous red is the exact mirror of the P-002.3 false-green rule: an exit code alone proves
nothing, and a selector that matches no test cannot be the deliverable. The same reasoning bars a
panic or a timeout — a non-zero exit is not evidence that an assertion failed.

#### Step 0.5b: Zero-delta gate

This branch consumes an existing harness, so the task's own changed-file set must be **empty**.
Compute it exactly as the harness-exempt gate does, against the Ship-supplied baseline and the
working tree — never against `..HEAD`, which at this point would compare the baseline with itself:

* `git diff --name-only {red_baseline_sha}` **and** `git diff --cached --name-only {red_baseline_sha}`

Because `red_baseline_sha` is captured **after** the wave's scaffolding commit, the wave's sibling
harnesses are already behind the baseline and cannot appear in this set. Any file that does appear
was written by this dispatch, which the branch is not permitted to do:

* a non-test `*.go` file → halt with `RED_DELIVERABLE_PRODUCTION_DELTA_REFUSED`. A red deliverable
  declares no production change; the behaviour that turns it green belongs to its green-makers.
* any other file → halt with `RED_DELIVERABLE_DELTA_OUT_OF_SURFACE`. There is no permitted surface
  at all, so a `*_test.go` extension, a configuration file, and a documentation file are equally
  out of it.

#### Step 0.5c: Red evidence for open-red accounting

Report to Ship, verbatim:

* the exact `harness_cmd` executed, which is the declared `red_selector_command`
* the observed `--- FAIL:` function names and the exit code
* confirmation that `go test -run=^$ -count=1 ./...` passed and that the zero-delta gate was empty
* the declared `green_maker_tasks` and `green_maker_closes_wave`, carried through unchanged

Ship Step 4.5 builds the `open_red_deliverables` entry from this report, and Step 4.6 items 4 and 5
re-confirm that entry at every later gate. An incomplete report leaves the entry unaccounted for:
report all four items or halt with `RED_DELIVERABLE_EVIDENCE_INCOMPLETE`.

#### Step 0.5d: Inverted quality gates, no fix iteration

Run the Post-Loop Quality Gates below with these substitutions, and do not iterate on a failure:

1. **Lint**: `golangci-lint run` — unchanged.
2. **Format**: `gofmt -l .` — unchanged.
3. **Test suite**: item 3 **inverts**. `harness_cmd` must still be observed **RED**, and a green
   result fails the gate with `WAVE_RED_DELIVERABLE_EARLY_GREEN`. The supplied
   `green_regression_cmds` array still runs unchanged and each command must still pass — it pins
   green behaviour this task does not touch. The tolerated-red set is exactly
   `sibling_red_selectors ∪ open_red_selectors ∪ {this task's own declared selector}`. That single
   addition is the task's declared deliverable; nothing else may be added, and a failure outside
   the union is a real failure.
4. **Harness-exempt completion gate**: does not apply — precondition 1 excluded it.

Then skip `### Commit`: the zero-delta gate has just proved there is nothing to commit. Report
success to Ship, which completes the task at its Step 4.5 and records the open-red entry there.

#### Step 0.5 executable coverage

This branch is a *contract executed by agents*, so nothing in the Go test suite can fail when it
regresses. Its routing and result classification are pinned by the tracked, read-only simulation
instead: `tests/simulation/wave-scheduler-contract.json` declares the
`red_deliverable_branch_controls` block and `scripts/wave-scheduler-sim.ps1` classifies each entry
with the same ordering this step specifies. Run it with
`pwsh -NoProfile -File scripts/wave-scheduler-sim.ps1`.

The controls cover every outcome above: the confirmed assertion-RED deliverable, an early-green
selector, a selector matching no test, panic and timeout rejection, an uncompilable scaffolded
harness, every dispatch-precondition refusal, both zero-delta refusals, an incomplete evidence
report, and two routing controls proving a `red_deliverable` dispatch never enters the generic loop
while an ordinary dispatch still does. The load-bearing control is
`red-deliverable-never-enters-generic-loop`: it feeds the exact observation the generic loop reads
as SUCCESS and requires this branch to halt on it instead, so deleting the branch fails the suite
rather than passing it silently.

When this step changes, update the controls in the same commit. A branch rule with no control is a
rule the simulation would still pass without.
### The Harness Loop (5-Attempt Circuit Breaker)

Enter this loop only when `red_deliverable` is `false` or absent. A `red_deliverable` task is
handled entirely by Step 0.5 and must never reach the loop, whose success condition — a passing
`harness_cmd` — is the opposite of that task's contract.

**Before entering the loop**: Read coding standards once — constitution Principle I
and `go.instructions.md`. These apply to all fix attempts.
Do not re-read the full standards on every iteration; only do a targeted re-read
if working on a file in an unfamiliar module or if the error pattern changes.

This loop is a skill-managed exception to the universal 3-retry circuit breaker
(per `circuit-breaker.instructions.md`). The 5-attempt limit governs within this
loop scope. However, if the **same error** recurs on attempts 3+, the universal
circuit breaker applies: stop and escalate.

```text
Attempt 1..5:
  1. Run harness_cmd → capture stdout/stderr
  2. If all tests pass → SUCCESS → exit loop
  3. Parse failure output → identify failing tests and error messages
  4. If error is substantially identical to previous attempt → check same-error recurrence limit
  5. Fix the code to address the specific failure
  6. Verify compilation: go test -run=^$ -count=1 ./...
  7. If compilation fails → fix compilation errors first
  8. Loop back to step 1

After 5 failures → mark task as BLOCKED → exit
```

### Step-by-Step Detail

#### Step 1: Run the Harness

Execute `harness_cmd` and capture the full output. Record execution time. This step and everything
that follows it belong to the generic loop; a `red_deliverable` task completed at Step 0.5d and
never arrives here.

**Stall timeouts**:

* Build/test commands: 45 minutes
* Other commands: 5 minutes

If the command exceeds the timeout, terminate and count it as a failed attempt.

#### Step 2: Evaluate Results

If all tests pass, proceed to quality gates. For a `harness-exempt` task, "pass" means
`harness_cmd` exits 0 **and** carries no P-002.3 false-green signal. An exit-0 run showing
`[no tests to run]`, `no test files`, or zero matching `--- PASS:` lines is a failed attempt, not
a success — parse it as a failure and continue the loop. When `harness_cmd` is
`exempt_verification_command` (every class except `covered-by`), an exit-0 run is additionally not
a success unless the declared `EXEMPT_VERIFY_OK:{task_id}` marker is present in its stdout. When
`harness_cmd` is `harness_owner_command` (`covered-by` only), the marker does not apply — evaluate
it the same way the owner's own harness is evaluated, by named `--- PASS:` count.

#### Step 3: Parse Failures

Extract from the test output:

* Which tests failed
* The assertion or error message
* The file and line where the failure occurred
* The expected vs. actual values (if applicable)

When the `agent-engram` capability pack is installed, use engram-first lookup to inspect symbols,
callers, and affected regions before expanding into broader file-based searches.

#### Step 4: Re-read Standards

Before writing any fix, re-read the relevant coding standards:

* Constitution Principle I (safety-first language practices)
* Technology-specific instructions (`go.instructions.md`)
* Any instruction files matching the files being modified

#### Step 5: Fix the Code

Apply targeted fixes to address the specific test failure. Do NOT:

* Modify the test to make it pass (tests are the specification)
* Add unrelated changes
* Refactor code not related to the failure
* Skip error handling to shortcut a fix

**Narrow exception — `verification-only` deliverables.** When `exempt_class` is
`verification-only`, the task's deliverable *is* new test or evidence content, so the skill MAY
create the new `*_test.go` files and evidence artifacts the task names. The exception is bounded:

* Only **new** files the task names, plus appends to the task's named evidence artifact.
* Never edit, delete, relax, rename, or narrow the selector of a **pre-existing** assertion in any
  file, including one authored earlier in the same task.
* Never add a build tag, `t.Skip`, or short-circuit that removes an existing assertion from the run.
* Every function added must be a green-step guard that asserts shipped or prerequisite-delivered
  behavior. Adding a failing assertion here is a fabricated RED and a P-002.1 violation.

This exception is specific to `verification-only`. It does **not** apply to `docs-only` (zero
`*.go` changes) or to `covered-by` (the owner owns the harness; any `*_test.go` change is
`EXEMPT_DELTA_EXCEEDS_CLASS`). It also has no bearing on a **declaration** task: such a task is not
exempt at all (P-002.1, cycle 29), it arrives here with a `harness-ready` source-shape harness
already red, and its implementation drives that harness green like any other harness-ready task.

**No-collision rule.** "New" is evaluated against the execution order, not against the repository
state when the task was written. A task's named `*_test.go` file must not be a file that an earlier
task in the dependency order already creates; if it is, the file is pre-existing by the time this
task runs and the exception does not admit the append. Halt with
`EXEMPT_DELTA_EXCEEDS_CLASS` and report the collision so the task can be given its own guard file.
Do **not** relax this exception to permit appends — the narrowing is deliberate.

If the root cause is still unclear after repeated attempts, or the task touches a risky subsystem, invoke **safety-modes** in `investigate-first` or `freeze-scope` mode before continuing.

#### Step 6: Verify Compilation

Run `go test -run=^$ -count=1 ./...` to confirm the fix compiles. If compilation fails, fix compilation errors before returning to the harness loop.

This check stays **repo-wide even inside a wave**: `-run=^$` executes no test, so a sibling's
still-red harness cannot redden it. It verifies exactly what it claims — that the tree still
builds — and must never be narrowed to the task's own package to dodge a real compile break.

### Post-Loop Quality Gates

After the harness passes:

1. **Lint**: `golangci-lint run`
2. **Format**: `gofmt -l .`
   * If violations found: `gofmt -w .` and re-check
3. **Test suite** — scope depends on `wave_scoped`:
   * **`wave_scoped: true`** (a P-002.6 wave dispatch): run the task's own scoped `harness_cmd`
     plus exactly the supplied `green_regression_cmds` array (default `[]`) **and nothing else**.
     **Do not run `go test ./...` here**, infer commands from prose, or add a package by judgement.
     Step 4.0 of Ship leaves every non-exempt member of the wave red at once and then builds them
     one at a time, and earlier waves may have completed red deliverables that are still failing by
     design, so a repo-wide suite would fail no matter how correct this task is — an unsatisfiable
     gate, not a stricter one. The tolerated-red set is exactly
     `sibling_red_selectors ∪ open_red_selectors`; neither may be widened, and a failure outside
     that union is a real failure that fails this gate. The full repository suite is **not
     skipped** — it runs at Ship Step 4.6 whenever the open-red set is empty, and at final closure.
     For a `red_deliverable` task this item inverts exactly as Step 0.5d specifies: `harness_cmd`
     must still be **RED**, and a green result fails the gate.
   * **`wave_scoped: false` or absent**: `go test ./...`, unchanged.
4. **Harness-exempt completion gate** (P-002.3 / P-002.4), for `harness-exempt` tasks only:
   * Screen `exempt_gate_cmd` for a Principle VII destructive pattern (P-002.5) before running it
     again. A match at this point is unexpected — Step 0 should already have rejected it — but
     still halts with `EXEMPT_COMMAND_DESTRUCTIVE` rather than executing.
   * Run `exempt_gate_cmd` once. It MUST exit 0, print the exact declared
     `EXEMPT_VERIFY_OK:{task_id}` marker, carry no other false-green signal, and match the task's
     declared evidence — the named `--- PASS:` guard count, the declared content assertions, or the
     declared evidence-manifest rows and scalars. A vacuous pass is `EXEMPT_EVIDENCE_MISMATCH`; an
     exit-0 run without the marker is `EXEMPT_MARKER_MISSING`. Report either and stop.
   * **Diff against the working tree, not against `HEAD`.** This gate runs **before** the `###
     Commit` step below, so the task's work is staged and/or unstaged in the working tree and is
     **not yet in `HEAD`**. `git diff {exempt_baseline_sha}..HEAD` at this moment compares the
     baseline commit against itself, yields an empty delta, and passes every check trivially — a
     gate that reads as fail-closed while enforcing nothing. Use the two-dot form with **no
     right-hand side**, which diffs the commit against the working tree, and add the `--cached`
     pass so staged changes are included:
     * path pass — `git diff --name-only {exempt_baseline_sha}` **and**
       `git diff --cached --name-only {exempt_baseline_sha}`; the union of the two is the task's
       changed-file set.
     * content pass — `git diff {exempt_baseline_sha} -- <each allowed file>` **and**
       `git diff --cached {exempt_baseline_sha} -- <each allowed file>`.
   * **An empty changed-file set is a halt, never a pass.** If the union above is empty, report
     `EXEMPT_DELTA_EXCEEDS_CLASS` with detail `empty delta — gate measured a range that does not
     contain the task's work` and stop. An exempt task that changed nothing has no deliverable and
     cannot have legitimately passed `exempt_gate_cmd`.
   * Confirm the changed-file set is a subset of the P-002.4 delta surface for `exempt_class`,
     diffing from `exempt_baseline_sha` rather than from a self-derived range. Anything outside the
     surface is `EXEMPT_DELTA_EXCEEDS_CLASS`; report it and stop. Do not "fix" a class violation by
     editing the contract.
   * Run the P-002.4 **content pass** as well. The class surfaces are content restrictions, so a
     path-only check passes changes it should reject. Under `verification-only`, a hunk that
     weakens, deletes, renames, or narrows the selector of a pre-existing assertion is
     `EXEMPT_DELTA_EXCEEDS_CLASS`, and so is any hunk in `.gitignore` or another
     repository-configuration file — `verification-only` is not a repository-hygiene class. Under
     `docs-only`, any `*.go` hunk is `EXEMPT_BEHAVIOR_NO_OWNER`. Under `covered-by`, any
     `*_test.go` hunk is `EXEMPT_DELTA_EXCEEDS_CLASS`.
   * Ship re-runs both passes at its own Step 4.3 against the same baseline — but **after** this
     skill's commit, so Ship correctly uses the `{exempt_baseline_sha}..HEAD` form there. The two
     forms are intentionally different because the two gates sit on opposite sides of the commit.
     Do not copy Ship's form into this step or this step's form into Ship's.

### Commit

If all quality gates pass — including the harness-exempt completion gate above, which ran against
the working tree precisely because this commit had not happened yet:

1. Stage all changes
2. Create a conventional commit message referencing the task ID
3. Report success to the caller

Do **not** reorder this step ahead of the completion gate to make an `..HEAD` diff work. The gate
must observe the delta before it is committed so that a failing class check can stop the task
without leaving a non-compliant commit behind.

## Behavioral Constraints

* No subagent spawning (leaf executor)
* Never modify an existing test file to reach green (tests are the specification). Creating the new test or evidence files a `verification-only` task names is the single narrow exception, bounded by Step 5
* Never weaken, delete, relax, rename the selector of, skip, or build-tag away an existing harness assertion to reach green — including on a `harness-exempt` task whose deliverable is a new green-step guard
* Never author a failing assertion for a `harness-exempt` task; a fabricated RED is a P-002.1 violation, not compliance
* Never begin the loop for a `harness-exempt` task without first observing `exempt_gate_cmd` fail (Step 0)
* Never treat an exit-0 run carrying a P-002.3 false-green signal as success
* Never treat an exit-0 run on `exempt_gate_cmd` that is missing its declared `EXEMPT_VERIFY_OK:{task_id}` marker as success — this is the distinct `EXEMPT_MARKER_MISSING` evidence failure (Step 0 item 4, Step 2), explicitly **not** a P-002.3 false-green signal
* Never execute an `exempt_gate_cmd` or `harness_owner_command` that the P-002.5 screen matches as destructive; report `EXEMPT_COMMAND_DESTRUCTIVE` and route it to Principle VII approval instead
* Never widen, infer, or extend `sibling_red_selectors` or `open_red_selectors`. They are closed sets supplied by Ship, and their union is the only red this skill may tolerate; a failure outside it is a real failure. Treating a wave as a blanket "ignore failing tests" mode is a P-002.6 violation. The single documented addition is Step 0.5d's `{this task's own declared selector}` on a `red_deliverable` dispatch, which is that task's deliverable and widens neither supplied set
* Never substitute implementer judgement for `green_regression_cmds`. The post-loop suite runs
  exactly Ship's supplied canonical array (default `[]`) and nothing else; task prose and "a
  package that looked green before" are not gate inputs
* Never drive a `red_deliverable` task to green, and never report success on its `harness_cmd` passing. Its deliverable is a compiling, failing harness; a green result is a contract violation to report
* Never run a `red_deliverable` task through the generic harness loop. Step 0.5 is the only branch that may execute it, and the loop's success condition is the inverse of its contract
* Never accept a `red_deliverable` result that is red only because the selector matched nothing. A no-tests-to-run signal at any exit code is `WAVE_RED_DELIVERABLE_VACUOUS`, never the deliverable
* Never scaffold, land, edit, or repair a red deliverable's harness inside this skill. `harness-architect` owns it and already scaffolded it with the wave; an uncompilable or non-asserting harness returns to that skill as `RED_DELIVERABLE_HARNESS_UNCOMPILABLE` or `RED_DELIVERABLE_NOT_ASSERTION_RED`
* Never write any file on a `red_deliverable` dispatch. The changed-file set against `red_baseline_sha` must be empty; a non-test `*.go` file is `RED_DELIVERABLE_PRODUCTION_DELTA_REFUSED` and anything else is `RED_DELIVERABLE_DELTA_OUT_OF_SURFACE`
* Never re-derive `red_baseline_sha` from `HEAD` or diff against `..HEAD` on this branch. Ship captures the baseline after the wave's scaffolding commit precisely so a sibling's harness cannot appear in this task's delta
* Never complete a `red_deliverable` task without returning the Step 0.5c red-evidence report; Ship builds the `open_red_deliverables` entry from it, and an unaccounted entry cannot be re-confirmed at Step 4.6
* Never accept a `harness_cmd` that fails the P-002.6 task-scoped requirements — a bare `./...`, a missing `-count=1`, an unanchored or sibling-matching selector, a `-short`/build-tag/`t.Skip`/`|| true` weakening, or a command that passes vacuously. Halt and report the contract defect rather than substituting a weaker command
* Never narrow the Step 6 compilation check (`go test -run=^$ -count=1 ./...`) to the task's own package. It runs no test, so a sibling's red harness cannot affect it
* Maximum 5 attempts before circuit breaker trips (skill-managed exception; see `circuit-breaker.instructions.md`)
* Same-error recurrence at attempt 3+ triggers the universal circuit breaker
* Read coding standards once at task start; targeted re-read for unfamiliar modules
* One file change per tool call; broadcast after each write
* When the `agent-intercom` capability pack is installed, use intercom broadcasts for attempt milestones and file-write visibility

## Quality Criteria

* All harness tests pass
* No lint violations
* No format violations
* The task-scoped suite passes; under `wave_scoped: true` the full repository suite is Ship's Step
  4.6 wave convergence gate, which runs unfiltered whenever the open-red set is empty and must pass
  before the next wave is admitted
* Changes are scoped to the task requirements
* For a `harness-exempt` task: the pre-work probe was observed failing (marker absent), the
  completion gate passes non-vacuously with the exact marker present, both commands cleared the
  P-002.5 read-only screen before either ran, and the changed-file set stays inside the P-002.4
  class delta surface
* For a `red_deliverable` task: Step 0.5 ran instead of the generic loop, the repo-wide compile
  check passes, `harness_cmd` fails on named assertions rather than on a build error, a
  no-tests-to-run signal, or a panic, the changed-file set against `red_baseline_sha` is empty, and
  the Step 0.5c evidence report was returned in full

## Model Routing

This skill operates at **Tier 2 (Standard)** — routine build loop execution and quality verification.

Generated by autoharness | Template: build-feature/SKILL.md.tmpl
