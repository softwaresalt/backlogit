---
title: "011-S Agent-Automation Hooks: Post-Merge Closure"
description: "Operational closure record for shipment 011-S, PR #25, merge commit 86e07e2"
ms.date: 2026-04-11
ms.topic: reference
---

## Closure Summary

| Field | Value |
|---|---|
| Feature | 027-F: Agent-Automation Hooks for MCP Event Signals |
| Shipment | 011-S |
| PR | [#25](https://github.com/softwaresalt/backlogit/pull/25) |
| Merge commit | `86e07e2` |
| Mode | post-merge |
| Readiness | **READY WITH CONDITIONS** |
| Validation window | 48 hours post-merge |
| Owner | backlogit maintainers |

**Condition**: 027.004-T (built-in post-hook emitters) is deferred pending 007-DL
implementation. The hook event infrastructure (queue, checkpoints, MCP tools) is
fully operational; agents can poll and ack without emitters — the queue simply
remains empty until emitters are wired.

## Change Summary

This shipment delivers the core JSONL-backed hook event infrastructure for
agent-automation signalling.

| Area | Outcome |
|---|---|
| Hook event queue | `internal/events/HookEventWriter` — append-only JSONL at `.backlogit/hooks_queue.jsonl`, dual-layer lock (in-process mutex + cross-process `.lock` sidecar, 60 s TTL) |
| Per-consumer checkpoints | `internal/events/CheckpointStore` — atomic temp-file-then-rename, monotonic enforcement, per-consumer `sync.Map` mutex to prevent read-modify-write races |
| Poll/ack orchestration | `internal/events/PollHookEvents` + `AckHookEvents` — filters by consumer checkpoint, supports optional `DerivedSignalProvider` interface |
| MCP tools | `backlogit_poll_hook_events` and `backlogit_ack_hook_events` registered and wired through `internal/mcp/hook_tools.go` |
| HooksConfig schema | `HooksConfig` expanded with `EventThresholds`, `AgentSubscriptions`, `HookEventThresholds`; `hooks.yaml` written by `backlogit init` |
| Agent protocol | Stage and Ship agent files updated with Hook Event Consumption section; ack rules clarified (events array only, skip when empty) |
| Deferred | 027.004-T (emitters) blocked on 007-DL (`RegisterPostHook` API) |

## CI Status

| Check | Result |
|---|---|
| `test (1.23)` | ✅ pass |
| `test (1.24)` | ✅ pass |
| Copilot review threads | ✅ 10/10 resolved |

10 Copilot review comments were filed. Three were false positives (reviewer missed
cross-file changes). Six received code fixes in commit `7a3188c`: float64 fractional
validation, per-consumer checkpoint mutex, scanner buffer expansion, agent doc
clarification (×2), and stale test comment removal. One O(n) advisory was
acknowledged as a documented v1 trade-off.

## Healthy Signals

* PR #25 merged cleanly into `main` at `86e07e2`.
* `go test ./...`, `go vet ./...`, and `golangci-lint run` passed before merge.
* CI passed on both Go 1.23 and Go 1.24.
* Shipment `011-S` and all shipped tasks archived with merge traceability.
* MCP server starts and registers both hook tools without startup errors.
* `backlogit_poll_hook_events` with any `consumer_id` returns an empty `events`
  array when the queue file does not exist (graceful empty-queue handling).

## Failure Signals

* `backlogit_poll_hook_events` returns an error (not empty events) on a missing
  or unreadable `hooks_queue.jsonl`.
* `backlogit_ack_hook_events` with a fractional or zero seq returns success
  instead of a validation error.
* Concurrent ack calls for the same consumer advance the checkpoint backward
  (monotonic regression).
* Scanner token exceeded error in `ReadHookEvents` on event payloads larger than
  1 MiB.
* MCP server fails to start after `backlogit init` on a new workspace because
  `hooks.yaml` was not written by `WriteDefaults`.

## Monitoring Plan

| Surface | Check | Frequency |
|---|---|---|
| Queue file growth | Monitor `.backlogit/hooks_queue.jsonl` size over time | On operational review or when poll latency increases |
| Checkpoint freshness | Confirm `.backlogit/runtime/hooks/*.checkpoint.json` updates after ack | On hook consumption integration testing |
| Emitter wiring | When 007-DL ships, verify `AppendHookEvent` is called at `feature_review_ready` and `post_merge_closure` trigger points | On 007-DL shipment |
| Scanner limit | If event payloads include embedded diffs or large content, raise the 1 MiB buffer or switch to structured streaming | On hook event schema changes |

## Rollback Plan

**Rollback trigger:** MCP server instability, checkpoint corruption, or
`backlogit_poll_hook_events` returning incorrect events after merge.

**Rollback steps:**

1. Revert merge commit `86e07e2` on `main`.
2. Re-run `go test ./...` and `golangci-lint run` on the reverted state.
3. Delete `.backlogit/hooks_queue.jsonl` to reset the queue on any affected workspace.
4. Delete `.backlogit/runtime/hooks/` to reset all consumer checkpoints.
5. Reopen a backlog item referencing PR #25 and the failing surface.

**Data note:** The hook queue and checkpoint files are ephemeral (`runtime/` is
gitignored). Reverting the binary does not corrupt workspace state — agents
simply restart from seq=0 on next poll.

## Validation Window

48 hours post-merge. Confirm:

* No follow-up CI failures on the merged hook events scope.
* `backlogit_poll_hook_events` and `backlogit_ack_hook_events` behave correctly
  in agent sessions (Stage and Ship consumers).
* No checkpoint regression or queue file corruption reported.
* `backlogit init` on a fresh workspace writes `hooks.yaml` successfully.

## Follow-up Items

* 027.004-T: Built-in post-hook emitters — deferred until 007-DL ships the
  `RegisterPostHook` interface. Item is queued in backlog for the next 007-DL
  shipment cycle.
* O(n) `scanMaxSeq`: The current sequence counter scans the full queue on each
  append. A sidecar counter file or tail-read optimization is stashed for a
  future improvement cycle (acceptable for v1 low-volume lifecycle events).
* Queue compaction: No compaction or rotation policy exists for
  `hooks_queue.jsonl`. Long-running workspaces will accumulate events. Stashed
  for a future maintenance cycle.
