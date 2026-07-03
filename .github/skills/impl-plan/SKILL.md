---
description: "Transform feature or chore descriptions and requirements into structured implementation plans grounded in repo patterns and research"
---

## Implementation Plan

Transform WHAT (requirements document) into HOW (implementation plan). Produces a structured plan that the stage agent decomposes into tasks via the harvest skill.

## When to Use

Invoke when a deliberation outcome or spike findings document is ready for technical planning. The output feeds into `plan-harden` when the work is risky, then into the `plan-review` skill for validation before the stage agent harvests it into backlog work.

## Inputs

* `source`: (Required) Path to source document (`docs/decisions/{file}.md` for deliberation outcomes or spike findings).

## Output

A plan file at `docs/exec-plans/{YYYY-MM-DD}-{slug}-plan.md`.

## Required Protocol

When the `agent-intercom` capability pack is installed, follow
`.github/instructions/agent-intercom.instructions.md`: establish heartbeat / ping visibility at the
start of planning, broadcast major planning milestones, and use the intercom clarification flow
when unresolved source ambiguity or planning trade-offs require operator input.

When the `agent-engram` capability pack is installed, follow
`.github/instructions/agent-engram.instructions.md`: verify the engram search surface before relying
on indexed discovery, and prefer engram-first lookup while researching the codebase.

### Phase 1: Understand the Source

1. Read and parse the source document
2. Extract: problem frame, requirements, success criteria, scope boundaries
3. Identify any outstanding questions that need resolution before planning

### Phase 2: Research the Codebase

Search the learnings library (`docs/compound/`) for relevant past solutions BEFORE deeper repo analysis. Treat retrieval as mandatory pre-planning context, not an optional fallback.

Use workspace search tools to understand:

* Existing patterns and conventions in the codebase
* Modules and symbols relevant to the feature or chore
* Test patterns established in the project
* Dependencies and integration points

When the `agent-engram` capability pack is installed, prefer `unified_search` for broad discovery,
`list_symbols` for inventory, `map_code` for caller/callee context, and `impact_analysis` before
manual caller tracing.

### Phase 3: Structure the Plan

Produce a plan with these sections:

#### Plan Frontmatter Contract (REQUIRED)

Emit a docline frontmatter block as the **first block** of the plan file. Plans live under `docs/exec-plans/**`, which the CI "Docline frontmatter gate" (`make docs-lint`) lints on every PR, so a non-compliant block blocks the downstream Ship PR — this is the exact 075-S regression this contract prevents (see `docs/compound/2026-06-26-docline-frontmatter-contract.md` and the authoring guide `docs/docline-frontmatter-authoring-guide.md`).

The block MUST set these three **gate-required** fields (authoring profile):

* `doc_type: plan` — the closed-vocabulary value for `docs/exec-plans/**`. Never `exec-plan` (a natural but **invalid** guess that fails the gate with `unknown_doc_type`).
* `title:` — a top-level, single-quoted plan title.
* `source:` — a top-level field equal to the plan's own repo-relative POSIX path (e.g. `docs/exec-plans/{YYYY-MM-DD}-{slug}-plan.md`).

For green-reference parity (**recommended, not gate-required**), also include `description`, `schema_version`, and `chunk_strategy`, matching the field shape of the green reference plan `docs/exec-plans/2026-07-01-doctor-target-nil-headerdef-hardening-plan.md`. Use placeholder values below as a copyable template — always set `source` to the plan's **own** path. The self-lint (authoring profile) only checks that `source` is **present** (non-empty); it does not validate the format or that it matches the file path, so a stale copied `source` would pass silently:

```yaml
---
chunk_strategy: h1-h2-h3
description: 'One-sentence summary of the plan.'
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/{YYYY-MM-DD}-{slug}-plan.md
title: 'Concise plan title'
---
```

**YAML pitfall (single-quote to avoid silent truncation):** single-quote any scalar containing `#`, `:`, or a leading special character. Plan titles and descriptions routinely cite PR numbers (`#164`) and ratios, so an unquoted `#` or `:` truncates the value and can silently drop a required field.

**Optional deterministic derivation:** instead of hand-authoring `source`/`doc_type`, derive them with `backlogit docs migrate`. Run the dry-run/plan first to review the diff, then `backlogit docs migrate --apply --yes --path docs/exec-plans/<file>` to write. Prefer the diff-first flow: `--apply --yes` is an in-place overwrite (git-tracked, so revertible), but standing guidance shows the operator the diff before writing (Principle VII).

#### Problem Frame

Restate the problem in technical terms, referencing specific code paths and modules.

#### Requirements Trace

Map each requirement from the source document to specific implementation actions.

#### Implementation Units

Break the work into discrete units, each following the granularity constraints:

* **2-Hour Rule**: Fewer than 3 files, fewer than 5 functions, fewer than 4 test scenarios
* **Width Isolation**: Single domain per unit (code OR docs OR tests OR config)
* **Atomic Milestone**: Each unit produces a verifiable outcome

For each unit, specify:

* What changes are needed
* Which files are affected
* What tests verify the change
* Execution posture (test-first, characterization-first, migration-first, spike)

#### Dependency Graph

Identify which units depend on others. Sequence them to minimize blocking.

#### Decisions and Rationale

Document key technical decisions with the reasoning behind each choice.

#### Risks and Caveats

Identify potential issues, unknowns, and mitigation strategies.

#### Plan Hardening Signals (REQUIRED)

Every plan MUST include this section. Explicitly record whether the plan needs
hardening before review. Mark each signal as present or absent and include a
short justification:

* public API, schema, or contract change
* security, auth, permission, or compliance-sensitive behavior
* migration, backfill, destructive data/config action, or irreversible step
* external integration, operator checkpoint, or external dependency
* high runtime, rollout, or rollback risk

Conclude with `Requires plan hardening: yes|no`. This conclusion is mandatory —
P-006 treats its absence as `yes` (fail-safe). Even trivial plans must include
`Requires plan hardening: no` to pass the gate without unnecessary hardening.

#### Runtime Verification and Closure

For each implementation unit, identify:

* Whether it changes a runtime surface (CLI, API, browser UI, background jobs)
* What runtime verification should prove before the work is considered absorbed
* What operational closure artifact should exist (monitoring checklist, rollback trigger, ownership, validation window)

When one or more hardening signals are present, seed enough detail that the
downstream `plan-harden` step can tighten the plan instead of inventing safety,
verification, or rollback expectations from scratch.

### Phase 4: Self-Lint the Frontmatter (MANDATORY)

After writing the plan, verify its frontmatter against the docline contract
**before the plan is considered complete**:

1. Run the docline linter via the **same entrypoint CI uses**. `make docs-lint`
   runs the repo-wide gate (`go run ./cmd/backlogit docs lint`, no arguments). To
   scope the check to just the new plan, call the same source entrypoint directly:
   `go run ./cmd/backlogit docs lint --path docs/exec-plans/<file>`.
2. Confirm the result is `valid` with `0 violations`.
3. Treat **any** violation as a blocker: fix the frontmatter in place (or run
   `backlogit docs migrate` diff-first, then `--apply --yes --path`) and re-run
   the linter until it reports 0 violations.

Invoke the **source** entrypoint (`go run ./cmd/backlogit ...` / `make docs-lint`),
not a possibly-stale installed `backlogit` binary. The source entrypoint
guarantees the self-lint agrees with the CI Docline gate and cannot pass locally
while CI fails.

## Quality Criteria

* Every requirement from the source document maps to at least one implementation unit
* Every unit satisfies the 2-hour rule, width isolation, and atomic milestone constraints
* Dependency graph has no cycles
* Decisions include rationale (not just the choice)
* Risks identify mitigations
* Relevant prior learnings are surfaced before planning concludes
* Plans record whether `plan-harden` is required before review — this field is mandatory, not optional
* Plans include runtime verification and closure expectations for changed runtime surfaces
* The plan file opens with a docline frontmatter block setting `doc_type: plan` plus top-level `title` and `source` (the gate-required contract for `docs/exec-plans/**`)
* The authored plan passes `backlogit docs lint` (`make docs-lint`) with 0 violations before it is handed off to review or harvest

## Model Routing

This skill operates at **Tier 3 (Frontier)** — technical planning and codebase analysis require deep reasoning.

Generated by autoharness | Template: impl-plan/SKILL.md.tmpl
