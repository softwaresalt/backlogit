---
title: "033-S Hooks System: Post-Merge Closure"
description: "Operational closure record for shipment 033-S, PR #36, merge commit 1f927c9"
ms.date: 2026-04-14
ms.topic: reference
---

## Closure Summary

| Field | Value |
|---|---|
| Feature | 033-F: Two-Layer Hooks System |
| Shipment | 033-S |
| PR | [#36](https://github.com/softwaresalt/backlogit/pull/36) |
| Merge commit | `1f927c9` |
| Fix commits | `4e9864b` (review gate), `cc79594` (Copilot review, 5 comments) |
| Mode | post-merge |
| Readiness | **READY** |
| Owner | dewilliams |
| Validation window | 48 hours post-merge |

## Change Summary

Ten tasks shipped across two new packages and six modified packages. All
implemented against the reviewed execution plan at
`docs/exec-plans/2026-04-14-hooks-system-plan.md`.

| Task | Title | Package(s) |
|---|---|---|
| 033.001-T | HookRunner core engine | `internal/hooks` |
| 033.002-T | Built-in pre-hook: ValidateStatusTransition | `internal/hooks` |
| 033.003-T | Built-in post-hooks: EmitHookEvent, LogIndexStale | `internal/hooks`, `internal/errors` |
| 033.004-T | Hooks configuration schema and loader | `internal/config` |
| 033.005-T | Wire HookRunner into Workspace | `internal/core` |
| 033.006-T | Instrument CreateArtifact and UpdateArtifact | `internal/core` |
| 033.007-T | Instrument ArchiveItem, MoveShipmentStatus, ShipShipment, AdoptItem | `internal/core` |
| 033.008-T | WebhookNotifier with async dispatch | `internal/hooks` |
| 033.009-T | Wire WebhookNotifier into Workspace | `internal/core` |
| 033.010-T | Integration tests for webhook dispatch | `tests/integration` |

**Net change:** 43 files, +2225 / −13 lines.

## CI Status

| Check | Go 1.23 | Go 1.24 |
|---|---|---|
| `go test ./...` | ✅ pass | ✅ pass |
| `golangci-lint run` | ✅ pass | ✅ pass |
| `go vet ./...` | ✅ pass | ✅ pass |

Both CI matrix runs completed green on commit `cc79594` before merge. The
`ci.yml` lint timeout was increased from default (1m) to 5m to accommodate
growing codebase analysis time.

## Copilot Review Resolution

| Round | Comments | Resolved |
|---|---|---|
| Code review gate (`4e9864b`) | 2 | 2 fixed |
| Copilot PR review (`cc79594`) | 5 | 5 fixed |
| **Total** | **7** | **7** |

Notable fixes with architectural significance:

**Rate limiter placement:** `dispatchToEndpoint` originally called
`rateLimiter.Wait(context.Background())` synchronously before spawning the
goroutine, which could block `FirePost` under load. Moved into the goroutine
so post-hooks are fully non-blocking as designed.

**HTTP connection cleanup:** `WebhookNotifier.Shutdown()` now calls
`CloseIdleConnections()` on the HTTP client after draining the WaitGroup,
preventing connection pool leaks in long-running processes.

**OldValues accuracy:** `UpdateArtifact` post-hook was using `artifact.Title`
after mutations were applied, producing identical old/new values. Fixed by
capturing `previousTitle` before the update block, matching the existing
`previousStatus` pattern.

**ShipShipment post-hook:** Hardcoded `models.StatusActive` replaced with
`shipment.Status` for accuracy if the validation guard ever changes.

**env: prefix removal:** `LoadHooks` accepted both `$VAR` and `env:NAME` for
webhook header values, but `os.ExpandEnv` only resolves `$VAR` syntax. Dropped
`env:` support entirely; only `$` prefix is valid.

## Architecture: Two-Layer Hooks System

### Layer 1: Synchronous Internal Hooks

The `HookRunner` in `internal/hooks/hooks.go` provides a priority-ordered,
point-scoped hook registry with two fire modes:

- **FirePre**: Executes hooks in priority order (ascending). First error stops
  the chain and the caller aborts the lifecycle operation.
- **FirePost**: Executes all hooks; errors are logged but never propagate to
  the caller.

Six hook points cover all lifecycle operations:

| Hook Point | Operations |
|---|---|
| `HookCreateArtifact` | `CreateArtifact` |
| `HookUpdateArtifact` | `UpdateArtifact` (includes MCP `move_item`) |
| `HookArchiveItem` | `ArchiveItem` |
| `HookShipShipment` | `ShipShipment` |
| `HookMoveShipmentStatus` | `MoveShipmentStatus` |
| `HookAdoptItem` | `AdoptItem` |

**Built-in pre-hooks:**

- `ValidateStatusTransition` (priority 20): Config-driven transition map
  validates that status changes follow allowed paths. Fires on
  `HookUpdateArtifact`.

**Built-in post-hooks:**

- `EmitHookEvent` (priority 50): Writes structured JSONL event records via
  the `HookEventAppender` interface. Skips when `TopLevel == false`.
- `LogIndexStale` (priority 90): Marks the SQLite index as stale after
  mutations.

### Layer 2: Asynchronous External Webhooks

`WebhookNotifier` in `internal/hooks/webhook.go` dispatches HTTP POST
notifications to configured endpoints:

- Registered at priority 80 on all hook points
- Skips when `TopLevel == false` (same as EmitHookEvent)
- Async goroutine per endpoint with `context.Background()` timeout
- `rate.Limiter` for backpressure (default 10/sec, burst 20)
- `Shutdown()` drains pending dispatches via `sync.WaitGroup`

### TopLevel Guard

Nested operations (e.g., `ShipShipment` → `ArchiveItem`) pass
`TopLevel: false` in the `HookContext`. Post-hooks that emit external events
(EmitHookEvent, WebhookNotifier) check this flag and skip, preventing duplicate
events. Pre-hooks always fire regardless of TopLevel.

### Configuration

Hooks configuration lives in `.backlogit/hooks.yaml`:

```yaml
lifecycle:
  transitions:
    queued: [active, blocked]
    active: [done, blocked, review, shipped, abandoned]
    blocked: [active]
    review: [done, accepted, rejected]
    done: [archived]

notifications:
  enabled: true
  endpoints:
    - url: https://example.com/webhook
      events: [artifact.updated, shipment.shipped]
      headers:
        Authorization: $WEBHOOK_AUTH_TOKEN
      timeout: 10s
      rate_limit:
        requests_per_second: 5
        burst: 10
```

Header values must start with `$` for environment variable expansion.
`os.ExpandEnv` resolves values at dispatch time.

## Monitoring Plan

### Healthy signals

* All lifecycle operations (`CreateArtifact`, `UpdateArtifact`, `ArchiveItem`,
  `ShipShipment`) emit structured JSONL events in `.backlogit/logs/`.
* `ValidateStatusTransition` rejects invalid transitions with descriptive
  `ErrInvalidStatusTransition` errors surfaced through MCP responses.
* Webhook endpoints receive POST payloads within the configured timeout.
* `slog` output shows `webhook dispatch` info-level entries for successful
  deliveries.

### Failure signals

* `slog` warnings for `webhook dispatch error`, `webhook rate limit wait
  cancelled`, or `webhook marshal error` indicate endpoint or configuration
  issues.
* `pre-update hook` errors in MCP responses indicate a status transition
  validation failure — check the transition map in hooks config.
* Missing JSONL events after mutations: check that `EmitHookEvent` is
  registered and `TopLevel` is set correctly.

### Observability queries

```sql
-- Count indexed hook-related log events by type
SELECT event_type, COUNT(*) AS count
FROM item_log_entries
WHERE event_type LIKE 'hook%'
GROUP BY event_type
ORDER BY count DESC, event_type ASC;

-- Inspect recent hook-related log entries
SELECT timestamp, item_id, actor, event_type, content
FROM item_log_entries
WHERE event_type LIKE 'hook%'
ORDER BY timestamp DESC
LIMIT 20;

-- Find failed status transitions (via MCP error responses)
-- Look for ErrInvalidStatusTransition in agent logs

-- Pending hook queue signals are not stored in SQLite.
-- Use backlogit_poll_hook_events to inspect .backlogit/hooks_queue.jsonl.
```

## Rollback Plan

The hooks system is fully additive with graceful degradation:

**Quick disable:** Set `notifications.enabled: false` in
`.backlogit/hooks.yaml` to disable all webhook dispatch without code changes.
The internal hook system remains active.

**Full rollback trigger:** If `ValidateStatusTransition` produces false
negatives (rejects valid transitions) blocking normal workflow operations.

**Full rollback steps:**

1. `git revert -m 1 1f927c9` to revert the PR merge commit
2. Run `backlogit sync` to rebuild `backlogit.db`
3. All lifecycle operations return to their pre-hooks behavior — no validation,
   no JSONL event emission, no webhook dispatch

**Risk level:** Low. The hooks system wraps existing operations without
modifying their core logic. All pre-existing tests pass with the hooks active,
confirming backward compatibility.

## Dependency Added

| Package | Version | Purpose |
|---|---|---|
| `golang.org/x/time` | v0.11.0 | `rate.Limiter` for webhook backpressure |

Requires Go 1.23+. Compatible with CI matrix (Go 1.23 and 1.24).

## Follow-up Tasks

| Item | Description | Priority |
|---|---|---|
| Webhook retry with exponential backoff | Current dispatch is fire-and-forget; add configurable retry for transient failures | medium |
| Webhook HMAC signing | Add `X-Backlogit-Signature` header with HMAC-SHA256 of payload for endpoint verification | medium |
| CLI hooks management | `backlogit hooks list`, `backlogit hooks test <endpoint>` commands for operator visibility | low |
| Async hook event bus | Replace direct FirePost loop with a channel-based event bus for higher throughput | low |

## Validation Window

48 hours post-merge. Validation complete when:

* Normal backlogit workflows (create, update, move, archive, ship) function
  without unexpected hook errors.
* Hook events appear in `.backlogit/hooks_queue.jsonl` after lifecycle
  operations, and per-item lifecycle history is recorded under
  `.backlogit/logs/`.
* Webhook dispatch (when configured) delivers payloads to endpoints.
* `ValidateStatusTransition` correctly enforces the configured transition map
  without false rejections.
