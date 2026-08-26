---
description: "Execute a harness loop — iteratively run tests, capture failures, fix code, and repeat until the harness passes or the circuit breaker trips"
---

# Build Feature

Implement a requested feature by continuously looping against a strict, compiling, but failing test harness until all tests pass.

## When to Use

Invoked by the ship agent when a task is harness-satisfied — it carries the `harness-ready` label, or it carries `harness-exempt`, passed the ship agent's P-002.1 static intake, and cleared the Step 4.1a claim-time gate. Not invoked directly by users.

## Inputs

* `task_id`: (Required) The backlog task ID to implement.
* `harness_cmd`: (Required) The test command to run (e.g., `go test ./...`). For a `harness-exempt` task there is no scaffolded red harness; the caller passes the command the loop must drive from failing to passing — the task's `exempt_verification_command` for `declaration-only`, `docs-only`, and `verification-only`, or the predecessor owner's `harness_owner_command` for `covered-by`. The loop below runs against that command unchanged.
* `exempt_gate_cmd`: (Required for `harness-exempt` tasks) The task's `exempt_verification_command`. This is the completion gate that must pass and match declared evidence after the deliverable lands. For every class except `covered-by` it is the same command as `harness_cmd`.
* `exempt_class`: (Required for `harness-exempt` tasks) The task's `harness_exemption_class` — `declaration-only`, `docs-only`, `verification-only`, or `covered-by`. Determines the allowed changed-file surface (P-002.4).

## Output

* All harness tests passing
* For a `harness-exempt` task: `exempt_gate_cmd` observed failing (no marker) before the work and
  passing — exit 0 plus the exact `EXEMPT_VERIFY_OK:{task_id}` marker, non-vacuously — after it
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

### The Harness Loop (5-Attempt Circuit Breaker)

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

Execute `harness_cmd` and capture the full output. Record execution time.

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

The same bounded exception applies to the guard files a `declaration-only` task names. It does
**not** apply to `docs-only` (zero `*.go` changes) or to `covered-by` (the owner owns the harness;
any `*_test.go` change is `EXEMPT_DELTA_EXCEEDS_CLASS`).

**No-collision rule.** "New" is evaluated against the execution order, not against the repository
state when the task was written. A task's named `*_test.go` file must not be a file that an earlier
task in the dependency order already creates; if it is, the file is pre-existing by the time this
task runs and the exception does not admit the append. Halt with
`EXEMPT_DELTA_EXCEEDS_CLASS` and report the collision so the task can be given its own guard file.
Do **not** relax this exception to permit appends — the narrowing is deliberate.

If the root cause is still unclear after repeated attempts, or the task touches a risky subsystem, invoke **safety-modes** in `investigate-first` or `freeze-scope` mode before continuing.

#### Step 6: Verify Compilation

Run `go test -run=^$ -count=1 ./...` to confirm the fix compiles. If compilation fails, fix compilation errors before returning to the harness loop.

### Post-Loop Quality Gates

After the harness passes:

1. **Lint**: `golangci-lint run`
2. **Format**: `gofmt -l .`
   * If violations found: `gofmt -w .` and re-check
3. **Full test suite**: `go test ./...`
4. **Harness-exempt completion gate** (P-002.3 / P-002.4), for `harness-exempt` tasks only:
   * Screen `exempt_gate_cmd` for a Principle VII destructive pattern (P-002.5) before running it
     again. A match at this point is unexpected — Step 0 should already have rejected it — but
     still halts with `EXEMPT_COMMAND_DESTRUCTIVE` rather than executing.
   * Run `exempt_gate_cmd` once. It MUST exit 0, print the exact declared
     `EXEMPT_VERIFY_OK:{task_id}` marker, carry no other false-green signal, and match the task's
     declared evidence — the named `--- PASS:` guard count, the declared content assertions, or the
     declared evidence-manifest rows and scalars. A vacuous pass is `EXEMPT_EVIDENCE_MISMATCH`; an
     exit-0 run without the marker is `EXEMPT_MARKER_MISSING`. Report either and stop.
   * Confirm the changed-file set is a subset of the P-002.4 delta surface for `exempt_class`.
     Anything outside it is `EXEMPT_DELTA_EXCEEDS_CLASS`; report it and stop. Do not "fix" a
     class violation by editing the contract.

### Commit

If all quality gates pass:

1. Stage all changes
2. Create a conventional commit message referencing the task ID
3. Report success to the caller

## Behavioral Constraints

* No subagent spawning (leaf executor)
* Never modify an existing test file to reach green (tests are the specification). Creating the new test or evidence files a `verification-only` or `declaration-only` task names is the single narrow exception, bounded by Step 5
* Never weaken, delete, relax, rename the selector of, skip, or build-tag away an existing harness assertion to reach green — including on a `harness-exempt` task whose deliverable is a new green-step guard
* Never author a failing assertion for a `harness-exempt` task; a fabricated RED is a P-002.1 violation, not compliance
* Never begin the loop for a `harness-exempt` task without first observing `exempt_gate_cmd` fail (Step 0)
* Never treat an exit-0 run carrying a P-002.3 false-green signal — including a missing `EXEMPT_VERIFY_OK:{task_id}` marker on `exempt_gate_cmd` — as success
* Never execute an `exempt_gate_cmd` or `harness_owner_command` that the P-002.5 screen matches as destructive; report `EXEMPT_COMMAND_DESTRUCTIVE` and route it to Principle VII approval instead
* Maximum 5 attempts before circuit breaker trips (skill-managed exception; see `circuit-breaker.instructions.md`)
* Same-error recurrence at attempt 3+ triggers the universal circuit breaker
* Read coding standards once at task start; targeted re-read for unfamiliar modules
* One file change per tool call; broadcast after each write
* When the `agent-intercom` capability pack is installed, use intercom broadcasts for attempt milestones and file-write visibility

## Quality Criteria

* All harness tests pass
* No lint violations
* No format violations
* Full test suite passes
* Changes are scoped to the task requirements
* For a `harness-exempt` task: the pre-work probe was observed failing (marker absent), the
  completion gate passes non-vacuously with the exact marker present, both commands cleared the
  P-002.5 read-only screen before either ran, and the changed-file set stays inside the P-002.4
  class delta surface

## Model Routing

This skill operates at **Tier 2 (Standard)** — routine build loop execution and quality verification.

Generated by autoharness | Template: build-feature/SKILL.md.tmpl
