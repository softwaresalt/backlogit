---
title: "New bug backlog grouping and staging decision"
date: 2026-05-22
status: approved-for-planning
stash_ids:
  - 250CC1F9
  - 3E33EE12
  - ED0DAA74
  - D3BC5A25
  - 3BA4181D
  - 88A64440
  - 0B9C7903
  - FE806724
  - 36F1CB1A
  - 5A41B7C3
  - 6DD3062F
  - 6235FF06
  - 51D7384A
  - EE33B6ED
  - 21E17BFC
---

# New bug backlog grouping and staging decision

## Problem frame

The new stash backlog is almost entirely bug-shaped work. The fastest safe path
to unblock Ship is to batch the bugs by subsystem and failure mode instead of
building one oversized stabilization shipment.

## Triage summary

| Stash ID | Priority | Kind | Classification | Notes |
|---|---|---|---|---|
| 250CC1F9 | critical | bug | task-shaped | dependency insert validation |
| 3E33EE12 | high | bug | task-shaped | queue dependency error handling |
| ED0DAA74 | high | bug | task-shaped | dependency scheduler semantics |
| D3BC5A25 | high | bug | task-shaped | archive duplicate path resolution |
| 3BA4181D | high | bug | task-shaped | unarchive status restoration |
| 88A64440 | high | bug | task-shaped | adopt rollback corruption |
| 0B9C7903 | high | bug | task-shaped | stash harvest orphaning |
| FE806724 | high | bug | task-shaped | shipment claim rollback |
| 36F1CB1A | medium | bug | task-shaped | stale blocked metadata |
| 5A41B7C3 | high | bug | task-shaped | metadata catalog CLI parity |
| 6DD3062F | high | bug | task-shaped | export-command-map path root |
| 6235FF06 | high | bug | task-shaped | section-write DB re-upsert |
| 51D7384A | high | bug | task-shaped | CLI section corruption |
| EE33B6ED | high | bug | task-shaped | MergeSync dry-run side effects |
| 21E17BFC | low | feature | feature-shaped | deferred contingency item |

## Grouping options considered

### Option A

1. Dependency and queue integrity
2. Archive and hierarchy rollback integrity
3. Shipment state integrity
4. Metadata, section, and sync contract integrity

**Assessment:** Best fit. Each batch shares code surfaces and can ship as a
coherent reviewable bugfix release.

### Option B

1. Core data integrity
2. CLI and MCP parity
3. Shipment and archive lifecycle

**Assessment:** Too broad. The first batch would exceed the 2-hour task rule
for several child tasks and mix unrelated recovery paths.

### Option C

1. Critical item only
2. All remaining bugs in one stabilization wave

**Assessment:** Unblocks the first defect quickly but leaves Ship with an
unreviewable second shipment.

## Selected direction

The operator requested autonomous staging, so Stage applied **Option A** as the
recommended grouping set.

## Covering features approved for planning

| Feature title | Included stash IDs | Scope estimate |
|---|---|---|
| Dependency Queue Integrity | 250CC1F9, 3E33EE12, ED0DAA74 | 3 tasks x 2h |
| Archive and Hierarchy Rollback Integrity | D3BC5A25, 3BA4181D, 88A64440, 0B9C7903 | 4 tasks x 2h |
| Shipment State Integrity | FE806724, 36F1CB1A | 2 tasks x 2h |
| Metadata and Section Sync Integrity | 5A41B7C3, 6DD3062F, 6235FF06, 51D7384A, EE33B6ED | 5 tasks x 2h |

## Deferred item

* `21E17BFC` remains deferred. It is a low-priority contingency feature and does
  not belong in the current bugfix shipments.

## Learnings applied

* `docs/compound/workflow-issues/orphaned-tasks-without-parent-features-2026-04-10.md`
  reinforces up-front validation for hierarchy and dependency integrity
* `docs/compound/workflow-issues/source-artifact-archival-pattern-2026-04-20.md`
  reinforces parent-first shipment manifests and source-artifact cleanup
* `docs/compound/best-practices/crash-safe-delete-rename-rollback-go-2026-04-23.md`
  reinforces rollback-safe file restoration
* `docs/compound/2026-05-07-mcp-cli-config-parity.md`
  reinforces CLI and MCP metadata parity
* `docs/compound/database-issues/atomic-rehydration-sqlite-transaction-2026-04-08.md`
  reinforces transaction boundaries for sync and rehydration operations

## Planning handoff

Proceed to four reviewed implementation plans, one per covering feature. Do not
mix the deferred singleton MCP server feature into any bug shipment.
