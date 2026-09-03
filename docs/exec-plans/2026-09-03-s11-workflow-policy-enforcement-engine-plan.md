---
chunk_strategy: h1-h2-h3
description: "Execution plan for S11: deterministic harness-wide workflow-policy enforcement engine"
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-09-03-s11-workflow-policy-enforcement-engine-plan.md
title: "S11 Execution Plan — Workflow-Policy Enforcement Engine"
---

# S11 Execution Plan — Deterministic Workflow-Policy Enforcement Engine

**Covering feature**: Deterministic harness-wide workflow-policy enforcement engine
**Deliberation**: docs/decisions/2026-09-03-dark-factory-grouping-ledger.md (A2C91FE5 decision)
**Stash member**: A2C91FE5 (high)
**Tier**: feature (shipment sequence S11) — capstone; consumes S9, shares S10 DAG arch

## Problem Frame

Prose-only policy gates (P-002 TDD, P-004 red-phase, P-009 merge-commit,
Constitution III/IV containment) recurrently breach despite documentation and
review (incidents INC-P002-131S-148F, INC-P002-152F-134S). Build a machine-enforced
prerequisite graph with commit-SHA-bound evidence, fail-closed evaluation, audited
overrides, and CLI/MCP/agent surface parity, rolling up to shipment level. This
engine enforces ordering; the S9 red-test honesty gate produces the red-phase
evidence; the S10 evidence DAG provides the shared node/edge architecture.

## Constitution Check

| Principle | Compliance |
|-----------|-----------|
| I. Safety-First Go | Go 1.24; wrapped errors |
| II. Test-First (P-002) | declaration -> RED -> GREEN per unit; engine dogfoods P-002 |
| III. Workspace Isolation | evidence + graph stored within workspace |
| IV. CLI Containment | n/a |
| V. Observability | structured preflight failure + audited overrides |
| VI. Single Responsibility | one graph evaluator; reuses S9 evidence + S10 arch |
| VII. Destructive Approval | none |
| VIII. Safety Modes | fail-closed default BLOCK on ambiguous state |
| IX. Git-Friendly | graph + evidence declared in text |
| X. Context Efficiency | incremental evaluation |
| XI. Merge Commits | enforces P-009 |

Constitution Check: pass

## Implementation Units

### U1 — Prerequisite graph model + registry declaration
* Scope: typed nodes (harness-scaffold, red-phase-verify, green-phase-verify, review-complete, CI-pass, merge-ready) and ordering edges declared in the workflow-policy registry; runtime-evaluable. **Reuse boundary (explicit): this unit CONSUMES the shared graph-evaluation + evidence-binding core owned by S10 U1 as a library — it does NOT re-implement graph traversal, evaluation, incremental re-eval, or shipment roll-up. S11 adds ONLY its policy node types (red-phase-verify, merge-ready, etc.) and ordering semantics on top of S10's core.**
* Acceptance: the graph is declared and loads via the shared S10 core; node/edge schema validated; ambiguous declarations rejected; no duplicate graph-traversal/evaluation code is introduced (verified by importing S10's core).

### U2 — Commit-SHA-bound evidence + fail-closed evaluator
* Scope: each node requires a commit-SHA-bound evidence artifact (conforming to the S4 U4 contract); the evaluator (from S10's core) blocks (default BLOCK on ambiguity) when predecessor evidence is absent or out-of-order; evidence cannot be retroactively fabricated. Red-phase-verify consumes the S9 honesty gate. Commit SHAs are verified against protected/remote refs where available, not only the writable local repo. Review-complete / CI-pass evidence is derived through the S7 evidence-derivation module (single GitHub/git derivation source), not a second GitHub-API implementation.
* Acceptance: missing/out-of-order evidence fails preflight with a structured error; a satisfied graph passes; fabricated evidence is rejected; SHA verification prefers protected refs. Depends on U1 and S7 derivation.

### U3 — Surface parity (CLI/MCP/agent) + authenticated audited overrides
* Scope: identical enforcement across CLI commands, MCP tool calls, and agent skill dispatch; explicit overrides (P-002.1 harness-exempt) require an **authenticated, out-of-band operator signal** (interactive-TTY confirmation, or a signed/short-lived operator token verified independently of the invoking process) — a self-asserted "operator identity" field is INSUFFICIENT and override issuance from non-interactive/agent contexts is denied by default. Override records are append-only / tamper-evident (hash-chain or signed) and capture the authenticated principal, justification, and specific gate.
* Acceptance: no surface bypasses a gate another enforces (parity test reuses the S4 harness); an override requires and records an authenticated out-of-band signal; a self-asserted-only override attempt from an agent/non-interactive context is DENIED; override log is append-only and tamper-evident. Depends on U2.

### U4 — Shipment roll-up integration
* Scope: the prerequisite graph rolls up to shipment level so a shipment cannot transition to review-ready/ship-ready unless all constituent tasks satisfied all applicable gates; integrates with the S10 DAG terminal-node verification.
* Acceptance: a shipment with an unsatisfied task gate cannot become review-ready; a fully satisfied shipment proceeds. Depends on U3.

## Dependency Graph

U1 -> U2 -> U3 -> U4. Feature-level: depends on S4 U4 (shared evidence
contract), S7 (single GitHub/git evidence-derivation source), S9 (red-phase
evidence producer), and S10 (shared graph-evaluation core + authoritative
review-ready DAG that S11 composes policy nodes into) — enforced by shipment
sequence ordering.

## Runtime Verification and Closure

Runtime surface: CLI/MCP/agent preflight + shipment transition gate. Verification:
seeded satisfied/violating/ambiguous graphs across all three surfaces; incident-
replay scenarios (131S/148F, 152F/134S ordering) must now BLOCK. Closure: fail-
closed behavior + override audit trail documented; incremental rollout strategy
recorded before enablement.

## Plan Hardening

This plan carries multiple hardening signals (contract change, security-sensitive
overrides, high runtime/gate risk, cross-surface parity). Hardened detail:

| ProposedAction | ActionRisk | Mitigation |
|---|---|---|
| Fail-closed preflight blocking implementation steps | High — could block all work if misconfigured | Default BLOCK only on ambiguous/missing evidence; incremental rollout behind an explicit enable flag per gate; dry-run/report-only mode first |
| Commit-SHA-bound evidence store | Medium — evidence tampering / retroactive fabrication | Evidence bound to immutable commit SHAs; append-only; verified against git history, not self-asserted |
| Audited overrides (harness-exempt) | High — self-asserted identity collapses the fail-closed engine to opt-out | Overrides require an AUTHENTICATED out-of-band operator signal (interactive-TTY or signed/short-lived token verified independently); self-asserted identity fields are rejected; override issuance denied by default from non-interactive/agent contexts; override log append-only / tamper-evident |
| CLI/MCP/agent parity | Medium — one surface bypasses a gate | Parity enforced by a cross-surface test (reuses S4 harness); no surface-specific gate skips |
| Graph engine reuse | Medium — duplicate graph engine drifts from S10 | S11 imports S10 U1's shared graph-evaluation core as a library; adds only policy node types; no re-implemented traversal/roll-up |
| Incident replay | n/a | 131S/148F and 152F/134S ordering scenarios must BLOCK as regression proof; prior incidents are not retroactively rewritten |

Rollback trigger: if the engine produces false-positive blocks in report-only
rollout, disable the offending gate flag; the engine ships report-only before
enforcing. Ownership: harness maintainers. Validation window: report-only until
the seed incident-replay corpus is green.

### Plan Hardening Signals (REQUIRED)

* public API/schema/contract change: PRESENT — workflow-policy registry graph + evidence artifacts across CLI/MCP/agent surfaces.
* security/auth/permission/compliance-sensitive: PRESENT — audited overrides, evidence integrity, gate bypass surface.
* migration/backfill/destructive/irreversible: absent (additive; report-only first).
* external integration/operator checkpoint/external dependency: PRESENT — operator override checkpoints; git history as evidence source.
* high runtime/rollout/rollback risk: PRESENT — fail-closed gating of implementation and shipment transitions; incremental rollout required.

Requires plan hardening: yes

## Plan Review

dispatch_mode: multi-agent-dispatch
decision: PASS

Personas dispatched: Correctness Reviewer (always-on), Architecture Strategist (always-on), Security Reviewer (fail-closed gates + audited overrides + gate-bypass surface trigger); Adversarial Review re-review (post-remediation). Plan hardening was REQUIRED and is SATISFIED (## Plan Hardening present). This plan initially FAILED on P1 findings and PASSED after remediation. This is the A2C91FE5-mandated adversarial security lens.

Findings and remediation:
- Security P1 (self-asserted operator-identity overrides collapse the fail-closed engine to opt-out in an autonomous harness): REMEDIATED — U3 requires an authenticated out-of-band operator signal (interactive-TTY or independently-verified signed/short-lived token); self-asserted identity rejected; agent/non-interactive override issuance denied by default; override log append-only / tamper-evident. Adversarial re-review verdict: RESOLVED.
- Architecture P1 (re-declared a full parallel graph stack duplicating S10): REMEDIATED — U1 consumes S10 U1's shared core as a library; adds only policy node types + ordering; no re-implemented traversal/roll-up. Adversarial re-review verdict: RESOLVED.
- Architecture P2 (duplicate GitHub/git evidence derivation vs S7): REMEDIATED — U2 routes review-complete/CI-pass through the S7 single derivation source; SHA verification prefers protected/remote refs.

Plan-review cycle: attempt 1 FAIL (P1) -> remediated -> attempt 2 RESOLVED. No residual P0/P1.
