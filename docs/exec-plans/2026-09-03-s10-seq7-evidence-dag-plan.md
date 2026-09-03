---
chunk_strategy: h1-h2-h3
description: "Execution plan for S10: Sequence 7/7 shipment-level fault-line evidence DAG framework"
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-09-03-s10-seq7-evidence-dag-plan.md
title: "S10 Execution Plan — Sequence 7/7 Shipment-Level Fault-Line Evidence DAG"
---

# S10 Execution Plan — Shipment-Level Fault-Line Evidence DAG (Seq 7/7)

**Covering feature (epic)**: Shipment-level fault-line evidence DAG framework
**Stash member**: C0A382C7 (high, epic)
**Tier**: feature (shipment sequence S10) — integrates seq 1-6 detectors

## Problem Frame

Known regression fault lines should become machine-enforced shipment completeness
proofs. Design a shipment contract where each shipment carries multiple fault-line-
specific DAGs/checklists (nodes = required checks, edges = evidence dependencies).
Integrate the sequence 1-6 detectors (S4-S9) as node executors rather than
duplicating their logic. The shipment cannot be review-ready until all applicable
terminal nodes have verified evidence.

## Constitution Check

| Principle | Compliance |
|-----------|-----------|
| I. Safety-First Go | Go 1.24; wrapped errors |
| II. Test-First (P-002) | node/edge declaration -> RED -> GREEN per unit |
| III. Workspace Isolation | DAG state stored in workspace |
| IV. CLI Containment | n/a |
| V. Observability | DAG status queryable/visualizable |
| VI. Single Responsibility | framework orchestrates existing detectors, no duplication |
| VII. Destructive Approval | none |
| VIII. Safety Modes | fail-closed: unmet terminal node blocks review-ready |
| IX. Git-Friendly | DAG declared in text |
| X. Context Efficiency | incremental evaluation |
| XI. Merge Commits | P-009 by Ship |

Constitution Check: pass

## Implementation Units

### U1 — DAG node/edge model + applicability (shared graph-evaluation core)
* Scope: node schema declaring applicability, deterministic validator, required evidence, producing task/commit, status, waiver/override authority, and failure/blocking semantics; edges for evidence dependencies; initial DAG families (parity, mutation/state, compat/error, docs/audit, plan/task soundness, red-test honesty). **This unit owns the single shared graph-evaluation + evidence-binding core for the program; S11 consumes it as a library rather than re-implementing graph traversal, evaluation, or shipment roll-up.** Evidence conforms to the shared S4 U4 evidence-artifact contract.
* Acceptance: a shipment can declare DAG families; schema validated; applicability resolves per shipment; the graph-evaluation core is exposed as a reusable library with a documented interface for S11; consumes S4 U4 evidence artifacts.

### U2 — Detector integration as node executors
* Scope: bind S4-S9 detectors as node executors so the DAG consumes their S4 U4-conformant evidence instead of reimplementing it.
* Acceptance: each family's terminal node is satisfied only when its bound detector produced verified evidence conforming to the shared contract. Depends on U1.

### U3 — Review-ready gate + incremental evaluation + authorized waivers
* Scope: shipment review-ready gate requiring all applicable terminal nodes verified; incremental evaluation; audited path for genuinely non-applicable nodes; query/visualization; versioning. **This DAG terminal-node set is the AUTHORITATIVE review-ready gate; the S11 shipment roll-up composes by contributing policy nodes INTO this DAG rather than gating review-ready independently.** Waiver / non-applicability declarations are validated against an explicit authorization policy (which principal may waive which node family) AND require an independently authenticated out-of-band principal (interactive-TTY confirmation or signed/short-lived token verified independently of the invoking process) — recording identity is not sufficient and agent/non-interactive waiver issuance is denied by default, so an agent-accessible waiver route cannot bypass the S11 override controls that compose into this same gate. Waiver records are append-only / tamper-evident.
* Acceptance: a shipment with an unmet terminal node cannot become review-ready; a waived node records audited AND authorized AND independently-authenticated authority (unauthorized OR self-asserted OR agent/non-interactive waiver rejected — denial path tested); waiver records are append-only; incremental re-eval only recomputes affected nodes; S11 policy nodes compose into this gate. Depends on U2.

## Dependency Graph

U1 -> U2 -> U3. Feature-level: S10 depends on S4 U4 (shared evidence contract)
and S4-S9 detectors (evidence producers) — enforced by shipment sequence
ordering. S10 U1 is the single graph-evaluation core; S11 consumes it.

## Runtime Verification and Closure

Runtime surface: shipment review-ready gate. Verification: seeded shipments with
met/unmet/waived terminal nodes. Closure: gate integrated; DAG status queryable.

## Plan Hardening

| ProposedAction | ActionRisk | Mitigation |
|---|---|---|
| U3 Review-ready gate | High | fail-closed only on unmet applicable terminal nodes; report-only rollout before enforcement |
| Waiver/non-applicability authority | High — a self-waived terminal node defeats the gate and bypasses composed S11 controls | Waivers validated against an explicit authorization policy AND an independently authenticated out-of-band principal (interactive-TTY or signed token); self-asserted identity and agent/non-interactive issuance denied by default; waiver records append-only / tamper-evident (hash-chain or signed) |
| Detector integration (reuse S4-S9) | Medium — logic duplication / drift | Bind detectors as node executors; DAG consumes S4 U4-conformant evidence, never reimplements |

Rollback trigger: ship the DAG in report-only mode; disable a node family flag if
it false-blocks. Ownership: harness maintainers. Validation window: report-only
until the seeded shipment corpus is green.

#### Plan Hardening Signals (REQUIRED)

* public API/schema/contract change: PRESENT — defines a shipment DAG contract (additive to shipment schema).
* security/auth/permission/compliance-sensitive: PRESENT-minor — waiver/override authority must be audited.
* migration/backfill/destructive/irreversible: absent.
* external integration/operator checkpoint/external dependency: absent.
* high runtime/rollout/rollback risk: PRESENT-minor — gates shipment review-ready; must fail-closed and support incremental rollout.

Requires plan hardening: yes

## Plan Review

dispatch_mode: multi-agent-dispatch
decision: PASS

Personas dispatched: Correctness Reviewer (always-on), Architecture Strategist (always-on), Security Reviewer (waiver/override authority + review-ready gate trigger); Adversarial Review re-review (post-remediation). Plan hardening was REQUIRED and is SATISFIED. This plan initially FAILED on P1 findings and PASSED after two remediation passes.

Findings and remediation:
- Architecture P1 (S10 and S11 each built a full graph-evaluation engine — duplication/drift): REMEDIATED — U1 owns the single shared graph-evaluation/evidence-binding core exposed as a library; S11 consumes it.
- Architecture P1 (S10 consumed S4-S9 as evidence producers but no shared evidence interface existed): REMEDIATED — U1/U2 consume the shared S4 U4 evidence-artifact contract.
- Architecture P2 (S10 U3 and S11 U4 both gated review-ready): REMEDIATED — S10 terminal-node set declared the authoritative gate; S11 composes policy nodes into it.
- Security P2 (waiver authority recorded but not authorized): REMEDIATED — waivers validated against an authorization policy with append-only tamper-evident records.
- Adversarial re-review NEW P1 (S10 waiver path lacked the authenticated/agent-denied boundary S11 overrides carry, becoming a bypass route): REMEDIATED — U3 waivers now require an independently authenticated out-of-band principal; self-asserted and agent/non-interactive waiver issuance is denied by default (denial path tested), mirroring S11 U3.

Plan-review cycle: attempt 1 FAIL (P1) -> remediated -> attempt 2 (new consistency P1) -> remediated. No residual P0/P1.
