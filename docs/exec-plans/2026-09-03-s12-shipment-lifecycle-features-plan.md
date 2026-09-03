---
chunk_strategy: h1-h2-h3
description: "Execution plan for S12: shipment lifecycle features — parked state and queued-record forward repair"
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-09-03-s12-shipment-lifecycle-features-plan.md
title: "S12 Execution Plan — Shipment Lifecycle Features"
---

# S12 Execution Plan — Shipment Lifecycle Features

**Covering feature**: Shipment lifecycle — parked state and queued-record forward repair
**Deliberation**: docs/decisions/2026-09-03-dark-factory-grouping-ledger.md (BE32CAE2 decision)
**Stash members**: C52993E3 (feature), BE32CAE2 (task)
**Tier**: feature (shipment sequence S12)

## Problem Frame

Two shipment-lifecycle capability gaps: (1) no parked state to temporarily pause a
shipment while higher-priority work proceeds; (2) no sanctioned, non-destructive
way to correct a shipment record stuck at `queued` while its members have already
advanced consistently beyond queued (report-only detection exists via 112-F/118-S,
but no repair operation).

## Constitution Check

| Principle | Compliance |
|-----------|-----------|
| I. Safety-First Go | Go 1.24; wrapped errors |
| II. Test-First (P-002) | declaration -> RED -> GREEN per unit |
| III. Workspace Isolation | shipment records within workspace |
| IV. CLI Containment | n/a |
| V. Observability | parked filtering + audited repair evidence |
| VI. Single Responsibility | parked-state vs forward-repair separated |
| VII. Destructive Approval | forward-repair is forward-only, non-destructive, operator-invoked |
| VIII. Safety Modes | fail-closed: ambiguous pre-state rejected, not repaired |
| IX. Git-Friendly | shipment YAML |
| X. Context Efficiency | queue filtering |
| XI. Merge Commits | P-009 by Ship |

Constitution Check: pass

## Implementation Units

### U1 — Parked shipment state (C52993E3)
* Scope: add a `parked` state; parked shipments remain in the queue directory and are not archived, but queue queries and ready-work selection filter them out.
* Acceptance: a parked shipment is excluded from queue queries and ready-work selection but not archived; unparking restores eligibility; state transitions covered by tests.

### U2 — Queued-record forward-repair operation (BE32CAE2)
* Scope: operator-only, CLI-only, audited break-glass mirroring the `repair_member_evidence` / `force_cli_only` contract. Because "CLI-only" is a surface boundary and NOT an authorization boundary in an autonomous harness, the verb is additionally gated behind an enforced interactive-TTY / explicit operator-confirmation token so "operator-only" is technically enforced, not surface-implied; non-interactive/agent invocation is denied by default. Inspection-gated: proceeds only when shipment.status == queued AND all non-descoped members advanced consistently beyond queued; ambiguous/mixed/incompatible states rejected with explanation. Forward-only (queued->active), shipment-record-only (no member/parent mutation, no ClaimShipment replay), atomic and concurrency-safe, emitting append-only structured before/after/result evidence with operator justification. Explicitly EXCLUDES DD957688. Recognizes the U1 `parked` state (a parked shipment is not an eligible repair pre-state).
* Acceptance: repairs a genuine under-advanced queued record forward-only; rejects ambiguous pre-states AND parked shipments; never mutates members/parents; requires an enforced operator-confirmation signal (non-interactive invocation denied); emits append-only before/after/result evidence; MCP surface intentionally absent (documented). Independent of U1.

## Dependency Graph

U1 and U2 independent (different lifecycle concerns). Order: U1, U2.

## Runtime Verification and Closure

Runtime surface: queue/ready-work selection (U1) and a CLI break-glass verb (U2).
Verification: seeded parked shipments filtered; seeded under-advanced/ambiguous
records repaired/rejected. Closure: repair audit evidence schema documented.

## Plan Hardening

| ProposedAction | ActionRisk | Mitigation |
|---|---|---|
| Forward-repair mutates shipment record | Medium — state override could mask torn state | Inspection-gated to one narrow split shape; forward-only; rejects ambiguous states; atomic with rollback; never touches members/parents |
| Operator break-glass surface | Medium — misuse / silent self-heal / agent invocation | CLI-only AND enforced interactive-TTY / operator-confirmation token (surface boundary is not an authorization boundary); non-interactive/agent invocation denied by default; never automatic/background; MCP exclusion deliberate (mirrors force_cli_only); requires operator justification |
| Audit evidence | Low | Structured before/after/result evidence recorded for every invocation |

Rollback trigger: repair is forward-only and shipment-record-only; a bad
invocation is bounded to one shipment record and auditable. Ownership: operators.

### Plan Hardening Signals (REQUIRED)

* public API/schema/contract change: PRESENT — new parked state + new CLI repair verb.
* security/auth/permission/compliance-sensitive: PRESENT — operator break-glass state repair.
* migration/backfill/destructive/irreversible: PRESENT-minor — forward-only record transition (non-destructive, no backward repair).
* external integration/operator checkpoint/external dependency: PRESENT — operator-invoked checkpoint.
* high runtime/rollout/rollback risk: absent — bounded to one shipment record.

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
* U2 is declared independent even though it depends on U1's parked state; the dependency graph must be U1 -> U2.
* TTY or self-asserted token authorization is spoofable and cannot protect the break-glass transition.
* CLI-only does not enforce operator-only; the shared `queued -> active` transition choke point must reject bypass paths.
* The new `parked` state is not integrated into the closed shipment status taxonomy consumed by already-planned gates.
* No named atomic repair seam exists across Markdown, SQLite cache, and JSONL audit evidence.
