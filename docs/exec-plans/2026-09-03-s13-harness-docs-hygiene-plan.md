---
chunk_strategy: h1-h2-h3
description: "Execution plan for S13: harness and documentation hygiene"
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-09-03-s13-harness-docs-hygiene-plan.md
title: "S13 Execution Plan — Harness & Documentation Hygiene"
---

# S13 Execution Plan — Harness & Documentation Hygiene

**Covering feature (chore)**: Harness and documentation hygiene
**Stash members**: 66834D9E, 360A183F, 633818E1
**Tier**: documentation-only (shipment sequence S13, last)

## Problem Frame

Three documentation/harness-guidance follow-ups: commit-scope vocabulary drift,
upstreaming checkpoint-context continuity wording into the instruction template,
and a confirmed no-action plugin-bundle scope boundary. All are documentation-only
and staged last (feature work supersedes docs-only).

## Constitution Check

| Principle | Compliance |
|-----------|-----------|
| I. Safety-First Go | n/a (docs) |
| II. Test-First (P-002) | docs-lint gate is the verification |
| III. Workspace Isolation | in-repo docs; cross-repo caveat noted (U2) |
| IV. CLI Containment | n/a |
| V. Observability | n/a |
| VI. Single Responsibility | three independent docs edits |
| VII. Destructive Approval | none |
| VIII. Safety Modes | n/a |
| IX. Git-Friendly | Markdown |
| X. Context Efficiency | clearer agent guidance |
| XI. Merge Commits | P-009 by Ship |

Constitution Check: pass

## Implementation Units

### U1 — Reconcile commit-scope vocabulary (66834D9E)
* Scope: update commit-message.instructions.md scope enumeration and body-length guidance to match actual repo practice (events, docline, memory, compound, cli-reference; longer multi-bullet bodies), OR document the tightened limit. Choose the reconcile-to-practice direction.
* Acceptance: the documented scope vocabulary and body-length guidance match observed practice; docs-lint passes.

### U2 — Upstream checkpoint-context continuity wording (360A183F)
* Scope: upstream the checkpoint-context Continuity Protocol wording (schema_version 1: arbitrary keys nested under `context`; top level + progress closed; rejected-create means retry-nested; context is unredacted git-tracked durable state that must not carry secrets) into backlogit.instructions.md.tmpl. CROSS-REPO CAVEAT: the `.tmpl` may live in the upstream autoharness harness repo (read-only here); if so, this unit is executed there, and the durable in-repo statement remains the U4b create_checkpoint MCP tool description.
* Acceptance: the continuity wording exists in the template (or the cross-repo caveat is recorded and the in-repo MCP tool description confirmed as the durable statement); docs-lint passes on any in-repo edit.

### U3 — Record plugin-bundle P-002 scope boundary (633818E1)
* Scope: record, as an accepted scope boundary, that the P-002/P-002.1-P-002.5 harness-satisfied contract is intentionally NOT propagated to plugin/agents/ship.agent.md or plugin/skills/build-feature/SKILL.md (per 101-F plugin/.github non-synchronization). No code change.
* Acceptance: the scope boundary is documented as accepted (no action) with reference to 101-F; no plugin content is modified.

## Dependency Graph

U1, U2, U3 independent. Order: U1, U2, U3.

## Runtime Verification and Closure

No runtime surface. Verification: docs-lint gate. Closure: documentation reflects
practice and accepted boundaries.

#### Plan Hardening Signals (REQUIRED)

* public API/schema/contract change: absent (docs only).
* security/auth/permission/compliance-sensitive: absent.
* migration/backfill/destructive/irreversible: absent.
* external integration/operator checkpoint/external dependency: PRESENT-minor — U2 may target an upstream read-only repo (caveat recorded, not acted on here).
* high runtime/rollout/rollback risk: absent.

Requires plan hardening: no

## Plan Review

dispatch_mode: multi-agent-dispatch
decision: PASS

Personas dispatched: Correctness Reviewer (always-on), Architecture Strategist (always-on). No security trigger (docs-only). Plan hardening NOT required (Requires plan hardening: no).

Findings and remediation:
- Correctness P3 (U2 could close as a no-op when the .tmpl lives upstream): REMEDIATED — U2 acceptance requires the in-repo fallback (confirm the create_checkpoint MCP tool description is the durable statement) as a verifiable step; cross-repo caveat recorded.
- Architecture: clean — three independent docs edits, cohesion-separated.

No P0/P1/P2. No residual blocking items.
