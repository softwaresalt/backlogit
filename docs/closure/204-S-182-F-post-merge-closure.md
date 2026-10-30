---
chunk_strategy: h1-h2-h3
closure_status: READY
compaction_status: done
description: "Post-merge closure for shipment 204-S and covering feature 182-F: option A scheduler-baseline marker lifecycle regression net."
doc_type: closure
schema_version: "1.0"
shipment_id: 204-S
feature_id: 182-F
source: docs/closure/204-S-182-F-post-merge-closure.md
title: "204-S / 182-F Post-Merge Closure"
---

# 204-S / 182-F Post-Merge Closure

## Release record

| Field | Value |
|---|---|
| Shipment | `204-S` |
| Covering feature | `182-F` |
| Tasks | `182.001-T` (1 task, 1 wave, `harness-exempt` verification-only) |
| Implementation PR | [#616](https://github.com/softwaresalt/backlogit/pull/616) (merge commit) |
| Merge commit | `196b2d81e30ea727b0ed7b390b8601da42de3c83` |
| Reviewed implementation HEAD | `3e2a46d26fe2f6c5ef011936622a270f4b271ca0` |
| Closure branch | `post-merge/204-scheduler-baseline-marker-option-a` |
| Closure status | `READY` |
| Context compaction | `done` (P-020 `compact-context` invoked) |

## Delivered

* New `internal/core/shipment_claim_marker_lifecycle_test.go` with `TestU4_OptionALifecycleInvariant` and `TestU4_TerminalAndGenericPathsKeepMarker`
* Lifecycle invariant covers claim, block, unblock to active and `ReturnBlockedItem`, asserting the marker in frontmatter and index with the three-part predicate
* Terminal and generic paths cover `ShipShipment`, abandoned and generic update or status moves
* No production code or design doc changed

## Gates

| Gate | Result |
|---|---|
| Exempt gate | Probe failed before the file existed; `EXEMPT_VERIFY_OK:182.001-T` afterwards |
| Local review (P-014) | `READY_WITH_FOLLOWUPS` (P0 0, P1 0, P2 2 addressed, P3 2 minor) |
| Copilot review (P-018) | `SATISFIED`; one thread fixed and resolved |
| CI | All required checks passed for the merged HEAD |
| Merge strategy (P-009) | Merge commit |
| Governed closure | `backlogit_ship_shipment` archived `204-S`, `182-F` and `182.001-T` with the merge SHA |

## Runtime verification and releasability

Releasability: `READY`. The change is tests-only and touches no runtime surface, so runtime verification is not applicable.

* Monitoring: the two U4 tests run in the standard CI suite
* Rollback: revert the merge commit
* Owner: backlogit maintainers
* Validation window: none required

## Compaction (P-020)

`compact-context` ran with `target: all`. The 18 age-eligible memory files are each cited by path from durable exec-plans, decisions and closure records, so nothing was moved and no reference was broken. Compacted summaries already exist for those release units.

## Environment note

The local `golangci-lint` binary fails to load export data (toolchain mismatch), which fails the `TestU175_*_LintFix` tests locally. CI is unaffected and passed.

## Follow-ups

* None stashed
