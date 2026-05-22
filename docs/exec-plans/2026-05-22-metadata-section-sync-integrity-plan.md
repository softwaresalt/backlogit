---
title: "Metadata and Section Sync Integrity"
date: 2026-05-22
origin: "docs/decisions/2026-05-22-new-bug-backlog-grouping.md"
status: reviewed
stash_ids:
  - 5A41B7C3
  - 6DD3062F
  - 6235FF06
  - 51D7384A
  - EE33B6ED
---

## Problem Frame

The metadata and sync surfaces drift across CLI, MCP, file rewrite, and SQLite
indexing paths. These bugs are contract-level correctness issues: the tool
reports one thing, writes another thing, or mutates state during an advertised
dry run.

## Requirements Trace

| # | Requirement | Origin |
|---|---|---|
| R1 | metadata catalog must expose CLI command data in MCP just as it does in CLI | stash 5A41B7C3 |
| R2 | export-command-map must resolve paths from the workspace root consistently across CLI and MCP | stash 6DD3062F |
| R3 | section writes must re-upsert the rewritten body so DB and FTS stay current | stash 6235FF06 |
| R4 | CLI `update --section` must surface malformed-section errors instead of silently duplicating blocks | stash 51D7384A |
| R5 | MergeSync dry-run must not mutate the SQLite index | stash EE33B6ED |

## Scope Boundaries

### In Scope

* metadata catalog and command-map parity
* section write consistency between file and DB
* MergeSync dry-run contract enforcement
* regression tests for CLI and MCP parity

### Non-Goals

* new metadata features
* telemetry schema expansion
* shipment or archive lifecycle fixes

## Implementation Units

### Unit 1: Restore CLI metadata parity in MCP catalog

**Files:** `internal/cli/metadata.go`, `internal/mcp/metadata.go`, `internal/core/metadata_catalog.go`
**Test files:** metadata catalog coverage
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first
**Dependencies:** none

**Acceptance criteria**

* metadata catalog includes CLI command section from MCP path
* export-command-map has access to the same catalog data in CLI and MCP

### Unit 2: Align export-command-map path anchoring

**Files:** `internal/mcp/metadata.go`, `internal/core/metadata_catalog.go`
**Test files:** metadata catalog path coverage
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first
**Dependencies:** Unit 1

**Acceptance criteria**

* CLI and MCP write the same relative target path under the workspace root
* valid workspace-relative paths no longer fail containment checks spuriously

### Unit 3: Re-upsert section writes after file rewrite

**Files:** `internal/mcp/tools.go`
**Test files:** section write coverage
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first
**Dependencies:** none

**Acceptance criteria**

* section updates keep `items.description` and FTS in sync with rewritten Markdown
* get-item without section view reflects updated body immediately

### Unit 4: Stop CLI section corruption fallback

**Files:** `internal/cli/update.go`
**Test files:** CLI section update coverage
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first
**Dependencies:** none

**Acceptance criteria**

* malformed section markers raise an error
* CLI no longer appends duplicate section markers on unrelated write failures

### Unit 5: Enforce true dry-run behavior in MergeSync

**Files:** `internal/db/merge_sync.go`, MCP merge-sync surface if required
**Test files:** merge-sync unit and integration coverage
**Effort size:** medium
**Skill domain:** code
**Execution note:** test-first
**Dependencies:** none

**Acceptance criteria**

* `dry_run=true` performs no database writes
* fallback-triggered rehydration is skipped or simulated in memory during dry run
* existing dry-run tests and a regression for the fallback path pass

## Dependency Graph

* Units 1 and 2 share the metadata parity lane
* Units 3, 4, and 5 are parallel correctness lanes

## Decisions

| # | Decision | Rationale |
|---|---|---|
| D1 | Treat parity and write-contract bugs as one release unit | they all correct mismatches between reported and actual behavior |
| D2 | Prioritize dry-run purity as a contract guarantee, not a best effort | mutating state during dry run breaks operator trust and downstream tooling |

## Learnings Applied

* MCP and CLI config parity must be verified on both code paths
* atomic rehydration guidance reinforces why dry-run and write boundaries must be explicit

## Plan Hardening Signals (REQUIRED)

* public API, schema, or contract change: **present** but bugfix-only
* security, auth, permission, or compliance-sensitive behavior: **absent**
* migration, backfill, destructive data/config action, or irreversible step: **absent**
* external integration, operator checkpoint, or external dependency: **absent**
* high runtime, rollout, or rollback risk: **low**

Requires plan hardening: no

## Plan Review

**Gate Decision: PASS**

Review notes:

* each unit maps to one concrete defect and stays backlog-sized
* the plan preserves separation between metadata parity and database write semantics
