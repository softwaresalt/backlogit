---
name: plan-review
description: "Multi-model, multi-persona review gate for implementation plans. Validates architectural soundness, scope boundaries, and coding standards compliance before the harvest skill decomposes a plan into backlogit work items."
argument-hint: "[path to plan file in docs/exec-plans/]"
---

# Plan Review Gate

Validates implementation plans through multi-persona review before the harvest skill decomposes them into backlogit work items. This gate prevents flawed plans from generating flawed work hierarchies.

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

The user provides the path to a plan file in `docs/exec-plans/` when invoking this skill.

If no path is provided, search `docs/exec-plans/` for the most recent `*-plan.md` file and confirm with the user.

## Reviewer Personas

Spawn all always-on personas and any triggered cross-model personas. Use different
models when available to force genuine diversity of critique.

### Always-On Personas (same model as caller)

| Persona Agent                | Focus                                                                                                                                  |
| ---------------------------- | -------------------------------------------------------------------------------------------------------------------------------------- |
| **Constitution Reviewer**    | Map plan units against the project's core principles. Flag violations.                                                                 |
| **Go Quality Reviewer**      | Evaluate proposed type signatures, error handling patterns, package boundaries, and verification steps. Will the plan produce code that passes gofmt, golangci-lint, go vet, and go test? |
| **Scope Boundary Auditor**   | Scope creep, YAGNI, unnecessary complexity, verification criteria completeness.           |
| **Learnings Researcher**     | Confirm the plan is not ignoring relevant prior solutions.                                                                             |

### Cross-Model Personas (different model when available)

| Persona Agent               | Focus                                                                                     | Suggested Model                            |
| --------------------------- | ----------------------------------------------------------------------------------------- | ------------------------------------------ |
| **Architecture Strategist** | Cohesion, coupling, module boundaries, dependency chains. Are the dependencies realistic? | GPT-5.4 or Gemini                          |
| **Agent-Native Parity Reviewer** | Plans that expose MCP tools, agent-facing actions, or user/agent parity-sensitive workflows. | GPT-5.4 Medium |

If cross-model invocation is not available, run all personas with the caller's model. Multi-model is preferred but not blocking.

## Workflow

### Step 1: Load Plan and Context

1. Read the plan file from `docs/exec-plans/`
2. Extract implementation units, dependency graph, decisions, risks, hardening signals, and whether a `## Plan Hardening` section is present.
3. When `strict-safety` is enabled and the plan contains a `## Plan Hardening` section, also extract any `ProposedAction` / `ActionRisk` entries.
4. If the plan references an origin document in `.backlogit/queue/` or `docs/research/`, read that too
5. Broadcast: `[PLAN-REVIEW] Starting review of: {plan_path}`

### Step 2: Spawn Reviewer Subagents

Spawn all always-on personas plus the cross-model personas whose trigger
conditions are met. Each receives:

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
| Plan shows hardening signals but lacks plan hardening or equivalent high-risk detail | **FAIL** | Return the plan to `plan-harden` or manual revision before `harvest`. |
| Strict-safety enabled, hardening present, but risky actions lack `ProposedAction` / `ActionRisk` classification | **FAIL** | Plans with hardening signals must classify risky actions explicitly when strict-safety is active. |
| Any P0 or P1 findings | **FAIL**     | Present findings to user. Plan must be revised before proceeding to `harvest`. |
| P2 findings only      | **ADVISORY** | Present findings to user. User decides: revise or proceed.                     |
| P3 findings only      | **PASS**     | Log findings as advisory. Proceed to `harvest`.                                |
| No findings           | **PASS**     | Plan is clean. Proceed to `harvest`.                                           |

Broadcast the gate decision.

### Step 5: Append Review to Plan

Review findings are **appended to the plan file** as a `## Plan Review` section,
not written as a separate file. The plan-review skill produces a gate decision
(`PASS`, `ADVISORY`, or `FAIL`) that is recorded in the appended section.

Append a `## Plan Review` section to the plan file with:

* Gate decision and rationale
* Whether plan hardening was required and whether that requirement was satisfied
* All findings organized by severity
* Specific recommendations for addressing P0/P1 issues
* Acknowledgment of P2/P3 items for awareness
* Runtime verification and operational closure gaps called out explicitly when missing

A separate tracking artifact may also be written to `.copilot-tracking/plan-review/` for compaction-friendly reference.

```markdown
---
title: "Plan Review: {plan_title}"
date: YYYY-MM-DD
plan: "{plan_path}"
gate: pass|fail|advisory
reviewers: [constitution-reviewer, go-quality-reviewer, scope-boundary-auditor, learnings-researcher, architecture-strategist, agent-native-parity-reviewer]
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

{Based on gate decision: revise plan, proceed to `harvest`, or let the user decide}
```

Broadcast the file path when written.

### Step 6: Present Results

- On **FAIL**: Present P0/P1 findings. Recommend specific plan revisions.
- On **ADVISORY**: Present P2 findings. Ask user: "Revise the plan, or proceed to harvest?"
- On **PASS**: Report clean pass. Suggest: "Run harvest to decompose this plan into backlogit work items."
