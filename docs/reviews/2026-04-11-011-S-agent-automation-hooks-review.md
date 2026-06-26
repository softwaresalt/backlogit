---
chunk_strategy: h1-h2-h3
description: ""
doc_type: review
docline:
    branch: ship/011-S-agent-automation-hooks
    date: 2026-04-11T00:00:00Z
    shipment: 011-S
    status: PASS
ingested_at: "2026-06-26T02:33:53Z"
schema_version: "1.0"
source: docs/reviews/2026-04-11-011-S-agent-automation-hooks-review.md
title: 'Review: 011-S Agent Automation Hooks'
---

## Summary

Report-only review of shipment 011-S (`ship/011-S-agent-automation-hooks`).
11 files changed, 844 lines added. All quality gates pass.

## Findings

### P2 — Fixed

**MCP-1: `handleAckHookEvents` did not validate `seq >= 1`**

The `AckHookEventsRequest` struct carries `validate:"required,gte=1"` but the
handler extracted parameters manually without enforcing the lower bound. A
caller passing `seq=0` would succeed silently (since `0 < 0` is false for a
fresh checkpoint). Fixed by adding an explicit `seq < 1` guard returning
`ValidationFailed`.

**MCP-2: `ErrValidation` from `AckHookEvents` mapped to `InternalError`**

When `SaveCheckpoint` rejected a regressive sequence number, the handler
wrapped the error as `InternalError`. Callers would see a confusing "internal
error" message for a caller-controlled input constraint. Fixed by checking
`errors.Is(ackErr, backlogiterrors.ErrValidation)` and returning
`ValidationFailed` in that path.

Both fixes are covered by two additional contract tests:
`TestAckHookEvents_Handler_ZeroSeq_ReturnsError` and
`TestAckHookEvents_Handler_RegressionSeq_ReturnsError`.

### P3 — Advisory (no action required)

**GQ-1: `scanMaxSeq` is O(n) per append**

`HookEventWriter.AppendHookEvent` scans all JSONL lines to find the max
sequence number on each write. For v1 with low event volume this is
acceptable. A separate counter file or in-memory cache would be the right
optimization when queue depth grows beyond a few hundred events.

**GQ-2: `HookEvent.Payload` uses `any`**

Flexible payload typing is required for v1 since each event type carries
different data. The field is documented in the plan as intentionally untyped.
Consider a typed union (`json.RawMessage` + per-type unmarshal) in a future
release.

## Verification

| Gate | Result |
|---|---|
| `go test ./...` | PASS (16 packages) |
| `golangci-lint run` | PASS (0 findings) |
| `go vet ./...` | PASS |
| `gofmt -l .` | PASS |
| Contract tests (10 tests) | PASS |
| Internal events tests (28 tests) | PASS |

## Blocked Items

**027.004-T** remains returned from the shipment. It requires `RegisterPostHook()`
from 007-DL (not yet implemented). No impact on shipped scope.
