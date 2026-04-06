---
name: safety-modes
description: "Interactive safety workflows for elevated-risk work including careful, freeze-scope, and investigate-first modes"
argument-hint: "mode={careful|freeze-scope|investigate-first} [scope=...] [context=...]"
---

# Safety Modes

Use an explicit safety mode before risky work. This slows the workflow down on purpose so the next step stays legible and bounded.

## Inputs

* `mode`: Required. One of `careful`, `freeze-scope`, or `investigate-first`.
* `scope`: Optional directory or subsystem boundary.
* `context`: Optional description of the risky task.

## Output

Produce a structured safety checklist. When useful, write it to `docs/closure/{YYYY-MM-DD}-{slug}-safety-check.md`.

## Required protocol

### Step 1: Classify the risk

Identify whether the main risk is destructive action, production impact, scope creep, root-cause uncertainty, or data/security loss.

### Step 2: Enter the requested mode

#### `careful`

1. Enumerate risky actions.
2. Define rollback or backup strategy.
3. Separate non-destructive and destructive steps.
4. Require explicit approval before destructive actions.

#### `freeze-scope`

1. Declare the allowed boundary.
2. List the files or directories inside it.
3. Refuse edits outside the boundary unless scope expands.
4. Re-state the boundary before risky edits.

#### `investigate-first`

1. Gather evidence before changing anything.
2. Produce at least one explicit root-cause hypothesis.
3. Separate evidence from assumptions.
4. Do not apply a fix until the hypothesis is testable.

### Step 3: Produce the checklist

Include:

* active mode
* declared boundary
* actions allowed immediately
* actions requiring approval
* exit condition for leaving the mode
