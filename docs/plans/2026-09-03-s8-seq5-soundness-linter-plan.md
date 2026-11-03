---
chunk_strategy: h1-h2-h3
description: "Execution plan for S8: Sequence 5/7 plan and work-item soundness linter"
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-09-03-s8-seq5-soundness-linter-plan.md
title: "S8 Execution Plan — Sequence 5/7 Plan & Work-Item Soundness Linter"
---

# S8 Execution Plan — Plan & Work-Item Soundness Linter (Seq 5/7)

**Covering feature**: Plan and work-item soundness linter
**Stash member**: 4863B04B (high)
**Tier**: feature (shipment sequence S8)

## Problem Frame

Unsound decomposition (over-budget units, multi-responsibility tasks, unresolvable
references, missing verification surfaces, dependency inconsistency, duplicate or
contradictory work, un-verifiable units) creates expensive review-fix churn.
Validate harvested plans, features, tasks, and subtasks before shipment assembly.

## Constitution Check

| Principle | Compliance |
|-----------|-----------|
| I. Safety-First Go | Go 1.24; wrapped errors |
| II. Test-First (P-002) | rule declaration -> RED -> GREEN per unit |
| III. Workspace Isolation | reads workspace backlog + plans |
| IV. CLI Containment | n/a |
| V. Observability | lint report structured |
| VI. Single Responsibility | one rule engine, declarative budgets |
| VII. Destructive Approval | none |
| VIII. Safety Modes | fail-closed on unverifiable unit |
| IX. Git-Friendly | text config |
| X. Context Efficiency | n/a |
| XI. Merge Commits | P-009 by Ship |

Constitution Check: pass

## Implementation Units

### U1 — Soundness rule engine + configurable budgets
* Scope: rule engine enforcing configured file/function/scenario budgets, one-responsibility-per-unit, resolvable references, and explicit verification surfaces over harvested plans/features/tasks/subtasks.
* Acceptance: over-budget and multi-responsibility units are flagged; a sound plan passes; budgets are configurable.

### U2 — Consistency and duplication checks
* Scope: dependency consistency, unique-or-intentionally-related titles, canonical procedures, no contradiction with originating decisions or sibling tasks, impossible-test detection, duplicated-work detection, missing-artifact detection, and independent-verifiability. The truthful decomposition is eight independently verifiable tasks: `160.002-T` dependency consistency, `160.005-T` title uniqueness, `160.006-T` canonical procedures, `160.007-T` decision-contradiction, `160.003-T` impossible-test, `160.008-T` duplicate-work, `160.009-T` missing-artifact, and `160.010-T` independent-verifiability (note: `160.004-T` is the U3 gate-integration task, not a check class). They share the rule-engine scaffolding from U1. Reference/anchor resolution shares one resolver with S7 U3.
* Acceptance: each defect class flags a seeded case and passes clean input; completion evidence maps to the eight task IDs above, one independently verifiable task per check class.

### U3 — Pre-assembly gate integration (report-only first)
* Scope: expose the soundness engine as a library and wire it as a pre-shipment-assembly gate so unsound decomposition halts before a shipment is built. Ships REPORT-ONLY first with per-rule enable flags before it fail-closed blocks assembly. The same engine library is invoked by the S10 "plan/task soundness" DAG node executor (single implementation, two call sites) to avoid divergent enforcement paths.
* Acceptance: in enforce mode, assembling a shipment over an unsound plan is blocked with specific findings; in report-only mode it warns without blocking; a sound plan proceeds; the S10 node and this gate call the same engine. Depends on U2.

## Dependency Graph

U1 -> U2 -> U3. Single domain per unit.

## Runtime Verification and Closure

Verification surface: linter run over seeded sound/unsound plans. Closure: gate
integrated and green in CI. No product behavior change.

## Plan Hardening

| ProposedAction | ActionRisk | Mitigation |
|---|---|---|
| Fail-closed pre-assembly gate that blocks shipment assembly | High — a misconfigured rule could block all assembly | Ship report-only first with per-rule enable flags; fail-closed only after report-only validation; specific findings on block |
| Single engine, two call sites (gate + S10 node) | Medium — divergent enforcement paths | One engine library invoked by both; no duplicated rule logic |

Rollback trigger: disable the offending rule flag or revert to report-only.
Ownership: harness maintainers. Validation window: report-only until the seeded
sound/unsound plan corpus is green.

### Plan Hardening Signals (REQUIRED)

* public API/schema/contract change: absent.
* security/auth/permission/compliance-sensitive: absent.
* migration/backfill/destructive/irreversible: absent.
* external integration/operator checkpoint/external dependency: absent.
* high runtime/rollout/rollback risk: PRESENT — U3 fail-closed gate blocks shipment assembly; mitigated by report-only rollout + per-rule enable flags.

Requires plan hardening: yes

## Prior Plan Review (invalidated)

dispatch_mode: multi-agent-dispatch
decision: INVALIDATED

The prior PASS record is retained only as invalidated history. It omitted mandatory personas and is superseded by the genuine multi-agent Plan Review below.

## Plan Review

<!-- plan-review-attempt: 2 -->

dispatch_mode: multi-agent-dispatch
decision: FAIL

personas:
* Constitution Reviewer (`claude-opus-4.8`)
* Go Reviewer, anchor (`gpt-5.6-sol`, effort high)
* Scope Boundary Auditor (`gemini-3.7-flash`)
* Correctness Reviewer (`claude-sonnet-4.6`)
* Architecture Strategist (`grok-4.6`)
* Security Reviewer (`gpt-5.6-terra`) when risk-triggered for the plan
* Learnings Researcher over `docs/compound/`

Controlling P1 findings:
* The realized decomposition violated the 2-hour rule: the report verified `160.002-T` and `160.003-T` each bundled four check classes.
* U3's acceptance that S10 and this gate call the same engine is unverifiable at S8 ship time and creates a false-green path.
* S8 and S10 describe contradictory engine/evidence integration contracts.
* Per-rule enable flags and report-only fallback can fail open unless the assembly transition is guarded at the shared choke point.
