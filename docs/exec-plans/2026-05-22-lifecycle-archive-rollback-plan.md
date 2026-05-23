---
title: "Archive and Hierarchy Rollback Integrity"
date: 2026-05-22
origin: "docs/decisions/2026-05-22-new-bug-backlog-grouping.md"
status: reviewed
stash_ids:
  - D3BC5A25
  - 3BA4181D
  - 88A64440
  - 0B9C7903
---

## Problem Frame

Several lifecycle mutations can leave backlog artifacts split across queue,
archive, and database state when a partial failure occurs. These defects all
share the same safety goal: rollback must restore a discoverable, internally
consistent artifact state.

## Requirements Trace

| # | Requirement | Origin |
|---|---|---|
| R1 | Archive must prefer the queued source artifact or fail clearly when duplicate queue/archive copies exist | stash D3BC5A25 |
| R2 | Unarchive must restore logical status as well as physical location | stash 3BA4181D |
| R3 | Adopt rollback must restore frontmatter and file identity consistently on failure | stash 88A64440 |
| R4 | Stash harvest must avoid orphaned artifacts when JSONL persistence fails after DB writes | stash 0B9C7903 |

## Scope Boundaries

### In Scope

* archive and unarchive lifecycle paths
* adopt rollback safety
* stash harvest rollback safety
* regression tests for duplicate-path and partial-failure recovery

### Non-Goals

* new lifecycle features
* shipment-claim activation logic
* metadata catalog or merge-sync fixes

## Implementation Units

### Unit 1: Prefer queue source during archive resolution

**Files:** `internal/core/archive.go`
**Test files:** archive consistency coverage
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first
**Dependencies:** none

**Acceptance criteria**

* archiving with duplicate queue/archive copies uses the queued artifact as source or returns a clear duplicate-ID error
* queue copy is not left behind after a successful archive

### Unit 2: Restore status on unarchive

**Files:** `internal/core/archive.go`
**Test files:** `internal/core/archive_test.go`
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first
**Dependencies:** Unit 1

**Acceptance criteria**

* unarchived items return to queue with a non-archived status
* restored items appear in normal list and queue flows

### Unit 3: Make AdoptItem rollback restore old identity fully

**Files:** `internal/core/shipment_lifecycle.go`
**Test files:** adopt rollback coverage
**Effort size:** medium
**Skill domain:** code
**Execution note:** characterization-first
**Dependencies:** none

**Acceptance criteria**

* failed adopt restores old path and old frontmatter ID
* DB and file state remain consistent after rollback
* sync does not rehydrate a mismatched old-path/new-ID artifact

### Unit 4: Make stash harvest rollback atomic across DB and stash file

**Files:** `internal/core/stash.go`
**Test files:** stash harvest coverage
**Effort size:** medium
**Skill domain:** code
**Execution note:** test-first
**Dependencies:** none

**Acceptance criteria**

* a failed stash JSONL write does not leave a second harvested artifact orphaned
* retrying a failed harvest does not duplicate artifacts or break stash lineage
* stash link and item state remain consistent after failure

## Dependency Graph

* Units 1 and 2 are a single archive-focused lane
* Units 3 and 4 are parallel rollback lanes

## Decisions

| # | Decision | Rationale |
|---|---|---|
| D1 | Treat all four issues as rollback-integrity work under one feature | they share the same recoverability standard |
| D2 | Prefer explicit failure over silent duplicate-path mutation | hidden lifecycle corruption is harder to recover than a surfaced error |

## Learnings Applied

* Crash-safe rename and rollback patterns must restore discoverable files, not
  temp artifacts
* Source-artifact cleanup patterns reinforce end-to-end lineage preservation

## Plan Hardening Signals (REQUIRED)

* public API, schema, or contract change: **absent**
* security, auth, permission, or compliance-sensitive behavior: **absent**
* migration, backfill, destructive data/config action, or irreversible step: **absent**
* external integration, operator checkpoint, or external dependency: **absent**
* high runtime, rollout, or rollback risk: **low**

Requires plan hardening: no

## Plan Review

**Gate Decision: PASS**

Review notes:

* each unit is independently executable within the 2-hour rule
* rollback-heavy work stays isolated from shipment and metadata subsystems
* lifecycle safety learnings are directly referenced for implementation
