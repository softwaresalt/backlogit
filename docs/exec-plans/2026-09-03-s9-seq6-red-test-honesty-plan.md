---
chunk_strategy: h1-h2-h3
description: "Execution plan for S9: Sequence 6/7 red-test honesty gate"
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-09-03-s9-seq6-red-test-honesty-plan.md
title: "S9 Execution Plan — Sequence 6/7 Red-Test Honesty Gate"
---

# S9 Execution Plan — Red-Test Honesty Gate (Seq 6/7)

**Covering feature**: Red-test honesty gate
**Stash member**: 48F28B8D (high)
**Tier**: feature (shipment sequence S9)

## Problem Frame

False-green tests pass review: a claimed red test may already pass, not exercise
the changed branch, only assert sibling fields, or construct expected output
through the same buggy production path. Add a harness gate that identifies newly
added regression tests and runs them against the pre-fix/base commit or a fault-
injected baseline, rejecting dishonest reds and recording the observed red/green
as shipment evidence. This gate is consumed by the S11 enforcement engine.

## Constitution Check

| Principle | Compliance |
|-----------|-----------|
| I. Safety-First Go | Go 1.24; wrapped errors |
| II. Test-First (P-002) | gate declaration -> RED -> GREEN per unit |
| III. Workspace Isolation | baseline checkout in isolated worktree |
| IV. CLI Containment | n/a |
| V. Observability | red/green evidence recorded as shipment evidence |
| VI. Single Responsibility | one detector + one baseline runner |
| VII. Destructive Approval | none |
| VIII. Safety Modes | fail-closed: unverifiable red is rejected |
| IX. Git-Friendly | evidence is text |
| X. Context Efficiency | n/a |
| XI. Merge Commits | P-009 by Ship |

Constitution Check: pass

## Implementation Units

### U1 — New-test identification + baseline runner
* Scope: identify newly added regression tests in a change and run them against the pre-fix/base commit or an equivalent fault-injected baseline in an isolated worktree.
* Acceptance: newly added tests are correctly identified; baseline run captures pass/fail per test.

### U2 — Dishonest-red rejection rules
* Scope: reject a claimed red if it already passes at baseline, does not exercise the changed branch, only asserts sibling fields, or constructs expected output through the same buggy production path.
* Acceptance: each dishonest-red class is rejected on a seeded case; a genuine reproducer passes. Depends on U1.

### U3 — Shipment evidence recording
* Scope: emit the observed red failure and green result as structured evidence **against the shared fault-line evidence-artifact contract defined in S4 U4** (S9 is a producer, NOT the schema owner). Evidence is retrievable per task/commit for the S10 DAG and S11 enforcement engine.
* Acceptance: evidence conforms to the shared S4 U4 contract and is retrievable per task/commit; a conformance test validates S9's emitted artifact against the shared schema. Depends on U2 and the S4 U4 contract.

## Dependency Graph

U1 -> U2 -> U3. Single domain per unit.

## Runtime Verification and Closure

Verification surface: gate run over seeded honest/dishonest reds. Closure: gate
green in CI; evidence schema documented for S10/S11 consumers.

#### Plan Hardening Signals (REQUIRED)

* public API/schema/contract change: PRESENT-minor — emits against the shared S4 U4 evidence-artifact contract (S9 is a producer, not the schema owner); additive.
* security/auth/permission/compliance-sensitive: absent.
* migration/backfill/destructive/irreversible: absent.
* external integration/operator checkpoint/external dependency: absent.
* high runtime/rollout/rollback risk: absent.

Requires plan hardening: no

## Plan Review

dispatch_mode: multi-agent-dispatch
decision: PASS

Personas dispatched: Correctness Reviewer (always-on), Architecture Strategist (always-on); Adversarial Review re-review (post-remediation). Plan hardening NOT required (Requires plan hardening: no). This plan initially FAILED on a P1 and PASSED after remediation.

Findings and remediation:
- Architecture P1 (U3 became the de-facto owner of the shared evidence schema consumed by S10/S11): REMEDIATED — U3 now emits against the shared S4 U4 evidence-artifact contract; S9 is a producer, not the schema owner; conformance test added. Adversarial re-review verdict: RESOLVED.
- Correctness: clean — U1->U2->U3 sound; four dishonest-red classes each map to a rejection rule.

Plan-review cycle: attempt 1 FAIL (P1) -> remediated -> attempt 2 RESOLVED. No residual P0/P1.
