---
title: "Ship 154-S Wave 1 Lifecycle Block"
date: 2026-09-13
agent: ship
shipment_id: 154-S
feature_id: 173-F
status: blocked
---

## Outcome

Ship restored the operator-selected checkpoint
`checkpoint-20260914-054250.json`, completed the required Engram prune/gate,
resumed the recorded wave-admission cursor, and resolved only that checkpoint.

Wave 1 harnesses for `173.006-T` and `173.007-T` were created and committed in
`47518681df968a42786091e9ebf3231ad3ff169d`.

## Verification

* `go test -run=^$ -count=1 ./...` passed
* `go test -count=1 -run '^TestU0a_ClaimMarkerLifecycle$' ./internal/core`
  failed on the expected missing activation marker and rollback cleanup
* `go test -count=1 -run '^TestU0b_ClaimMarkerReadSurface$' ./internal/cli ./internal/mcp`
  failed on the expected missing CLI and MCP marker projections
* All declared characterization subtests passed independently
* Both tasks carry `harness-ready` and remain `queued`
* The worktree was clean at the committed RED baseline

## Blocker

The unforced lifecycle topology gate returned exit code 1:

```text
PREDECESSOR_NOT_SHIPPED: predecessor 153-S is not in a shipped terminal state
```

Gate details:

* Target shipment: `154-S`
* Predecessor: `153-S`
* Live predecessor status: `queued`
* Forced: `false`
* Telemetry log: unavailable

No Wave 1 task was claimed, so the blocked lifecycle gate did not strand an
active task.

## Telemetry Degradation

`backlogit_log_telemetry` remains MCP-only and is not available in this
runtime. The registry provides no CLI write fallback. This memory record and
the structured checkpoint preserve the gate evidence but are not represented
as an equivalent P-005 telemetry event.

## Resume

Resume from the lifecycle gate after either:

* shipment `153-S` reaches `shipped`, or
* the operator separately authorizes and audits a lifecycle force override

Existing pre-claim and post-claim overrides do not authorize lifecycle force.
Use RED baseline SHA
`47518681df968a42786091e9ebf3231ad3ff169d` for Wave 1 red-deliverable
validation.
