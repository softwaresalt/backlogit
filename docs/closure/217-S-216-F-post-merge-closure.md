---
title: "217-S / 216-F post-merge closure"
description: "Post-merge operational closure and releasability evidence for Shipment 217-S (Feature 216-F / CAP-3)."
doc_type: closure
source: docs/closure/217-S-216-F-post-merge-closure.md
ingested_at: "2026-11-18T03:19:00Z"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
closure_status: READY
compaction_status: done
docline:
  shipment_id: 217-S
  feature_id: 216-F
  merge_commit: 75e27861fac309e79113c270bcaba1927dfdc767
  close_path: cascade
  close_evidence: docs/closure/evidence/217-S-216-F-close-evidence.json
  closure_status: READY
  compaction_status: done
---

## Release and governed scope

PR `#656` (`https://github.com/softwaresalt/backlogit/pull/656`,
`headRefOid: 7835aa223724db8e0776db53f03f95b98b3bddb6`) passed current-HEAD
local multi-persona, adversarial, correctness, and constitution reviews
(`READY_WITH_FOLLOWUPS`, `P0=0, P1=0` in-scope), 8/8 CI checks, P-018
`autoharness gate copilot-review 656` (`SATISFIED`, `exit_code: 0`,
`PRR_kwDORzozKM8AAAABYqZyew`), and P-009 merge-commit-only settings, and merged
under `DARK_MODE_ACTIVE` pre-authorization (`merge_approval_pre_authorized: true`)
via normal merge commit `75e27861fac309e79113c270bcaba1927dfdc767` at
`2026-11-18T02:49:58Z`.

Local `main` was synchronized via fast-forward-only sync
(`HEAD == origin/main == 75e27861fac309e79113c270bcaba1927dfdc767`) before
creating `post-merge/217-s-closure`. Lifecycle topology gate returned
`BRANCH_POST_MERGE_CLOSURE_ELIGIBLE` (`exit_code: 0`).

Flat explicit membership is `M = [216-F, 216.001-T..216.008-T]` plus shipment
control `217-S`. Pre-close, safe-close, and post-close reconciliation passed
under one held `.backlogit/queue/217-S.md` agent-lock, which was released at
post-mode completion. `217-S` is archived with `archived_status: shipped`;
`216-F` and `216.001-T`..`216.008-T` are archived with `archived_status: done`
and merge commit `75e27861fac309e79113c270bcaba1927dfdc767`. Zero non-member
changes and zero tracked archive deletions occurred (`P-007 PASS`).

| Governing report | Result |
| --- | --- |
| `.backlogit/reconcile/217-S-pre-20261118T025500Z.md` | `PROCEED` |
| `.backlogit/reconcile/217-S-safe-close-20261118T031000Z.md` | `CLOSED` |
| `.backlogit/reconcile/217-S-post-20261118T031100Z.md` | `PROCEED` |

Canonical governed evidence: [217-S close evidence](evidence/217-S-216-F-close-evidence.json)
(`shipment_status: shipped`, `returned_ids: []`, `phase: post_close`,
`postcondition_verdict: pass`, `failures: []`, invoked via
`backlogit_ship_shipment` MCP tool).

## Invariants and validator evidence

* `AssociateCommit` and `LinkCommit` (`internal/core/commits.go`) enforce canonical lock ordering `lifecycle-global -> C -> B` across persistence and compensation and fail closed with `*ShipmentLifecycleLockOrderError` on preheld `C` without verified `lifecycle-global` ownership (`216.001-T`).
* `events.ReadAllEvents` (`internal/events/reader.go`) uses `64 KiB` initial / `1 MiB` max scanner buffer and surfaces scanner/context errors (`216.002-T`).
* `findArtifactWithPath` (`internal/core/artifacts.go`) and `snapshotShipArtifacts` (`internal/core/shipment_lifecycle.go`) perform contained direct `{id}.md` lookup before `WalkDir` fallback (`216.003-T`).
* `isAncestor` (`internal/core/shipment_gate.go`, `internal/core/workspace.go`) runs `git --no-replace-objects merge-base --is-ancestor` with `--`, symlink/junction root resolution, and bounded 4,096-entry memoization strictly guarded by `.git/shallow` and `.git/info/grafts` absence (`216.004-T`).
* `db.Open` (`internal/db/connection.go`, `internal/db/schema.go`) orders `_pragma=busy_timeout(30000)` first and materializes persistent Page 1 WAL header `[2, 2]` on empty `.db` files with bounded concurrent retry (`216.005-T`).
* `custodyGit` (`internal/core/custody_exec.go`) bounds subprocesses (`15s` timeout, `WaitDelay = 2s`, `GIT_TERMINAL_PROMPT=0`), `custodyBlobDigest` validates OIDs and deduplicates across identical blobs, and `custExecNewRepo` retains default synchronous safety (`216.006-T`).
* `newAuthSeamFixture` (`internal/core/shipment_reconcile_auth_seam_test.go`) uses `sync.Once` (`authSeamSeed`), and `TestUR1B` (`internal/core/shipment_blocked_lifecycle_harness_test.go`) bounds child subprocess readiness (`216.007-T`).
* `lockArchiveGovernance` (`internal/core/archive.go`) emits deterministic `"attempt_membership"` and `"attempt_item_log"` hook stages for load-hardened lock-contention tests (`216.008-T`).

See [post-merge runtime verification](217-S-post-merge-runtime-verification.md)
for full probe output (`verdict: PASS`).

## Pre-deploy audit and rollout

| Check | State |
| --- | --- |
| Deployment / rollout path | Merge-only CLI/library release; no database migration required |
| Pre-deploy audit | All 50/50 Go packages, `-race` (`internal/{db,events,core}`), `go vet`, `golangci-lint`, `gofmt`, `md-lint`, `docs lint`, and `wave-scheduler-sim` passed |
| Post-deploy checks | Post-merge `TestU1_`..`TestU7_`, `TestU172_LockOrder_`, `TestArchiveItemGovernance_`, `go build ./cmd/backlogit`, isolated CLI `init`/`add`/`update --commit`/`sync` WAL Page 1 `[2, 2]` probe, and `docs lint` passed on `main` |
| Risky action: governed cascade close | `ActionRisk: destructive`; authorized via `DARK_MODE_ACTIVE` pre-authorized release closure; `ActionResult: applied` (`shipment_status: shipped`, `returned_ids: []`) |

## Monitoring plan, owner, and rollback

* **Owner**: `_Ship` agent (closure verification); operator (rollback authorization); `_Stage` agent (deferred follow-up triage).
* **Validation window**: Immediate post-merge checks (passed) plus 7-day observation window (`2026-11-18` through `2026-11-25`).
* **Healthy signals**: All 50 Go packages, `golangci-lint`, `go vet`, and `go build ./cmd/backlogit` pass on `main`; `ShipShipment` closes shipments without MCP `-32001` stalls when rebuilt binary is deployed; zero `no such table: commit_links` or lock-order inversion errors.
* **Failure signals / rollback trigger**: Any regression in commit-writer lock ordering, `ShipShipment` closure gate integrity, SQLite WAL pool initialization, or custody verification on `main`.
* **Rollback procedure**: Preserve archived backlog/event history and submit an operator-approved scoped mitigation PR (or coordinated `git revert -m 1 75e27861fac309e79113c270bcaba1927dfdc767` excluding `.backlogit/archive/` history).

## Source artifact provenance

Feature `216-F` and tasks `216.001-T`..`216.008-T` record source stash IDs
`703835DC`, `76553D8D`, `F448AA52`, `9CFC88E5`, `B25BF131`, and `63909DAE`, and
deliberations `096-DL`..`101-DL`
(`docs/decisions/2026-11-15-cap3-zero-flake-concurrency-ship-closure-deliberation.md`,
archived `.backlogit/archive/096-DL.md`..`101-DL.md`). All referenced source
stash and deliberation artifacts are retained untouched as provenance only.

## Tracked residual risks and P-021 C2 deferrals

* Unnamed `df4a2df9` `internal/core` panic (`63909DAE` / `101-DL`, `docs/closure/197-S-197-F-post-merge-closure.md:132-137`) remains explicitly unresolved as a tracked diagnostic risk (unproven origin) until a reproducing test name and stack trace are captured.
* Four out-of-scope P-021 C2 `DEFERRED SCOPE EXPANSION` entries (`451486E7`, `FDFC52FF`, `DA6809A0`, `D1CA631D`) remain active in `.backlogit/stash.jsonl` awaiting Stage triage.

## Knowledge refresh, P-020 compaction, and follow-ups

* Invoked `compound-refresh` (`scope: recent`, `mode: propose`;
  [217-S compound refresh](217-S-compound-refresh.md)): 5 compound entries
  reviewed and kept (`keep: 5`).
* Invoked `compact-context` (`target: all`;
  [217-S compaction report](2026-11-18-217-s-compaction-report.md)): created
  [217-S compacted memory](../memory/compacted/2026-11-18-217-s-216-f-compacted.md)
  and finalized `compaction_status: done`.

## Structured releasability and compaction

| Evidence | Profile requirement | State |
| --- | --- | --- |
| Validator evidence | `required: false`, `minimum_verdict: PASS` | Satisfied (`PASS`) |
| Monitoring plan | `required: false` | Recorded |
| Rollback procedure | `required: false` | Recorded |
| Owner and validation window | `required: false` | Recorded (`_Ship`, 7 days) |
| Final releasability | `required: true` (`status_when_satisfied: READY`) | `READY` |
| P-020 compaction status | Mandatory post-merge closure duty | `done` |
