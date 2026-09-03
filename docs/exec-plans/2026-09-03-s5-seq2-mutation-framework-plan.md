---
chunk_strategy: h1-h2-h3
description: "Execution plan for S5: Sequence 2/7 mutation postcondition and consistency framework"
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-09-03-s5-seq2-mutation-framework-plan.md
title: "S5 Execution Plan — Sequence 2/7 Mutation Postcondition Framework"
---

# S5 Execution Plan — Mutation Postcondition & Consistency Framework (Seq 2/7)

**Covering feature**: Mutation postcondition and consistency framework
**Stash member**: C1808666 (critical)
**Tier**: feature (shipment sequence S5)

## Problem Frame

Mutating operations must update many representations (Markdown/frontmatter, routed
file path, SQLite projection, append-only event log, shipment/work-item status,
timestamps, archive metadata, provenance). State drift, silent data loss, stale
queries, and provenance loss recur. Build a reusable harness where each mutating
operation declares the representations it must update and the harness verifies
them after success, failure, and crash-boundary scenarios.

## Constitution Check

| Principle | Compliance |
|-----------|-----------|
| I. Safety-First Go | Go 1.24; wrapped errors |
| II. Test-First (P-002) | declaration -> RED -> GREEN per unit |
| III. Workspace Isolation | isolated temp workspaces + fault-injection fixtures |
| IV. CLI Containment | n/a |
| V. Observability | postcondition report is structured |
| VI. Single Responsibility | one declaration model + verifier |
| VII. Destructive Approval | none |
| VIII. Safety Modes | partial-write / indeterminate-atomic fixtures asserted |
| IX. Git-Friendly | text fixtures |
| X. Context Efficiency | n/a |
| XI. Merge Commits | P-009 by Ship |

Constitution Check: pass

## Implementation Units

### U1 — Representation declaration model
* Scope: a declarative model where each mutating op lists the representations it must update; a registry binding ops to their declared representation set.
* Acceptance: at least two existing mutating ops declare their representation sets; schema validated.

### U2 — Postcondition verifier (success/failure paths)
* Scope: after a mutation, verify all declared representations converged on success and rolled back / left untouched on failure.
* Acceptance: a correct mutation passes; an injected drift in any declared representation fails. Depends on U1.

### U3 — Crash-boundary and stale-index fixtures
* Scope: partial-write, stale-index, old-index, and indeterminate-atomic-write fixtures exercised against the verifier.
* Acceptance: each fixture provokes the expected verifier outcome (drift/loss detected or safe no-op). Depends on U2.

## Dependency Graph

U1 -> U2 -> U3. Single Go test-infra domain per unit.

## Runtime Verification and Closure

Framework is a verification surface; closure = it runs in CI over the declared
ops. No production behavior change (declarations describe existing behavior).

#### Plan Hardening Signals (REQUIRED)

* public API/schema/contract change: absent (test infra + declarations).
* security/auth/permission/compliance-sensitive: absent.
* migration/backfill/destructive/irreversible: absent.
* external integration/operator checkpoint/external dependency: absent.
* high runtime/rollout/rollback risk: absent.

Requires plan hardening: no

## Plan Review

dispatch_mode: multi-agent-dispatch
decision: PASS

Personas dispatched: Correctness Reviewer (always-on), Architecture Strategist (always-on). Plan hardening NOT required (Requires plan hardening: no).

Findings:
- Correctness: clean — U1->U2->U3 ordering sound; acceptance criteria testable; declared no-behavior-change (declarations describe existing ops).
- Architecture: clean — single declaration model + verifier; cohesive.

No P0/P1/P2. No residual items.
