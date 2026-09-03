---
chunk_strategy: h1-h2-h3
description: "Execution plan for S4: Sequence 1/7 cross-surface golden parity harness"
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-09-03-s4-seq1-parity-harness-plan.md
title: "S4 Execution Plan — Sequence 1/7 Cross-Surface Golden Parity Harness"
---

# S4 Execution Plan — Cross-Surface Golden Parity Harness (Seq 1/7)

**Covering feature**: Cross-surface golden parity harness
**Stash member**: 5A4DBE3C (critical)
**Tier**: feature (shipment sequence S4) — program foundation

## Problem Frame

Transport and contract divergence between CLI, MCP, and internal/event APIs
recurs (typed errors collapsing to exit 1, domainError mapping drift, retryability
disagreement, arrays lost through omitempty) and is caught late. Build a
generated, table-driven harness that runs one governed scenario through all three
surfaces and compares response shape, error category, exit code, retryability,
remediation guidance, serialization guarantees, and durable side effects.

## Constitution Check

| Principle | Compliance |
|-----------|-----------|
| I. Safety-First Go | Go 1.24; wrapped errors |
| II. Test-First (P-002) | scenario declaration -> RED -> GREEN per unit |
| III. Workspace Isolation | harness runs against isolated temp workspaces |
| IV. CLI Containment | n/a |
| V. Observability | comparison report is structured |
| VI. Single Responsibility | one comparator, table-driven scenarios |
| VII. Destructive Approval | none |
| VIII. Safety Modes | fail-closed on any surface mismatch |
| IX. Git-Friendly | golden fixtures are text |
| X. Context Efficiency | n/a |
| XI. Merge Commits | P-009 by Ship |

Constitution Check: pass

## Implementation Units

### U1 — Scenario model and three-surface driver
* Scope: define the governed-scenario table schema and a driver that executes one scenario through CLI, MCP, and internal/event APIs against an isolated workspace.
* Acceptance: a single seed scenario runs through all three surfaces and captures response shape, error category, exit code, retryability, remediation guidance, serialization, and durable side effects.

### U2 — Cross-surface comparator + divergence report
* Scope: compare the three captures dimension-by-dimension and emit a deterministic divergence report; fail closed on any mismatch.
* Acceptance: identical behavior passes; an injected divergence in each dimension is detected and reported. Depends on U1.

### U3 — Seed the recurring-failure corpus
* Scope: seed scenarios for the known recurring failures (typed error -> exit 1 collapse, domainError mapping drift, retryability disagreement, omitempty array loss).
* Acceptance: each seeded scenario is present and green against current code; any remaining red must be triaged to an existing tracked defect (never left as an undocumented failing test). Depends on U2.

### U4 — Shared fault-line evidence-artifact contract (program foundation)
* Scope: define the shared, versioned evidence-artifact interface that ALL program detectors (S4-S9) emit against and that the S10 evidence DAG and S11 enforcement engine consume: producing task/commit, node/family applicability, validator identity, verified-evidence payload, and status. This is the single owner of the evidence schema — S9 and the other detectors are producers, not schema owners. Lives in a shared package, not inside any one detector.
* Acceptance: the evidence-artifact contract is defined and versioned; the S4 parity harness emits its results against it; the schema is documented for S5-S9 producers and S10/S11 consumers; a conformance test validates a sample artifact. Independent foundation unit.

## Dependency Graph

U1 -> U2 -> U3. U4 (shared evidence contract) is an independent foundation unit
consumed by S5-S11. Single skill domain (Go test infrastructure) per unit.

## Runtime Verification and Closure

Harness is itself a verification surface; closure = the harness runs in CI and
the seed corpus is green. No production runtime change.

#### Plan Hardening Signals (REQUIRED)

* public API/schema/contract change: absent (test infra).
* security/auth/permission/compliance-sensitive: absent.
* migration/backfill/destructive/irreversible: absent.
* external integration/operator checkpoint/external dependency: absent.
* high runtime/rollout/rollback risk: absent.

Requires plan hardening: no

## Plan Review

dispatch_mode: multi-agent-dispatch
decision: PASS

Personas dispatched: Correctness Reviewer (always-on), Architecture Strategist (always-on). Plan hardening NOT required (Requires plan hardening: no).

Findings and remediation:
- Architecture P2 (billed as program foundation but did not own the shared evidence-artifact contract that S10 needs from every detector): REMEDIATED — added U4 defining the single shared, versioned fault-line evidence-artifact contract that S5-S9 emit against and S10/S11 consume.
- Correctness P3 (U3 permitted closing with an undocumented remaining red): REMEDIATED — U3 now requires any remaining red to be triaged to an existing tracked defect.

No P0/P1. Resolves the cross-plan evidence-contract P1 raised against S9/S10.
