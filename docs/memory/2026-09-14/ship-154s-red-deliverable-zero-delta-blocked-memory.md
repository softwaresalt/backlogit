---
title: "Ship 154-S Red-Deliverable Zero-Delta Block"
date: 2026-09-14
agent: ship
shipment_id: 154-S
feature_id: 173-F
status: blocked
---

## Outcome

The audited lifecycle override was verified and applied only to shipment
`154-S` lifecycle predecessor checks. The gate returned exit code 0 with
`forced: true` and token `PREDECESSOR_NOT_SHIPPED` for queued predecessor
`153-S`.

Ship claimed `173.006-T` and dispatched its existing RED harness through the
`build-feature` red-deliverable branch. The harness itself is valid:

* Repository compilation passed
* The scoped selector exited 1 on named assertion failures
* No panic, timeout, build error, or vacuous selector was observed
* Lint passed
* No production or test files changed during dispatch

## Blocking Contract Conflict

The mandatory pre-claim baseline was:

```text
21d5af76ae4681de4058f84842cf7c340f9f4e1e
```

Ship Step 4.1b then performed the mandatory task claim. That governed mutation
changed:

```text
.backlogit/hooks_queue.jsonl
.backlogit/queue/173.006-T.md
```

The `build-feature` Step 0.5b zero-delta gate requires the union of tracked,
staged, and untracked changes against the pre-claim baseline to be completely
empty. It defines no exception for Ship's mandatory claim bookkeeping.
Therefore the task cannot simultaneously satisfy both contracts:

1. Capture `red_baseline_sha` before claiming the task
2. Claim the task before dispatch
3. Present an empty changed-file set against that pre-claim SHA

The dispatch halted with the effective classification
`RED_DELIVERABLE_DELTA_OUT_OF_SURFACE`. No workaround, baseline substitution,
stash, revert, or instruction edit was applied.

## Evidence

* Harness commit: `47518681df968a42786091e9ebf3231ad3ff169d`
* Pre-claim baseline used for dispatch:
  `21d5af76ae4681de4058f84842cf7c340f9f4e1e`
* Task: `173.006-T` is `active`
* Sibling `173.007-T` remains `queued`
* Deferred workflow defect: stash `7AA35A39`
* Lifecycle force audit:
  `.autoharness/gates/pipeline-topology-force-audit.log`

## Telemetry Degradation

`backlogit_log_telemetry` remains MCP-only and unavailable in this runtime.
The registry has no CLI write fallback. This memory record and the structured
checkpoint preserve the P-002.6 halt evidence but are not an equivalent P-005
telemetry event.

## Resume

Stage or the harness owner must reconcile the contradictory baseline/claim
contract before Ship can complete `173.006-T`. Ship must not:

* recapture the baseline after the claim
* exclude claim bookkeeping from the changed-file union without a contract
  amendment
* edit shared workflow instructions inside shipment `154-S`
* move the active task back to queued outside Ship's allowed transitions

After the contract is amended, resume the same task from its valid RED evidence
without changing the harness.
