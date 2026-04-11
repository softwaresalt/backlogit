---
title: "006-S Event Traceability and Commit Tracking: Post-Merge Closure"
description: "Operational closure record for shipment 006-S, PR #21, merge commit 8e0dd27"
ms.date: 2026-04-10
ms.topic: reference
---

## Closure Summary

| Field | Value |
|---|---|
| Feature | 023-F: Event traceability and observability |
| Task | 023.008-T: Add commit traceability to event log entries |
| Shipment | 006-S |
| PR | [#21](https://github.com/softwaresalt/backlogit/pull/21) |
| Merge commit | `8e0dd27` |
| Fix commits | `1dbfe20` (Copilot review fixes) |
| Mode | post-merge |
| Readiness | **READY** |
| Validation window | 48 hours post-merge |

## Change Summary

Three implementation units shipped across `internal/events`, `internal/core`,
`internal/mcp`, and `tests/contract`. All implemented test-first against failing
harnesses, verified via full CI on both Go 1.23 and 1.24.

| Unit | Title | Package(s) |
|---|---|---|
| Unit 1 | Extend Event struct with CommitSHA | `internal/events` |
| Unit 2 | Thread commit SHA through core lifecycle | `internal/core` |
| Unit 3 | MCP tool parameter extension | `internal/mcp` |

**Net change:** 6 files, +259 / -4 lines, 10 new tests.

## CI Status

| Check | Go 1.23 | Go 1.24 |
|---|---|---|
| `go test ./...` | ✅ pass | ✅ pass |

Two CI rounds completed green (initial commit + fix commit). Two Copilot
review comments resolved in commit `1dbfe20`.

## Healthy Signals

* `go test ./...` passes on both Go 1.23 and 1.24 with no skipped tests.
* `backlogit_append_comment` with `commit_sha` parameter produces JSONL
  entries containing `"commit_sha":"<value>"`.
* `backlogit_archive_item` with `commit_sha` parameter produces archive
  events with `commit_sha` field populated.
* `backlogit_move_item` with `commit_sha` parameter emits `status_changed`
  events with `{from, to, reason}` delta schema plus `commit_sha`.
* All three tools work identically to prior behavior when `commit_sha` is
  omitted (backward compatible).
* Existing JSONL entries without `commit_sha` unmarshal correctly (zero value).

## Failure Signals

* **Event schema break:** If existing JSONL entries fail to unmarshal due to
  the new `CommitSHA` field, the `omitempty` tag is not working correctly.
  Check `internal/events/stream.go` for the `json:"commit_sha,omitempty"` tag.
* **Delta schema mismatch:** If `status_changed` events from `handleMoveItem`
  contain `{"status": ...}` instead of `{"from", "to", "reason"}`, the fix
  from `1dbfe20` regressed. Check `internal/mcp/tools.go` handler.
* **Silent event failures:** If `commit_sha`-bearing events are not appearing
  in JSONL logs, check the `slog.Warn` / `logger.Warn` output for append or
  index errors.
* **CI failures:** Any red check on `go test ./...` indicates a regression.

## Monitoring Plan

| Surface | Check | Frequency |
|---|---|---|
| Event schema | Unmarshal existing + new JSONL entries | On event read operations |
| Commit traceability | Verify `commit_sha` appears in events after tracked operations | After operations with commit_sha |
| Archive events | Confirm archive events carry `commit_sha` when `WithCommitSHA` option used | After archive operations |
| MCP tool schema | `commit_sha` parameter appears in tool introspection | On MCP server startup |

## Rollback Plan

**Rollback trigger:** Event deserialization failures, unexpected JSONL schema
breaks, or downstream telemetry correlator errors related to `status_changed`
delta format.

**Rollback steps:**
1. Revert to the prior release tag via `git revert 8e0dd27` or by creating a
   hotfix branch from `34ad6ce` (the pre-006-S main HEAD).
2. Rebuild and redeploy the binary.
3. Existing JSONL log files are not modified by this change. Events already
   written with `commit_sha` will simply have the field ignored by older code.
4. File an incident item in the backlog with the specific error + reproduction
   steps before reverting.

**Blast radius:** All changes are additive. No existing event fields, MCP tool
parameters, or core function signatures were removed. The only new behavior is:
- `Event` struct has one additional optional field
- `ArchiveItem` accepts variadic options (existing callers pass zero args)
- Three MCP tools accept one additional optional parameter

## Validation Window

48 hours post-merge. Evaluate:

* No JSONL deserialization errors in event read paths.
* `commit_sha` correctly appears in events when provided to MCP tools.
* No downstream telemetry correlator failures related to `status_changed` delta format.

## Follow-up Items

* External system synchronization (Azure DevOps, Jira, GitHub Issues) is out of
  scope per deliberation 011-DL. Tracked as a separate future feature.
* `AutoLinkCommits` back-fill of `CommitSHA` on historical events was deferred
  (append-only logs should not be rewritten).
