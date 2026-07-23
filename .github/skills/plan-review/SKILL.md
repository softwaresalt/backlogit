---
name: plan-review
description: "Multi-persona review gate for implementation plans. Validates architectural soundness, scope boundaries, and coding standards compliance before the harvest skill decomposes a plan into work items."
argument-hint: "[path to plan file in docs/exec-plans/]"
---

# Plan Review Gate

Validates implementation plans through multi-persona review before the harvest skill decomposes them into work items. This gate prevents flawed plans from generating flawed work hierarchies.

## Subagent Depth Constraint

This skill spawns reviewer subagents. Those subagents are leaf executors and MUST NOT spawn their own subagents. Maximum depth: plan-review skill → persona subagent (1 hop).

## Severity Scale

| Level | Meaning | Gate action |
|---|---|---|
| **P0** | Plan will produce unshippable or unsafe code (missing security, broken contracts, impossible scope) | Block harvest |
| **P1** | Plan has a high-impact gap that will cause significant rework (missing requirements, wrong decomposition, absent verification) | Block harvest |
| **P2** | Plan has a moderate gap (edge case coverage, missing test scenario, suboptimal decomposition) | Record as backlog follow-up |
| **P3** | Plan has a minor improvement opportunity (wording, optional optimization) | Advisory |

Use the same severity conventions as the code review skill adapted for plan
artifacts rather than code diffs.

## Agent-Intercom Communication

When the `agent-intercom` capability pack is installed, call `ping` at session start. If reachable, broadcast at every step. If unreachable, warn the operator that visibility is degraded.

| Event | Level | Message prefix |
|---|---|---|
| Review start | info | `[PLAN-REVIEW] Starting review of: {plan_path}` |
| Persona spawned | info | `[SPAWN] {persona_name} for plan review` |
| Persona returned | info | `[RETURN] {persona_name}: {finding_count} findings` |
| Merge complete | info | `[PLAN-REVIEW] Merged: {total_findings} findings ({p0} P0, {p1} P1, {p2} P2, {p3} P3)` |
| Gate decision | success/error | `[PLAN-REVIEW] Gate: {PASS\|FAIL\|ADVISORY}` |
| Review appended | success | `[PLAN-REVIEW] Review appended to: {plan_path}` |

## Inputs

* `plan_path`: (Required) Path to the plan file (`docs/exec-plans/{YYYY-MM-DD}-{slug}-plan.md`).

If no path is provided, search `docs/exec-plans/` for the most recent `*-plan.md` file and confirm with the operator.

## Output

Review findings are **appended to the plan file** as a `## Plan Review` section,
not written as a separate file. The plan-review skill produces a gate decision
(`PASS`, `ADVISORY`, or `FAIL`) that is recorded in the appended section. When
the compact-context skill later consolidates the plan, it merges the plan and
appended reviews into a decided-plan.

## Reviewer Personas

Spawn all always-on personas and any triggered cross-model personas. Use different
models when available to force genuine diversity of critique.

### Always-On Personas (same model as caller)

| Persona Subagent | Focus |
|---|---|
| **Constitution Reviewer** | Map plan units against constitutional principles. Flag violations. |
| **Go Reviewer** | Evaluate proposed type signatures, error handling patterns, package boundaries, and verification steps. |
| **Scope Boundary Auditor** | Verify units stay within declared scope. Detect scope creep, YAGNI, unnecessary complexity. |
| **Learnings Researcher** | Search `docs/compound/` for prior solutions relevant to the plan's scope. Report P0 if the plan contradicts a known past resolution; P1 if it ignores a highly relevant prior solution. |

### Cross-Model Personas (different model when available)

| Persona Subagent | Focus | Suggested Model |
|---|---|---|
| **Architecture Strategist** | Cohesion, coupling, module boundaries, dependency chains. | Different from caller |
| **Agent-Native Parity Reviewer** | Plans that expose MCP tools, agent-facing actions, or user/agent parity-sensitive workflows. | Different from caller |
| **Security Lens Reviewer** (`security-lens-reviewer.agent.md`) | Plans that touch auth/authz systems, API surfaces, sensitive data stores, external integrations, or secrets management. | Different from caller |

If cross-model invocation is not available, run all personas with the caller's model. Multi-model is preferred but not blocking.

## Dispatch Capability and Declared Degradation

Reviewer personas are normally dispatched as independent sub-agents. Sub-agent
dispatch capability varies by environment (available via the Copilot CLI `task`
tool and VS Code Copilot agents; absent in some others). This gate is
**capability-aware** so it is satisfiable in every environment and is never
silently skipped.

**Relationship to P-012.** P-012 ("Tool Availability and Declared Degradation",
`.github/policies/workflow-policies.md`) governs *backlog-registry* tools and
their registry-declared `cli_command` fallbacks. Reviewer sub-agent dispatch is
**not** a backlog-registry tool, so P-012's registry mechanism does not model it
directly. This section applies P-012's **declared-degradation principle** —
probe before relying on a capability, declare any degraded mode explicitly, and
never fall back silently — to sub-agent dispatch, and defines the permitted
fallback and terminal states locally. Formalizing this as an explicit P-012
capability clause is a recommended follow-up (see the decision artifact).

**Persona → definition manifest.** Both dispatch modes resolve the same persona
definitions. All personas live under `.github/agents/review/` **except** the
Learnings Researcher, whose definition is
`.github/agents/research/learnings-researcher.agent.md`. Use this single manifest
for both real dispatch and the sequential rubric pass so no required lens is
dropped.

**Probe.** Before spawning any reviewer (at the start of Step 2, before the plan
is dispatched to personas), determine whether sub-agent dispatch is available and
record `dispatch_mode`:

| Capability | `dispatch_mode` | Action |
|---|---|---|
| Sub-agent dispatch available | `multi-agent-dispatch` | Dispatch the real persona sub-agents from the manifest. Log `TOOL_OK: reviewer-subagent-dispatch`. Preferred, full-fidelity path. |
| Sub-agent dispatch unavailable | `single-agent-declared-degradation` | Run a **single-agent persona pass**: sequentially apply each manifest persona's definition as a review rubric, one lens at a time, over the full plan. Log `TOOL_DEGRADED: reviewer-subagent-dispatch — single-agent persona pass`. |

**Terminal states (no partial gate).** Coverage of every *selected* persona is
mandatory in both modes:

* `multi-agent-dispatch` yields a valid gate result **only when every selected
  persona completes and returns findings.** If any dispatched persona fails to
  start or return mid-gate, do **not** issue a PASS/ADVISORY on partial coverage
  and do **not** keep the `multi-agent-dispatch` label. Record the dispatch
  failure, then run a **complete** sequential rubric pass over **all** selected
  personas and emit `single-agent-declared-degradation`.
* `single-agent-declared-degradation` yields a valid gate result **only when the
  rubric pass covers every selected persona.**
* If neither a complete dispatch nor a complete rubric pass can be performed,
  **halt** with `TOOL_UNAVAILABLE: reviewer-subagent-dispatch` rather than
  issuing a gate decision on incomplete coverage.

Rules:

* **Prefer real dispatch** wherever the environment supports it — the
  single-agent persona pass is a fallback, not a default.
* The single-agent persona pass is a **legitimate, sanctioned** gate outcome
  when declared and complete. It is neither a silent skip nor a per-plan operator
  waiver. It trades away independent-agent execution (and any model diversity) —
  a stronger degradation than the "multi-model preferred but not blocking" axis —
  so the `dispatch_mode` record makes that fidelity level explicit.
* **Silently skipping the gate — issuing a gate decision with no `dispatch_mode`
  record — is a policy violation** of P-012's declared-degradation principle (log
  `POLICY_VIOLATION: P-012` via P-005 telemetry). The honesty of the degradation
  record is the gate's integrity guarantee.
* P0/P1 blocking semantics are identical in both modes.

## Workflow

### Step 1: Load and Parse Plan

1. Read the plan file from `docs/exec-plans/`.
2. Extract implementation units, dependency graph, decisions, risks, hardening signals, and whether a `## Plan Hardening` section is present.
3. Extract the `## Constitution Check` section and its concluding verdict. A conforming plan ends the section with exactly `Constitution Check: pass` or `Constitution Check: documented-deviations`. Treat a missing section or a missing/unrecognized verdict as a governance gap (see the Step 4 gate row): the absence is fail-safe, mirroring how P-006 treats a missing `Requires plan hardening` field as `yes`.
4. When `strict-safety` is enabled and the plan contains a `## Plan Hardening` section, also extract any `ProposedAction` / `ActionRisk` entries.
5. If the plan references an origin document, read that too for context.
6. Broadcast: `[PLAN-REVIEW] Starting review of: {plan_path}`

### Step 2: Spawn Reviewer Subagents

First determine `dispatch_mode` per the **Dispatch Capability and Declared
Degradation** section above (probe before spawning) and log the corresponding
`TOOL_OK` / `TOOL_DEGRADED` line. In `multi-agent-dispatch` mode, spawn the
manifest personas as sub-agents. In `single-agent-declared-degradation` mode,
apply the same manifest definitions as sequential single-agent rubric passes over
the plan. Coverage of every selected persona is mandatory; on a mid-gate dispatch
failure, fall back to a complete rubric pass per the terminal-states rule (do not
merge partial dispatch coverage into a full-fidelity decision).

Spawn (or apply) all always-on personas plus the cross-model personas whose
trigger conditions are met. Each receives:

- The full plan content
- The origin requirements doc (if any)
- The project's coding standards and conventions (reference `.github/instructions/constitution.instructions.md`)
- Instructions to return structured findings

Trigger conditions for cross-model personas:
* **Architecture Strategist**: always triggered
* **Agent-Native Parity Reviewer**: triggered when the plan exposes MCP tools, agent-facing actions, or user/agent parity-sensitive workflows
* **Security Lens Reviewer**: triggered when the plan touches authentication or authorization systems, API surfaces, sensitive data stores, external integrations crossing trust boundaries, or secrets and credentials management

Broadcast each spawn in `multi-agent-dispatch` mode. In
`single-agent-declared-degradation` mode no sub-agents are spawned, so emit
persona-pass start/complete events (e.g. `[PERSONA-PASS] {persona}` /
`[PERSONA-DONE] {persona}`) instead of `[SPAWN]` / `[RETURN]`.

### Step 3: Collect and Merge Findings

As each persona returns (or each rubric pass completes):

1. Broadcast the return with finding count
2. Collect all findings into a unified list
3. Deduplicate: merge findings that identify the same issue from different perspectives
4. Assign final severity (use the more conservative severity when personas disagree)

### Step 4: Gate Decision

| Condition | Decision | Action |
|---|---|---|
| Plan shows hardening signals but lacks plan hardening or equivalent high-risk detail | **FAIL** | Return the plan to `plan-harden` or manual revision before `harvest`. |
| Strict-safety enabled, hardening present, but risky actions lack `ProposedAction` / `ActionRisk` classification | **FAIL** | Plans with hardening signals must classify risky actions explicitly when strict-safety is active. |
| `## Constitution Check` section or its verdict is missing or unrecognized (not `Constitution Check: pass` or `Constitution Check: documented-deviations`) | **FAIL** | Return the plan to the planner to add or correct the Constitution Check verdict before `harvest`. Absence is fail-safe, mirroring the P-006 plan-hardening default. |
| Any P0 or P1 findings | **FAIL** | Present findings to user. Plan must be revised before proceeding to `harvest`. |
| P2 findings only | **ADVISORY** | Present findings to user. User decides: revise or proceed. |
| P3 findings only or none | **PASS** | Log findings as advisory. Proceed to `harvest`. |

Broadcast the gate decision.

### Step 5: Append Review to Plan

Append a `## Plan Review` section to the plan file with:

* The `dispatch_mode` (`multi-agent-dispatch` or
  `single-agent-declared-degradation`) and, in the degraded case, the
  `TOOL_DEGRADED: reviewer-subagent-dispatch` declaration (P-012
  declared-degradation principle). A gate decision recorded with no
  `dispatch_mode` is a policy violation.
* Gate decision and rationale
* Whether plan hardening was required and whether that requirement was satisfied
* All findings organized by severity
* Specific recommendations for addressing P0/P1 issues
* Acknowledgment of P2/P3 items for awareness
* Runtime verification and operational closure gaps called out explicitly when missing

The review is appended (not written as a separate file) so that the plan and its
review travel together as a single artifact. The compact-context skill later
consolidates this into a decided-plan.

## Quality Criteria

* Every implementation unit is reviewed by at least the always-on personas
* The `dispatch_mode` is probed and recorded, every selected persona is covered,
  and any degraded mode is declared per P-012's declared-degradation principle and
  never silently skipped
* The gate decision correctly reflects finding severities
* Findings include actionable recommendations
* The review is appended to the plan before the gate decision is communicated
* Plans with hardening signals are failed when hardening is missing or materially incomplete
* Plans missing a recognized Constitution Check verdict are failed structurally at the gate, independent of persona findings
* Plans that touch runtime surfaces are checked for verification and closure readiness


## Model Routing

This skill operates at **Tier 2 (Standard)** — plan review coordination and finding assembly.

Generated by autoharness | Template: plan-review/SKILL.md.tmpl
