---
title: "Dependency Queue Integrity"
date: 2026-05-22
origin: "docs/decisions/2026-05-22-new-bug-backlog-grouping.md"
status: reviewed
stash_ids:
  - 250CC1F9
  - 3E33EE12
  - ED0DAA74
---

## Problem Frame

Dependency creation and queue filtering disagree about what counts as a valid
or blocking edge. The result is either phantom dependencies that hide work from
the queue or dependency types that do not actually constrain execution order.

## Requirements Trace

| # | Requirement | Origin |
|---|---|---|
| R1 | Reject dependency inserts that reference missing items | stash 250CC1F9 |
| R2 | Queue evaluation must not silently hide items on dependency read failure | stash 3E33EE12 |
| R3 | `parent_of` and `relates_to` must match documented execution-blocking semantics, or the contract must be narrowed explicitly in code and tests | stash ED0DAA74 |

## Scope Boundaries

### In Scope

* dependency validation in CLI, MCP, and DB path
* queue dependency filtering behavior
* dependency-type scheduling semantics
* regression tests for queue visibility

### Non-Goals

* redesign of the full dependency model
* shipment lifecycle changes
* archive or stash lifecycle fixes

## Implementation Units

### Unit 1: Validate dependency targets before insert

**Files:** `internal/db/dependencies.go`, `internal/cli/dep.go`, `internal/mcp/tools.go`
**Test files:** `internal/db/dependencies_test.go`, contract coverage for dependency tools
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first
**Dependencies:** none

**Acceptance criteria**

* adding a dependency to a missing item returns an error
* no `item_deps` row is written for a missing source or target
* CLI and MCP surface the validation failure cleanly

### Unit 2: Make queue dependency evaluation fail visible

**Files:** `internal/core/queue.go`
**Test files:** `internal/core/queue_test.go`
**Effort size:** small
**Skill domain:** code
**Execution note:** characterization-first
**Dependencies:** Unit 1

**Acceptance criteria**

* dependency lookup failure does not silently remove an item from queue output
* queue callers get either an explicit error or an intentional fail-open result
* regression coverage proves the item remains observable under dependency-read faults

### Unit 3: Align dependency scheduler semantics with allowed dep types

**Files:** `internal/core/queue.go`, `internal/db/dependencies.go`, dependency tool docs/tests
**Test files:** queue and dependency scheduler coverage
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first
**Dependencies:** Unit 1

**Acceptance criteria**

* execution-blocking dependency types are enforced consistently
* tests cover `blocks`, `parent_of`, and `relates_to`
* queue order matches the declared dependency contract

## Dependency Graph

* Unit 1 first
* Units 2 and 3 follow after the validation baseline is in place

## Decisions

| # | Decision | Rationale |
|---|---|---|
| D1 | Keep the fix as a bugfix shipment, not a broad dependency redesign | current defects are contract mismatches, not missing features |
| D2 | Preserve explicit queue visibility under dependency faults | invisible work is worse than conservatively visible work |

## Learnings Applied

* Hierarchy and relationship validation should happen before persistence, not
  after rehydration repair

## Plan Hardening Signals (REQUIRED)

* public API, schema, or contract change: **present** but bugfix-only
* security, auth, permission, or compliance-sensitive behavior: **absent**
* migration, backfill, destructive data/config action, or irreversible step: **absent**
* external integration, operator checkpoint, or external dependency: **absent**
* high runtime, rollout, or rollback risk: **absent**

Requires plan hardening: no

## Plan Review

**Gate Decision: PASS**

Review notes:

* units are independently backlog-sized
* code surfaces remain within dependency and queue subsystems
* no build, migration, or PR work is required from Stage
