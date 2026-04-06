---
description: "Workflow policy registry for backlogit harness sequencing, gates, and violation handling"
applyTo: '**'
---

# Workflow Policy Registry

**Version**: 1.0.0

This registry defines the cross-agent policies that keep the backlogit harness predictable and safe.

## P-001: Single-feature completion

| Field      | Value                |
|------------|----------------------|
| Policy ID  | P-001                |
| Applies To | `build-orchestrator` |
| Gate Point | Pre-flight           |

**Statement**: The build orchestrator should finish one feature through a stable handoff before starting work on a different feature.

**Precondition**: No backlog tasks with status `active` exist under a different feature unless the operator explicitly overrides the policy.

**Postcondition**: The current feature's executable work is complete or intentionally paused before a new feature is claimed.

**Violation action**: Halt and ask for operator direction.

## P-002: TDD gate

| Field      | Value                             |
|------------|-----------------------------------|
| Policy ID  | P-002                             |
| Applies To | `build-orchestrator`, `harness-architect` |
| Gate Point | Queue build and task claim        |

**Statement**: The build orchestrator may only claim work after the harness architect confirmed a compilable red phase.

**Precondition**: The task carries the `harness-ready` label.

**Postcondition**: The harness architect verified `go test -run=^$ -count=1 ./...` passes and the targeted harness tests fail for the expected reason.

**Violation action**: Halt and direct the operator to run the harness architect first.

## P-003: Decomposition chain integrity

| Field      | Value               |
|------------|---------------------|
| Policy ID  | P-003               |
| Applies To | `backlog-harvester` |
| Gate Point | Pre-harvest         |

**Statement**: Every decomposition stage must reference its source and parent context before creating backlog items.

**Precondition**:

1. The source document exists.
2. The plan references the source document.
3. Every created child item has a valid parent relationship.
4. Every executable task includes at least one verifiable acceptance criterion.

**Violation action**: Halt and do not create a partial hierarchy.

## P-004: Red phase before implementation

| Field      | Value                |
|------------|----------------------|
| Policy ID  | P-004                |
| Applies To | `harness-architect`  |
| Gate Point | Approval gate        |

**Statement**: The harness architect must confirm the red phase before applying `harness-ready`.

**Precondition**: `go test -run=^$ -count=1 ./...` exits successfully and the targeted harness command exits non-zero with expected failure markers.

**Postcondition**: The harness record shows compilation passed and the red phase was confirmed.

**Violation action**: Do not apply `harness-ready`.

## P-005: Policy violation telemetry

| Field      | Value            |
|------------|------------------|
| Policy ID  | P-005            |
| Applies To | governed agents  |
| Gate Point | Any violation    |

**Statement**: Policy violations are first-class operational signals.

**Required actions**:

* Broadcast the violation with the policy ID and a short summary.
* Record the violation in memory checkpoints when it affects ongoing work.
* Include notable policy deviations in PR or review context when relevant.
