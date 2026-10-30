---
chunk_strategy: h1-h2-h3
closure_status: READY
compaction_status: done
description: "Post-merge closure for shipment 208-S and covering feature 207-F: complexity attribution and provenance for model routing."
doc_type: closure
schema_version: "1.0"
shipment_id: 208-S
feature_id: 207-F
source: docs/closure/208-S-207-F-post-merge-closure.md
title: "208-S / 207-F Post-Merge Closure"
---

# 208-S / 207-F Post-Merge Closure

## Release record

| Field | Value |
|---|---|
| Shipment | `208-S` |
| Covering feature | `207-F` |
| Tasks | `207.001-T` through `207.003-T` (3 tasks, 3 waves, all test-first) |
| Implementation PR | [#631](https://github.com/softwaresalt/backlogit/pull/631) (merge commit) |
| Merge commit | `5440495205a0bca484095fe526e53a19cb89709f` |
| Reviewed implementation HEAD | `d12496f17c7627d67efc7d74bda22b6da384e04c` |
| Closure branch | `post-merge/208-s-complexity-provenance` |
| Closure status | `READY` |
| Context compaction | `done` (P-020 `compact-context` invoked) |

## Delivered

* Task-only optional `complexity_source` (`human`, `agent`, `derived`) and `complexity_ruleset_version` header-def fields, with legacy generated header-def upgrade
* Core `SetArtifactComplexityWithProvenance` seam: provenance completeness, control-character rejection, atomic three-field clear, stale-provenance deletion on value-only updates, fail-closed `estimate_history` (`dimension: complexity`) audit event before the durable write
* Explicitly supplied empty provenance values are rejected (presence, not non-emptiness), found by Copilot review and fixed in `d12496f1`
* SQLite projection for tasks only, bound-parameter `ComplexitySource` and `ComplexityRulesetVersion` filters, and the two-axis `IsTaskRouteEligible` predicate
* CLI `--complexity-source` and `--complexity-ruleset-version` (default human), MCP `complexity_source` and `complexity_ruleset_version` (default agent, explicit human rejected), registry params, regenerated CLI reference

## Gates

| Gate | Result |
|---|---|
| Local review (standard, report-only) | `READY`, P0 0, P1 0, P2 0, P3 0 (re-reviewed after the fix commit) |
| CI (all required checks, current HEAD) | pass |
| P-018 Copilot-review gate | `SATISFIED` after two threads were fixed, replied to, and resolved |
| Merge strategy (P-009) | merge commit |
| Dark mode | `DARK_MODE_MERGE_AUTHORIZED` for PR 631, no admin fallback used |

## Runtime verification and releasability

The change touches the CLI and MCP update surfaces only. No deployed runtime, monitoring, or rollback surface is affected.

| Item | Value |
|---|---|
| Releasability | `READY` |
| Monitoring | none required |
| Rollback | revert merge commit `5440495`; existing `size` provenance and legacy tasks without complexity provenance are unaffected |
| Owner | backlogit maintainers |

## Closure notes

* `backlogit_ship_shipment` was invoked directly for governed closure; the MCP call timed out client-side while the server completed shipment, commit tracking, and archival of `208-S`, `207-F`, and the three tasks. The `shipment-reconcile` pre, safe-close, and post modes were not run, so no safe-close report exists. Archive integrity was verified by inspecting the working tree (no archive deletions)
* `compact-context` (`target: all`) was invoked: every stale memory file is still cited by checkpoints, plans, or decisions, so nothing was compacted (skip decision, not a failure)
* `golangci-lint` was unusable in the local environment (toolchain export-data mismatch); CI was the lint authority
* RED-phase commits include minimal declaration stubs so the harnesses compile and fail on assertions
* Source artifacts: no `source_stash_id` or `source_deliberation_id` artifacts were archived by Ship; provenance is untouched for Stage

## Follow-ups

* Downstream consumers of `IsTaskRouteEligible` (routing) are outside this shipment
* Consider restoring `golangci-lint` local usability (toolchain mismatch)
