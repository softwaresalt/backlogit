---
name: harness-architect
description: "Scaffolds compilable but failing Go test harnesses for backlogit tasks"
argument-hint: "feature=F015 tasks=T001,T002"
input:
  properties:
    feature:
      type: string
      description: "Feature ID to scaffold harnesses for"
    tasks:
      type: string
      description: "Comma-separated task IDs to scaffold (optional, defaults to all ready tasks)"
  required:
    - feature
---

# Harness Architect Skill

Scaffold strict Go test harnesses for a feature's ready work items. The output
must compile cleanly, fail for the intended not-yet-implemented behavior, and
leave clear harness commands for downstream build execution.

## Purpose

Use this skill when a shipment or feature needs executable test boundaries
before implementation starts. The skill prepares the red phase and stops there.
It does not implement production logic.

## Inputs

* `${input:feature}`: (Required) Feature ID such as `F015`
* `${input:tasks}`: (Optional) Comma-separated task IDs to scaffold. When
  omitted, use all ready tasks under the feature

## Workflow

### Step 1: Claim the ready task set

1. Load the feature and its ready descendants through backlogit-native queue or
   query operations.
2. If `${input:tasks}` is present, restrict the scope to that explicit task set.
3. Exclude blocked, done, or otherwise non-ready work items.
4. Preserve the feature-to-task mapping so each harness can be traced back to
   the correct backlogit item.

### Step 2: Read task intent

1. Read each selected task's title, description, acceptance criteria, and file
   references.
2. Pull in feature-level acceptance criteria when task text depends on broader
   feature behavior.
3. Translate acceptance criteria into named test scenarios before writing code.
4. Identify the correct package, test tier, and affected files for each harness.

### Step 3: Generate failing harness skeletons

1. Create `_test.go` files that express the task intent as compilable Go tests.
2. Prefer table-driven tests when the task describes multiple scenarios.
3. Create matching production stubs or structural helpers with
   `panic("not implemented: ...")` bodies so the module compiles while the tests
   still fail for the intended reason.
4. Keep signatures, types, and package names aligned with the current codebase.

## File Placement Rules

Write harness files into the package that matches the work item's scope:

* colocated unit harnesses under `internal\...\*_test.go` when the task targets
  package-local behavior
* integration harnesses under `tests\integration\` when the task spans packages
  or runtime boundaries
* contract harnesses under `tests\contract\` when the task defines CLI, MCP, or
  schema behavior

Write any companion stub files into the production package that the tests
exercise. Do not place scaffolding in unrelated packages.

## Verification

After scaffolding:

1. Run `go build ./...` to confirm the repository still compiles.
2. Confirm the new harness imports successfully.
3. Confirm the harness remains in the red phase because of the intentional
   panicking stubs or other not-implemented markers.
4. Report the generated test files, stub files, and harness commands so the
   downstream build loop can use them directly.

## Completion Criteria

The skill is complete only when the selected tasks have:

* harness files in the correct packages
* structural stubs with intentional not-implemented behavior
* a successful `go build ./...` result after scaffolding
* clear mapping from backlogit task to harness command
