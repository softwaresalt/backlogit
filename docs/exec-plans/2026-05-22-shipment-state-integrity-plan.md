---
chunk_strategy: h1-h2-h3
description: ""
doc_type: plan
docline:
    date: 2026-05-22T00:00:00Z
    origin: docs/decisions/2026-05-22-new-bug-backlog-grouping.md
    stash_ids:
        - FE806724
        - 36F1CB1A
    status: reviewed
ingested_at: "2026-06-26T02:33:21Z"
schema_version: "1.0"
source: docs/exec-plans/2026-05-22-shipment-state-integrity-plan.md
title: Shipment State Integrity
---

## Problem Frame

Shipment lifecycle code can leave partially transitioned state behind: a claim
can activate only part of a shipment, and returned items can keep stale blocked
metadata after they re-enter the backlog.

## Requirements Trace

| # | Requirement | Origin |
|---|---|---|
| R1 | Claim rollback must restore shipment and item status when activation fails mid-flight | stash FE806724 |
| R2 | Returned backlog items must clear stale `blocked_reason` metadata when they are no longer blocked | stash 36F1CB1A |

## Scope Boundaries

### In Scope

* shipment claim transition rollback
* returned-to-backlog metadata cleanup
* regression tests for partial activation and returned item cleanup

### Non-Goals

* shipment assembly UX changes
* archive/unarchive fixes
* dependency scheduling fixes

## Implementation Units

### Unit 1: Make ClaimShipment activation atomic

**Files:** `internal/core/shipment_lifecycle.go`, CLI/MCP shipment claim surfaces as needed
**Test files:** shipment atomic coverage
**Effort size:** medium
**Skill domain:** code
**Execution note:** test-first
**Dependencies:** none

**Acceptance criteria**

* failed activation restores shipment status to queued
* previously activated items revert to queued on rollback
* normal claim success path remains unchanged

### Unit 2: Clear stale blocked metadata on returned items

**Files:** `internal/core/shipment_lifecycle.go`
**Test files:** shipment lifecycle coverage
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first
**Dependencies:** none

**Acceptance criteria**

* items returned to backlog no longer carry stale `custom_fields.blocked_reason`
* queued items reflect current availability accurately

## Dependency Graph

* Units 1 and 2 are parallel

## Decisions

| # | Decision | Rationale |
|---|---|---|
| D1 | Keep shipment rollback and returned-item cleanup together | both defects live in shipment lifecycle state transitions |

## Learnings Applied

* Parent-first shipment manifests remain required; this plan focuses on claim and
  return correctness after that manifest exists

## Plan Hardening Signals (REQUIRED)

* public API, schema, or contract change: **absent**
* security, auth, permission, or compliance-sensitive behavior: **absent**
* migration, backfill, destructive data/config action, or irreversible step: **absent**
* external integration, operator checkpoint, or external dependency: **absent**
* high runtime, rollout, or rollback risk: **absent**

Requires plan hardening: no

## Plan Review

**Gate Decision: PASS**

Review notes:

* shipment state fixes are tightly scoped to two lifecycle defects
* no extra implementation units are required to preserve atomicity
