---
type: operational-closure
title: "Operational Closure — 127-S shipped-event audit durability"
doc_type: closure
source: 127-S
chunk_strategy: h1-h2-h3
schema_version: "1.0"
shipment: 127-S
feature: 143-F
timestamp: 2026-08-19T03:20:00Z
merge_sha: 817d46794342f0747f381e4d42e899d75d01c3cf
pr_number: 367
status: READY_WITH_CONDITIONS
---

## Gate Outcomes

| Gate | Result |
|---|---|
| go build ./... | PASS |
| go vet ./... | PASS |
| golangci-lint (core, mcp packages) | PASS |
| gofmt (changed files) | PASS |
| Scope tests (new + affected) | PASS |
| CI (GitHub Actions — 5 checks) | ALL PASS |
| Copilot review | PASS (3 cycles, all findings fixed, 0 unresolved threads) |
| Pre-merge readiness gate (§1.9) | PASS (all 3 checks green) |
| P-009 merge-commit-only | PASS (merge commit used) |
| P-016 topology | PASS (single active implementation worktree) |

## Commits (14 + review fixes)

- 65b9cd38 test(core): RED harness for governed shipped-event durability
- 7401e69a test(core): RED harness for shipped-event reconciliation audit
- 2937b1bf feat(core): add shipment-scoped error-returning event appender
- 6bd899b8 feat(core): fail closed on the governed shipped-event append
- 6cc971b1 feat(core): classify the shipped-append failure and halt on indeterminate
- 77fdf6ae fix(core): make ship compensation honest and correct the defer order
- b0b2d0d0 feat(core): add report-only shipped-event reconciliation audit
- 867f4095 feat(cli): add --check-shipped-event-completeness to doctor
- 54679d02 feat(mcp): expose check_shipped_event_completeness on backlogit_doctor
- e5e5a6fd feat(mcp): key shipped-event recovery guidance on the producer
- 70e5afb8 docs(harness): carry the shipped-event indeterminate branch across surfaces
- d6e8546b test(core): drop the unused shipped-event helper
- 67629b82 fix(core): add path containment check and expand feature descendants
- 4d7f7b4e chore(harness): archive 143-F tasks and update 127-S lifecycle state
- ab0e3a31 fix(core): address copilot review findings on containment and path safety
- 80d6d8b2 fix(core): apply real-path containment anchored on storage root
- 97f1fd3b fix(core): restore two-layer logs containment to block subtree escapes
- 817d4679 MERGE COMMIT (PR #367)

## Shipment State

- 127-S: shipped and archived (commit 817d4679)
- 143-F: done, archived
- 143.001-T through 143.012-T: all done, archived (13 items in .backlogit/archive/)

## Monitoring Plan

This is a CLI/MCP library feature with no persistent service. Monitoring is manual:

| Signal | Baseline | Alert threshold | Owner |
|---|---|---|---|
| Doctor audit false-positive rate | 0 false positives | Any finding on a cleanly-shipped shipment | Operator |
| Shipped-event present after clean ship | 100% presence | Absence on any clean-path ship | Operator |
| Indeterminate ship incidents | 0 per release | Any occurrence triggers reconciliation check | Operator |

Post-deploy validation: run `backlogit doctor --check-shipped-event-completeness` after any ship operation to confirm the audit reports clean.

Rollback trigger: if `check-shipped-event-completeness` reports `FindingShippedUnarchivedResidue` on a cleanly-shipped shipment, roll back the ship attempt and file a bug.

## Conditions (READY_WITH_CONDITIONS)

`shippedEventPresence` in `internal/core/doctor.go` received a real-path EvalSymlinks fix in cycle 2 (commit `80d6d8b2`) but does not verify that the resolved logs directory remains under the workspace storage root. A symlinked `logs` directory can cause the doctor audit to read outside the workspace. This is a defense-in-depth gap — the read-only audit cannot write outside the workspace, but the isolation boundary is not fully enforced. Tracked as follow-up item below.

## Copilot Review Summary

Cycle 1 (3 findings): lexical-only path check, status-only archive filter, hardcoded .backlogit path
Cycle 2 (2 findings): additional lexical check in shippedEventPresence, symlinked logs dir bypass
Cycle 3 (1 finding): storage-root anchor permits subtree escapes within logs/
All 6 findings were valid, fixed, replied to, and threads resolved before merge.

## Residual Risks

- The full test suite takes ~10+ minutes due to pre-existing test volume in internal/core; not a regression
- Stash item 47B48DB0 (prevention of non-ShipShipment shipped-event producers) is explicitly excluded and remains active
- `shippedEventPresence` lacks storage-root anchor for logsDir (see Conditions section above)

## Follow-up Items

- 47B48DB0: complement the detection audit with a producer restriction (scope excluded from 127-S)
- FOLLOWUP-127S-1: add storage-root anchor check to `shippedEventPresence` in `internal/core/doctor.go` to match the pattern used in `appendShipmentEventErr`
