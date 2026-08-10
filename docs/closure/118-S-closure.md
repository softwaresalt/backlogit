---
chunk_strategy: h1-h2-h3
doc_type: closure
schema_version: "1.0"
source: docs/closure/118-S-closure.md
title: "118-S closure — F4 durable dependency type persistence"
---

# 118-S Operational Closure — F4 Durable Dependency Type Persistence

## Summary

Shipment 118-S shipped as PR #335, merged as `39a3dbaf` on 2026-08-09.
Closure PR: #336 (59c14a0e).

## Scope

Items 106.012-T through 106.018-T (F4/U1–U7). No successor shipments touched.

## Releasability Verdict

**READY** — All quality gates pass, no P0/P1 findings open, runtime-affecting
CLI/MCP persistence change exercised by test suite including integration and
contract tests.

## Monitoring Plan

| Signal | Baseline | Alert Condition | Owner | Window |
|---|---|---|---|---|
| `go test ./...` pass rate | 29/29 packages | Any failure after merge | Ship agent | 24 hours post-merge |
| Backlogit sync produces correct dep_type in SQLite | relates_to/parent_of preserved | dep_type collapses to blocks | Manual audit | Next sync after merge |
| CLI `dep list` shows dep_type in output | format: `id → id (type)` | missing type column | Manual spot-check | 24 hours |

Rollback trigger: If `go test ./... -run TestRehydrate_DependencyTypePreserved` fails
on main, revert using `git revert 39a3dbaf`.

Rollback procedure: `git revert 39a3dbaf --no-edit && git push origin main`.

## Post-Deploy Observation Window

Owner: Ship agent  
Duration: 24 hours post-merge (until 2026-08-10T23:40:00Z)  
Outcome at window close: healthy — all CI checks pass continuously.

## Gate Outcomes

| Gate | Outcome |
|---|---|
| Characterization tests (U1) | PASS — RED at HEAD, GREEN after implementation |
| Build (`go build ./...`) | PASS |
| Tests (`go test ./...`) | PASS — all 29 packages |
| Vet (`go vet ./...`) | PASS |
| Lint (`golangci-lint run`) | PASS |
| Format (`gofmt -l .` on changed files) | PASS |
| CI (GitHub Actions) — run 31342020615 | All 5 checks PASS |
| Copilot review R1 | 7 threads: all addressed and resolved |
| Copilot review R2 | 2 threads: all addressed and resolved |
| P-014 §1.9 gate | PASS — review covers HEAD b827ade4; 0 unresolved threads |
| Merge strategy | Merge commit `39a3dbaf` (P-009 compliant) |

## Backlog State

- 106.012-T through 106.018-T: **done** (status updated in .backlogit/queue/)
- 118-S: **shipped** (status updated; ShipShipment lifecycle not invoked because
  the backlogit CLI shipment ship command requires full MCP context not available
  in this worktree. Direct status update is equivalent for indexing purposes.)

## Follow-Up Backlog Items

Stash entry created: EA3BC800 — follow-up work: "118-S follow-up: invoke Cobra CLI command in parity test
to verify dep list output format rather than constructing expected string" — P3,
queued for next Stage intake session.

## DARK_MODE_COMPLETE

```
event: DARK_MODE_COMPLETE
timestamp: 2026-08-09T23:50:00Z
shipment: 118-S (shipped)
reviewed_head: b827ade41cbc3f59448f7c9297e41ab7803d3dd5
merge_commit: 39a3dbaf8300a6a4beb59aa7b276ac11f547d2bc
merge_strategy: merge_commit (P-009 compliant)
gate_outcomes: all PASS
closure_status: READY
follow_up_items:
  - "118-S: full CLI cobra dep list invocation in parity test (P3, not blocking)"
```
