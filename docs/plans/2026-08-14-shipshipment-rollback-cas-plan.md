---
title: "ShipShipment late-failure rollback and persistArtifact-level CAS hardening"
description: "Implementation plan for partial-release prevention and repository-wide CAS boundary tightening deferred from 106.033-T"
source: ".backlogit/archive/057-DL.md"
doc_type: plan
chunk_strategy: h1-h2-h3
schema_version: "1.0"
---

## Source

* Deliberation: `.backlogit/archive/057-DL.md` (3A649F8E)
* Trigger: PR #358 follow-up on rollback gap + CAS window gap

## Problem Frame

ShipShipment can mutate release scope items/features and then fail late before
final shipment-status persistence, leaving a partial-release state. In parallel,
current head-guard CAS checks occur before `persistArtifact`'s own prep stages,
so a residual race still exists between guard read and first mutating write.

The fix must address both tracks together because both meet at the final
"last read before persist" boundary.

## Requirements Trace

| Requirement | Implementation action |
|---|---|
| Prevent partial-release state on late failure | Unit 1 adds compensating rollback or deterministic reconciliation model |
| Tighten CAS boundary at shared persistence layer | Unit 2 designs and implements persistArtifact-level guard injection |
| Avoid narrow shipment-only workaround | Both units apply repository-wide persistence design where needed |
| Preserve traceability and diagnosability | Add doctor/reporting signals for detected partial state or guard refusal |

## Implementation Units

### Unit 1 - Late-failure rollback or reconciliation contract

* Changes
  * Define reversible mutation envelope for ShipShipment late-stage failures
  * Implement compensating undo or deterministic reconciliation when final persist fails
* Files
  * `internal/core/shipment_lifecycle.go`
  * `internal/core/shipment.go`
* Tests
  * Regression tests for late refusal/failure cases proving no persistent partial-release state
* Posture
  * Test-first with failure-injection seams

### Unit 2 - persistArtifact-level CAS redesign

* Changes
  * Introduce repository-scoped CAS boundary immediately before first mutating write inside persist path
  * Thread guard data without shipment-specific coupling
* Files
  * `internal/core/shipment.go`
  * `internal/core/*` shared persistence helpers
* Tests
  * Concurrency/race regression tests validating guard semantics for tasks/features/shipments
* Posture
  * Characterization-first then refactor

### Unit 3 - Diagnostics and doctor checks

* Changes
  * Add doctor-readable signal for partial-release detection
  * Add explicit refusal diagnostics when CAS mismatch blocks persist
* Files
  * `internal/cli/doctor.go`
  * `internal/core/*doctor*` or integrity module
* Tests
  * Doctor check assertions for both healthy and synthetic-failure states
* Posture
  * Verification-first

## Dependency Graph

* Unit 1 -> Unit 2
* Unit 1 -> Unit 3
* Unit 2 -> Unit 3

No external dependency blocks Stage harvest.

## Decisions and Rationale

* Keep this as dedicated feature because scope is larger than 106.033-T by design
* Prefer shared persist-layer CAS over shipment-only patch to avoid parity drift
* Require deterministic observability for any reconciliation path before Ship execution

## Risks and Caveats

* High blast radius across shared persistence logic
  * Mitigation: isolate interfaces, add cross-artifact regression matrix
* Rollback logic may hide real mutation ordering bugs if underspecified
  * Mitigation: explicit mutation envelope tests with failure injection per step
* CAS changes can create false refusals under normal operations
  * Mitigation: tune guard granularity and add actionable refusal diagnostics

## Plan Hardening Signals

* public API, schema, or contract change: **yes** (shipment lifecycle failure semantics)
* security, auth, permission, or compliance-sensitive behavior: **no**
* migration, backfill, destructive data/config action, or irreversible step: **yes** (rollback/reconciliation mutation semantics)
* external integration, operator checkpoint, or external dependency: **yes** (operational doctor/recovery workflows)
* high runtime, rollout, or rollback risk: **yes** (shared persistence and lifecycle core)

Requires plan hardening: **yes**

## Runtime Verification and Closure

* Runtime surfaces changed
  * ShipShipment lifecycle behavior
  * shared artifact persistence guard behavior
* Verification
  * Simulate late failures and verify no partial-release residue persists
  * Verify CAS refusal behavior across at least one task, feature, and shipment update path
  * Confirm doctor check reports clean state after successful runs
* Closure artifact expectations
  * Runtime verification note with failure-injection matrix
  * Rollback trigger: any reproducible partial-release residue after late failure
  * Owner: Ship execution agent for implementation cycle

## Plan Review

decision: PASS

* plan is coherent and explicitly larger than previous task scope
* interlock with shared persist layer is acknowledged and sequenced
* no blocking dependencies at Stage gate
* advisory
  * prefer reusable mutation-envelope abstraction over one-off rollback branches
  * track CAS refusal telemetry to validate production ergonomics
