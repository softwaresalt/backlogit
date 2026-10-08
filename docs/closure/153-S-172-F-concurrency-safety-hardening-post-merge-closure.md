---
chunk_strategy: h1-h2-h3
# Keep closure_status and compaction_status at the top level;
# pipeline-topology reads them via fm.get().
closure_status: READY
compaction_status: degraded
description: "Post-merge closure for shipment 153-S and covering feature 172-F, which hardened artifact-mutation writers for concurrency safety. Merged as PR #485 with a merge commit; shipped and archived; residual risks are tracked as P-021 deferred follow-ups."
doc_type: closure
schema_version: "1.0"
shipment_id: 153-S
feature_id: 172-F
source: docs/closure/153-S-172-F-concurrency-safety-hardening-post-merge-closure.md
title: "153-S / 172-F Post-Merge Closure"
---

# 153-S / 172-F Post-Merge Closure

## Release record

| Field | Value |
|---|---|
| Shipment | `153-S`: Concurrency-safety hardening for artifact-mutation writers |
| Covering feature | `172-F` |
| Tasks | `172.001-T` through `172.015-T` (15 tasks, all `done`, then archived) |
| Implementation branch | `feat/153-s-concurrency-safety-hardening-for-artifact-mutation-writers` |
| Implementation PR | [#485](https://github.com/softwaresalt/backlogit/pull/485) |
| Merge commit | `271c916e62a7c8c0c6851f57a497d2e1f14d8091` (merge commit strategy, P-009) |
| Merged at | 2026-10-08T07:05:39Z |
| Reviewed implementation HEAD | `1655bf2f` |
| Closure branch | `post-merge/153-s-concurrency-safety-hardening` |
| Run mode | Dark factory (P-017). Merge approval pre-authorized; admin fallback not authorized and not used. |
| Closure status | `READY` |
| Context compaction | `degraded`. See `2026-10-08-153-s-compaction-report.md`. |

## Shipped scope

* **Lock order.** Writers touch two cross-process lock domains: B, the
  artifact-mutation lock (`lockArtifactMutation`, `lockArtifactMutations`),
  and C, the item-log lock (`events.LockItemLogCrossProcess`). The canonical
  order is C before B when a writer must hold both. Standalone `ArchiveItem`
  and `archiveDescendants` cascade archival now release B before the deferred
  archive event acquires C, so B and C never overlap on those paths.
  `ShipShipment` still holds B while archival acquires C. That residual is
  documented and tracked as stash `E4908F30`. See
  `docs/design-docs/artifact-mutation-lock-order.md`.
* **Compare-and-swap guards.** Archive, archive-reconcile, and
  archived-status writers re-read the on-disk status under lock and refuse a
  stale write. This includes the reconcile rollback path that Copilot round 3
  flagged.
* **`BulkUpdateResult` conflict field.** Bulk status updates report per-item
  CAS conflicts rather than silently overwriting, and the CLI renders them.
* **B/C barrier seam.** Two nil-in-production acquisition hooks,
  `artifactMutationLockBarrierHook` for B (fires immediately after B is
  acquired) and `itemLogLockBarrierHook` for C (fires immediately before the
  cross-process item-log lock is acquired), let tests pause a writer at a lock
  and drive the B/C interleaving deterministically. `go test -race` is a
  secondary diagnostic only.

The work followed the wave harness discipline: each task landed a
source-shape RED harness, then the declaration, then the behavior harness,
then the GREEN implementation.

## Review record

| Pass | Result |
|---|---|
| Persona review, cycle 1 | Findings A to J; all in scope and fixed. |
| Adversarial review, 3 reviewers | Findings F1 to F6; in-scope findings fixed, the rest captured under P-021. |
| Copilot round 1 | 3 threads; fixed in `c89227e9`; replied and resolved. |
| Copilot round 2 | 1 PR-body finding; fixed in `bf6afac9`. |
| Copilot round 3 | 2 threads; fixed in `b7a06d53`; replied and resolved. |
| Copilot round 4 | 0 findings on `1655bf2f`. |
| `autoharness gate copilot-review` | `SATISFIED` |
| Required CI | Green on the merged HEAD. |

## Shipment reconciliation

* The pre-close `shipment-reconcile` run returned `PROCEED`: the 15 task
  members were `done`, covering feature `172-F` was `active` and classified
  `feature-pending-governed-completion`, and shipment `153-S` was active and
  unblocked. `ShipShipment` then marked `172-F` `done` during the governed
  transaction.
* Governed `backlogit_ship_shipment` was invoked with the merge SHA. The MCP
  client timed out. Following `2026-10-06-ship-shipment-mcp-timeout-result-unknown.md`,
  Ship did not retry. It observed the server-side call to completion: the
  shipment reached `shipped` at 07:14:45Z and archival finished at 07:30:33Z
  (hook sequence 3597).
* P-007 archive integrity check: clean, with no archive deletions in the
  working tree.
* The post-close report `.backlogit/reconcile/153-S-post-20261008T073458Z.md`
  recommends `CLOSED`.
* The archive state was committed as `c5d6bb06` on the closure branch.

## Provenance

`172-F` carries no `source_stash_id` or `source_deliberation_id`. No source
artifact was archived or mutated.

## Runtime verification and operational readiness

`backlogit` ships as a CLI binary and MCP server with no deployed service.
Runtime verification was the full local quality gate on the merged HEAD:
`go build ./cmd/backlogit`, `go test ./...`, `go vet ./...`,
`golangci-lint run`, and a CRLF-safe gofmt check. All passed.

The workspace has no monitoring system, so the release-observability
monitoring plan is recorded here as a manual observation requirement.

### Pre-deploy audit

* **Rollout gate:** none. The change ships in the next `backlogit` binary
  build; there is no feature flag.
* **Rollback procedure:** revert merge commit `271c916e` with
  `git revert -m 1 271c916e` on a branch, then merge through a PR.
* **Data and schema:** no artifact, index, or log format migration. The
  `BulkUpdateResult` conflict field is an additive JSON field.
* **Dependent services:** none. MCP clients see only the additive field and
  the new `ErrShipmentConflict` refusals on stale writes.

### Manual observation plan

| Item | Plan |
|---|---|
| Owner | Repository operator (softwaresalt). |
| Observation window | The first 7 days after merge (through 2026-10-15T07:05Z), or the first 10 CLI/MCP sessions that mutate artifacts, whichever is later. |
| Healthy signals | `go test ./...` stays green on `main`. `backlogit doctor` reports no orphans or duplicate IDs. Item logs show one event per governed mutation. |
| Failure signals | A CLI/MCP mutation hangs or times out on a lock. `BulkUpdateResult` reports conflicts with no concurrent writer. `ErrShipmentConflict` refusals appear on an uncontended path. `backlogit doctor` reports torn, duplicated, or orphaned artifacts. An archive event is missing from an item log. |
| Baseline | Before this change, no conflicts or CAS refusals were reported, because stale writes overwrote silently. |
| Alert threshold | Any single lock hang, or any reproducible conflict or refusal on an uncontended path. |
| Rollback trigger | A lock hang or deadlock in a CLI/MCP mutation, or `backlogit doctor` reporting artifact corruption attributable to these writers. Either triggers the revert above. |
| Outcome record | At window close, the owner records `healthy`, `degraded`, or `rolled back` as a comment on `172-F`. |

## Residual risks

These are tracked P-021 deferred scope expansions, not release conditions:

| Stash | Risk |
|---|---|
| `A0C92478` | `BulkUpdateConflict.Err` serializes as `{}` in JSON output. |
| `E4908F30` | `ShipShipment` holds B while archival acquires C. |
| `BACD94FC` | Archive-event ordering relative to the file move. |
| `517BAC76` | A C-before-B redesign for the remaining writers. |
| `DBF89C9C` | Post-archive side effects run outside the lock. |
| `36D86223` | `-race` CI job timeout. |
| `92AB6879` | Test flake observed during the run. |
| `95379DBA` | `pipeline-topology (ambient)` CI false positive (non-required check). |
| `A0C733C6` | Pre-existing entry, reused rather than duplicated. |

## Knowledge

* No design-doc graduation was needed. The lock-order and CAS rules live in
  the tests and the code comments the tasks added.
* `compound-refresh`: four related learnings were checked, and all are
  classified `keep`. The MCP-timeout learning gained a third corroborating
  case. The lock and re-persist learnings make no claim that 172-F changed.

## Context compaction (P-020)

`compact-context` was invoked with `target: all`. Eight aged memory notes were
summarized and their originals archived; `docs/memory/` remains over
threshold, so the status is `degraded`. Follow-up stash `A17EE897` already
tracks the next pass.

## Releasability

`READY`. All required gates passed, the shipment is closed, the pre-deploy
audit is complete, and the rollback procedure and trigger are documented.
The manual observation plan above is a post-release obligation for the
owner. It is not a pre-release condition, so it does not change the
releasability decision.