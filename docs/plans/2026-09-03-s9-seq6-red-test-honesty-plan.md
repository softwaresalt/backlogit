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

## Plan Hardening

| ProposedAction | ActionRisk | Mitigation |
|---|---|---|
| Emit red/green evidence against the S4 U4 shared contract | Medium — evidence feeds later S10/S11 gates | Treat S9 as an additive producer only; validate emitted artifacts against the S4-owned schema and reject unknown schema versions |
| Run newly added tests against baseline code | High — new test files do not exist at the base commit and can false-green without a defined overlay seam | Resolve the base ref to an immutable SHA; apply only test-file changes over baseline production code in a workspace-contained ignored directory, or use a defined fault-injection path |
| Record evidence for later gating | Medium — forged or replayed evidence can satisfy downstream gates | Bind evidence to producer identity, task ID, commit SHA, and schema version before applicability filtering |

Rollback: leave the gate report-only if the baseline overlay seam cannot be made
deterministic. Compatibility: S9 does not own the evidence schema and must follow
S4 U4. Ownership: S9 owns red-test honesty production; S4 owns schema; S10/S11
consume authenticated evidence.

### Plan Hardening Signals (REQUIRED)

* public API/schema/contract change: PRESENT-minor — emits against the shared S4 U4 evidence-artifact contract (S9 is a producer, not the schema owner); additive.
* security/auth/permission/compliance-sensitive: absent.
* migration/backfill/destructive/irreversible: absent.
* external integration/operator checkpoint/external dependency: absent.
* high runtime/rollout/rollback risk: absent.

Requires plan hardening: yes

## Prior Plan Review (invalidated)

dispatch_mode: multi-agent-dispatch
decision: INVALIDATED

The prior PASS record is retained only as invalidated history. It omitted mandatory personas and is superseded by the genuine multi-agent Plan Review below.

## Plan Review

<!-- plan-review-attempt: 2 -->

dispatch_mode: multi-agent-dispatch
decision: FAIL

Gate note: the controlling Go P1×2 (baseline-overlay seam; workspace-contained
immutable base runner) are unresolved, so under the any-unresolved-P1 rule this
verdict is FAIL, not ADVISORY. Only a re-review after remediation can downgrade it.

personas:
* Constitution Reviewer (`claude-opus-4.8`)
* Go Reviewer, anchor (`gpt-5.6-sol`, effort high)
* Scope Boundary Auditor (`gemini-3.7-flash`)
* Correctness Reviewer (`claude-sonnet-4.6`)
* Architecture Strategist (`grok-4.6`)
* Security Reviewer (`gpt-5.6-terra`) when risk-triggered for the plan
* Learnings Researcher over `docs/compound/`

Controlling P1 findings (unresolved — force FAIL):
* The baseline runner cannot execute newly added test files at the base commit without a defined overlay or fault-injection seam.
* The isolated checkout must resolve the base ref to an immutable SHA and stay inside a sanctioned workspace-contained ignored directory.
* Evidence needs producer, task, commit, and anti-replay authenticity binding before downstream applicability filtering.
* The hardening mismatch is mitigated because S9 is an additive producer and S4 owns the schema, but the section is now recorded explicitly.
