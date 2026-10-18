---
title: "Ship 154-S resumed after Stage contract amendment"
date: "2026-09-29"
agent: ship
shipment_id: 154-S
feature_id: 173-F
branch: feat/154-s-shipment-claim-scheduler-baseline-marker
status: active
---

## Resume and recovery

- The operator selected option 1 and explicitly authorized the governed unblock
  of `154-S`; that selection also selected
  `.backlogit/checkpoints/checkpoint-20260930-021714.json`.
- The checkpoint was valid, conforming, owned by Ship, and matched this branch,
  shipment, feature, and seven task IDs. It was resolved only after the unblock
  succeeded and the post-resume topology gate passed.
- Stage's commit `b49350d7` added the missing red-deliverable contracts to
  `173.006-T`, `173.007-T`, and `173.008-T`. All three section reads now return
  the canonical blocks.

## Task-state reconciliation

- Before unblock, `backlogit_query_sql` and the queue Markdown files agreed:
  `173-F` and all seven manifest tasks were `queued`. The blocked shipment's
  `member_status_snapshot` recorded all eight explicit members as `active`.
- The blocked envelope had a valid RFC3339 timestamp, exact manifest snapshot,
  checkpoint reference, and a correlated `.backlogit/ops` record with
  `schema_version: shipment-operation/v1`, `operation: block`,
  `phase: committed`, and `shipment_id: 154-S`. `backlogit_doctor` validated
  the shipment artifact.
- The approved `backlogit_unblock_shipment` call used `confirm: true` and
  `target: active`. Afterward, SQL and Markdown both reported the feature and
  all seven tasks `active`, matching the recorded snapshot. This is the
  operator-approved, shipment-specific bootstrap state; no task statuses were
  manually rewritten.
- The `post_claim` topology gate passed and confirmed `154-S` is the sole
  active shipment on the matching feature branch. The lifecycle topology gate
  passed after unblock. A pre-unblock lifecycle probe had correctly rejected
  the still-blocked record as `BACKLOG_UNAVAILABLE`; that probe was before the
  required unblock and is not the post-resume verdict.

## Pre-flight and frozen schedule

- `P-001`: no other active top-level feature/chore or active shipment was
  observed; predecessor shipments `155-S` and `182-S` are archived with
  `archived_status: shipped`.
- `go test -run=^$ -count=1 ./...` passed before build work.
- Live status sources agree: executable statuses are `queued`, `active`,
  `blocked`; terminal-success statuses are `done`, `archived`; every other
  catalog or off-catalog token is unsupported.
- Shipment `S` has eight explicit IDs: `173-F` plus seven tasks. Frozen
  `M` contains only `173.006-T`, `173.007-T`, `173.008-T`, `173.009-T`,
  `173.001-T`, `173.003-T`, and `173.005-T`; `173-F` is the excluded feature.
- The dependency operation produced five acyclic waves:
  1. `173.006-T`, `173.007-T`, `173.008-T`
  2. `173.009-T`
  3. `173.001-T`
  4. `173.003-T`
  5. `173.005-T`
- The three red-deliverable mappings are valid: `173.006-T` and
  `173.007-T` close at wave 3 via `173.001-T`; `173.008-T` closes at wave 2
  via `173.009-T`. No member declares a green-regression JSON block, so every
  frozen regression array is exactly `[]`.
- `wave-scheduler-sim.ps1 -VerifyAgainstQueue` returned
  `WAVE_SIM_OK: 186/186`; its output exercised the tracked `130-S` contract
  fixture. The `154-S` census and dependency partition were separately
  verified from its live shipment manifest, SQL artifact types, and dependency
  operation.

## Next step

Run the per-wave harness gate for wave 1 only, then build only its three
members. The approved `154-S` bootstrap exception is limited to the recorded
claim-activated status; dependency ordering, all harness gates, convergence,
review, CI, Copilot review, and the no-merge-without-approval rule remain in
force. No source changes, task implementation, PR, or merge have occurred yet.
