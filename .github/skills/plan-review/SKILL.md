---
name: plan-review
description: "Multi-model, multi-persona review gate for implementation plans. Validates architectural soundness, scope boundaries, coding standards compliance, and Python quality before the backlog harvester decomposes a plan into tasks."
argument-hint: "[path to plan file in .backlog/plans/]"
---

# Plan Review Gate

Validates implementation plans through multi-persona review before the backlog harvester decomposes them into tasks. This gate prevents flawed plans from generating flawed task hierarchies.

## Agent-Intercom Communication (NON-NEGOTIABLE)

Call `ping` at session start. If agent-intercom is reachable, broadcast at every step. If unreachable, warn the user that operator visibility is degraded.

| Event             | Level         | Message prefix                                                                         |
| ----------------- | ------------- | -------------------------------------------------------------------------------------- |
| Review start      | info          | `[PLAN-REVIEW] Starting review of: {plan_path}`                                        |
| Persona spawned   | info          | `[SPAWN] {persona_name} for plan review`                                               |
| Persona returned  | info          | `[RETURN] {persona_name}: {finding_count} findings`                                    |
| Merge complete    | info          | `[PLAN-REVIEW] Merged: {total_findings} findings ({p0} P0, {p1} P1, {p2} P2, {p3} P3)` |
| Gate decision     | success/error | `[PLAN-REVIEW] Gate: {PASS\|FAIL\|ADVISORY}`                                           |
| Waiting for input | warning       | `[WAIT] Blocked on user decision for P2 findings`                                      |
| Review written    | success       | `[PLAN-REVIEW] Review artifact: {file_path}`                                           |

## Subagent Depth Constraint

This skill spawns reviewer subagents. Those subagents are leaf executors and MUST NOT spawn their own subagents. Maximum depth: plan-review -> persona subagent (1 hop).

## Inputs

The user provides the path to a plan file in `.backlog/plans/` when invoking this skill.

If no path is provided, search `.backlog/plans/` for the most recent `*-plan.md` file and confirm with the user.

## Reviewer Personas

Spawn all 4 personas. Use different models when available to force genuine diversity of critique.

### Always-On Personas (same model as caller)

| Persona Agent                | Focus                                                                                                                                  |
| ---------------------------- | -------------------------------------------------------------------------------------------------------------------------------------- |
| **Constitution Reviewer**    | Map plan units against the project's core principles. Flag violations.                                                                 |
| **Python Quality Reviewer**  | Evaluate proposed type signatures, error handling patterns, module boundaries. Will the plan produce code that passes mypy and ruff?   |

### Cross-Model Personas (different model when available)

| Persona Agent               | Focus                                                                                     | Suggested Model                            |
| --------------------------- | ----------------------------------------------------------------------------------------- | ------------------------------------------ |
| **Architecture Strategist** | Cohesion, coupling, module boundaries, dependency chains. Are the dependencies realistic? | GPT-5.4 or Gemini                          |
| **Scope Boundary Auditor**  | Scope creep, YAGNI, unnecessary complexity, verification criteria completeness.           | GPT-5.4 Medium |

If cross-model invocation is not available, run all 4 with the caller's model. Multi-model is preferred but not blocking.

## Workflow

### Step 1: Load Plan and Context

1. Read the plan file from `.backlog/plans/`
2. If the plan references an origin document in `.backlog/brainstorm/`, read that too
3. Broadcast: `[PLAN-REVIEW] Starting review of: {plan_path}`

### Step 2: Spawn Reviewer Subagents

Spawn all 4 persona subagents. Each receives:

- The full plan content
- The origin requirements doc (if any)
- The project's coding standards and conventions (reference `.github/copilot-instructions.md`)
- Instructions to return structured JSON findings

Broadcast each spawn: `[SPAWN] {persona_name} for plan review`

### Step 3: Collect and Merge Findings

As each persona returns:

1. Broadcast: `[RETURN] {persona_name}: {finding_count} findings`
2. Collect all findings into a unified list
3. Deduplicate: merge findings that identify the same issue from different perspectives
4. Assign final severity (use the more conservative severity when personas disagree)
5. Assign action routing:
   - Plan revision needed -> `manual`
   - Advisory observation -> `advisory`

### Step 4: Gate Decision

| Condition             | Decision     | Action                                                                         |
| --------------------- | ------------ | ------------------------------------------------------------------------------ |
| Any P0 or P1 findings | **FAIL**     | Present findings to user. Plan must be revised before proceeding to harvester. |
| P2 findings only      | **ADVISORY** | Present findings to user. User decides: revise or proceed.                     |
| P3 findings only      | **PASS**     | Log findings as advisory. Proceed to harvester.                                |
| No findings           | **PASS**     | Plan is clean. Proceed to harvester.                                           |

Broadcast the gate decision.

### Step 5: Write Review Artifact

Write to `.backlog/reviews/{YYYY-MM-DD}-{slug}-plan-review.md`

```markdown
---
title: "Plan Review: {plan_title}"
date: YYYY-MM-DD
plan: "{plan_path}"
gate: pass|fail|advisory
reviewers: [constitution-reviewer, python-quality-reviewer, architecture-strategist, scope-boundary-auditor]
---

# Plan Review: {plan_title}

## Gate Decision: {PASS|FAIL|ADVISORY}

## Summary

{Total findings by severity and category}

## Findings

### P0 -- Critical (must fix before proceeding)

{Findings or "None"}

### P1 -- High (should fix before proceeding)

{Findings or "None"}

### P2 -- Moderate (user discretion)

{Findings or "None"}

### P3 -- Low (advisory)

{Findings or "None"}

## Reviewer Attribution

| Finding | Reviewer | Model |
|---|---|---|
| {finding_ref} | {persona_name} | {model_used} |

## Next Steps

{Based on gate decision: revise plan, proceed to harvester, or user decides}
```

Broadcast the file path when written.

### Step 6: Present Results

- On **FAIL**: Present P0/P1 findings. Recommend specific plan revisions.
- On **ADVISORY**: Present P2 findings. Ask user: "Revise the plan, or proceed to the backlog harvester?"
- On **PASS**: Report clean pass. Suggest: "Run backlog-harvester to decompose this plan into tasks."
