---
name: impl-plan
description: "Transform feature descriptions or requirements into structured implementation plans grounded in repo patterns and research. Use when the user says 'plan this', 'create a plan', 'how should we build', 'break this down', or when a brainstorm requirements document is ready for technical planning."
argument-hint: "source=.backlog/brainstorm/{file}.md or source=.backlog/research/{file}.md"
input:
  properties:
    source:
      type: string
      description: "Path to the source document to plan from. Accepted locations: .backlog/brainstorm/{file}.md (brainstorm requirements) or .backlog/research/{file}.md (research reports)."
  required:
    - source
---

# Create Implementation Plan

The `brainstorm` skill defines **WHAT** to build. The `impl-plan` skill defines **HOW** to build it. The `backlog-harvester` agent decomposes the plan into backlogit work items.

This skill produces a durable implementation plan. It does **not** implement code, run tests, or learn from execution-time results.

## Agent-Intercom Communication (NON-NEGOTIABLE)

Call `ping` at session start. If agent-intercom is reachable, broadcast at every step. If unreachable, warn the user that operator visibility is degraded.

| Event | Level | Message prefix |
|---|---|---|
| Session start | info | `[PLAN] Starting: {topic}` |
| Source doc found | info | `[PLAN] Using requirements doc: {path}` |
| Research phase | info | `[PLAN] Researching: {area}` |
| Learnings found | info | `[PLAN] Learnings researcher found {count} relevant solutions` |
| Plan section drafted | info | `[PLAN] Drafted: {section_name}` |
| Waiting for input | warning | `[WAIT] Blocked on user clarification: {question}` |
| Plan written | success | `[PLAN] Plan written: {file_path}` |
| Session complete | success | `[PLAN] Complete: {topic}` |

## Core Principles

1. **Use requirements as the source of truth** -- If `brainstorm` produced a requirements doc, build from it rather than re-inventing.
2. **Decisions, not code** -- Capture approach, boundaries, files, dependencies, risks, and test scenarios. Do not pre-write implementation code.
3. **Research before structuring** -- Explore the codebase and institutional learnings before finalizing the plan.
4. **Right-size the artifact** -- Small work gets a compact plan. Large work gets more structure.
5. **Separate planning from execution discovery** -- Resolve planning-time questions here. Defer execution-time unknowns to implementation.
6. **Keep the plan portable** -- The plan should work as a living document, review artifact, or backlog harvester input.
7. **Enforce task granularity** -- Every implementation unit must be scoped to roughly 2 hours of human-equivalent effort. Units that appear larger must be split. Units must not mix skill domains (e.g., Python code + documentation, database migration + API changes). Every unit must produce a verifiable state (passing test, successful build, or measurable output).

## Plan Quality Bar

Every plan must contain:

- A clear problem frame and scope boundary
- Concrete requirements traceability back to the request or origin document
- Exact file paths for the work being proposed
- Explicit test file paths for feature-bearing implementation units
- Decisions with rationale, not just tasks
- Existing patterns or code references to follow
- Specific test scenarios and verification outcomes
- Clear dependencies and sequencing
- **Granularity validation**: each implementation unit scoped to ≤2 hours of human effort
- **Width isolation**: each unit targets a single skill domain (code, docs, config, tests)
- **Atomic milestone**: each unit specifies a verifiable exit state (test pass, build pass, or measurable output)
- **Execution posture notes** per implementation unit

A plan is ready when an implementer can start confidently without needing the plan to write the code for them.

## Inputs

* `${input:source}`: (Required) Path to the source document to plan from. Accepted locations:
  - `.backlog/brainstorm/{filename}.md` — Requirements documents produced by the brainstorm skill
  - `.backlog/research/{filename}.md` — External research, evaluation reports, or design explorations

## Workflow

### Phase 0: Resume, Source, and Scope

#### 0.1 Resume Existing Plan Work

If the user references an existing plan file or there is an obvious recent matching plan in `.backlog/exec-plans/`:

- Read it
- Confirm whether to update in place or create new
- If updating, preserve completed checkboxes and revise only still-relevant sections

#### 0.2 Read Source Document

Read the source document at `${input:source}` in full.

1. Validate the file exists and is in an accepted location (`.backlog/brainstorm/` or `.backlog/research/`).
2. If the file does not exist, list available files in both directories and halt.
3. Determine the source type from the file path:
   - **Brainstorm**: Structured format with YAML frontmatter, `## Problem Frame`, `## Requirements`, `## Success Criteria`, `## Key Decisions`, `## Scope Boundaries`
   - **Research**: Free-form structure with H1/H2 sections, executive summary, proposed changes, evaluation criteria
4. Announce the source document: `broadcast` at `info` level: `[PLAN] Using source doc: ${input:source}`

#### 0.3 Use Source Document as Primary Input

1. Read it thoroughly
2. Announce it as the origin document for planning
3. Carry forward: problem frame, requirements, success criteria, scope boundaries, key decisions, dependencies, outstanding questions
4. Reference carried-forward decisions with `(see origin: {source-path})`
5. Do not silently omit source content

#### 0.4 Classify Outstanding Questions

If the origin doc has "Resolve Before Planning" questions:

- Review each before proceeding
- Reclassify technical/architectural questions as planning-owned
- Keep product behavior questions as true blockers
- Present blockers to user for resolution

### Phase 1: Research

**Codebase search**:

1. Use grep/glob to search for key concepts across the codebase
2. Identify affected modules and files
3. Search for related function and class definitions
4. Search for existing patterns that the implementation should follow
5. Fall back to broader file reads when needed

**Learnings check**: Invoke `learnings-researcher` as a subagent to search `.backlog/compound/` for relevant past solutions. Incorporate relevant learnings into the plan's decisions and caveats.

**Broadcast** research findings at each step.

### Phase 2: Structure the Plan

Write to `.backlog/exec-plans/{YYYY-MM-DD}-{slug}-plan.md`

```markdown
---
title: "{Feature Title}"
date: YYYY-MM-DD
origin: ".backlog/brainstorm/{slug}-requirements.md"
status: draft|reviewed|approved
---

# {Feature Title}

## Problem Frame

{Problem description and scope boundary}

## Requirements Trace

| # | Requirement | Origin |
|---|---|---|
| R1 | {requirement} | {origin doc reference or user request} |

## Scope Boundaries

### In Scope
{What this plan covers}

### Non-Goals
{What this plan explicitly excludes}

### Deferred to Implementation
{Questions the implementer must resolve during execution}

## Implementation Units

Each unit MUST be scoped to roughly 2 hours of human-equivalent effort. Use these
heuristics to evaluate size: fewer than 3 files modified, fewer than 5 functions
changed, fewer than 4 test scenarios. If a unit exceeds these heuristics, split it.
Each unit MUST target a single skill domain (do not mix Python code with documentation,
or database changes with API changes). Each unit MUST specify a verifiable exit state.

### Unit 1: {Title}

**Files:** {exact file paths}
**Test files:** {exact test file paths}
**Effort size:** small|medium — must not exceed "medium" (~2 hours human effort)
**Skill domain:** code|docs|config|tests — single domain per unit
**Execution note:** test-first|characterization-first|migration-first|spike
**Patterns to follow:** {links to existing code patterns in the codebase}
**Dependencies:** {other units this depends on}

**Approach:**
{Technical approach with rationale}

**Verification:**
{Specific, testable success criteria — must produce a verifiable state}

### Unit 2: ...

## Dependency Graph

{Sequencing of units with rationale}

## Decisions

| # | Decision | Rationale | Alternatives Rejected |
|---|---|---|---|
| D1 | {decision} | {why} | {what was rejected and why} |

## Risks and Caveats

{Known risks, gotchas from learnings-researcher, edge cases}

## Learnings Applied

{Solutions from .backlog/compound/ that informed this plan, with file paths}

## Standards Check

{Map proposed work against the project's coding standards and conventions; document any justified deviations}
```

### Phase 3: Complete

1. Confirm the plan file was written to `.backlog/exec-plans/{YYYY-MM-DD}-{slug}-plan.md`.
2. Return the plan file path to the caller.

When invoked standalone (not as a subagent of backlog-harvester), present next steps:

1. "Run backlog-harvester to decompose this plan into backlogit feature, task, and subtask items" (Recommended)
2. "Run plan-review to validate this plan with multi-persona review first"
3. "Revise specific sections"
