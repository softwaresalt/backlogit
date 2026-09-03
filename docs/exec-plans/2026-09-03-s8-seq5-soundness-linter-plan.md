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
* Scope: dependency consistency, unique-or-intentionally-related titles, canonical procedures, no contradiction with originating decisions or sibling tasks, impossible-test detection, duplicated-work detection, missing-artifact detection, independent-verifiability. Each check class is distinct logic, so to respect the 2-hour rule and single-responsibility this unit is harvested as ONE subtask PER check class, each with a seeded case; they share the rule-engine scaffolding from U1. Reference/anchor resolution shares one resolver with S7 U3.
* Acceptance: each defect class flags a seeded case and passes clean input; each lands as an independently verifiable subtask.

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

## Plan Review

dispatch_mode: multi-agent-dispatch
decision: PASS

Personas dispatched: Correctness Reviewer (always-on), Architecture Strategist (always-on). Plan hardening became REQUIRED after remediation and is SATISFIED (## Plan Hardening added).

Findings and remediation:
- Correctness P2 (U2 packed ~8 defect classes into one unit): REMEDIATED — U2 harvested as one subtask per check class.
- Correctness P2 (U3 fail-closed assembly-blocking gate declared low-risk / no hardening — inconsistent with S10/S11): REMEDIATED — added ## Plan Hardening, report-only rollout with per-rule enable flags, and flipped Requires plan hardening: yes.
- Architecture P3 (soundness logic enforced twice: S8 U3 gate + S10 node): REMEDIATED — single engine library invoked by both call sites.

No P0/P1. No residual blocking items.
