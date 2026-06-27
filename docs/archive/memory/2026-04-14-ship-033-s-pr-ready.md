# Ship 033-S: Hooks System — Session Memory

**Timestamp:** 2026-04-14T02:50:00Z
**Shipment:** 033-S
**Branch:** feat/033-hooks-system
**PR:** https://github.com/softwaresalt/backlogit/pull/36
**Status:** PR ready, awaiting user merge approval

## Completed Items

All 10 tasks and the parent feature are done:

| ID | Title | Status |
|---|---|---|
| 033-F | Hooks System: Internal Lifecycle Hooks and External Webhook Dispatch | done |
| 033.001-T | Unit 1: Hook Types and Runner | done |
| 033.002-T | Unit 2: Built-in Pre-Hooks | done |
| 033.003-T | Unit 3: Built-in Post-Hooks | done |
| 033.004-T | Unit 4: Config Expansion and LoadHooks | done |
| 033.005-T | Unit 5: Wire HookRunner into Workspace | done |
| 033.006-T | Unit 6: Instrument CreateArtifact/UpdateArtifact | done |
| 033.007-T | Unit 7a: Instrument ArchiveItem/MoveShipmentStatus | done |
| 033.008-T | Unit 7b: Instrument ShipShipment/AdoptItem | done |
| 033.009-T | Unit 8: WebhookNotifier | done |
| 033.010-T | Unit 9: Wire WebhookNotifier as Post-Hook | done |

## Blocked Returns

None — all items completed successfully.

## Key Decisions

1. **Shipment transitions in DefaultTransitions**: Added `shipped` and `abandoned` to `active→` transitions since shipments go through `UpdateArtifact` and the status transition pre-hook validates on `HookUpdateArtifact`.

2. **Go module version**: Kept at `go 1.24.0` by downgrading `x/time` from v0.15.0 to v0.11.0 (which requires only Go 1.23). CI workflows test against Go 1.23 and 1.24.

3. **Typed-nil prevention**: The `webhookNotifier` field on Workspace uses an interface type. Storing a typed-nil `*WebhookNotifier` pointer inside the interface would bypass the `!= nil` guard in `Close()`. Fixed by only assigning when non-nil.

4. **Rate limiter context**: Changed `rateLimiter.Wait(ctx)` to use `context.Background()` to decouple from parent hook context, ensuring all matching endpoints get dispatched even if parent context is cancelled.

5. **HTTP connection cleanup**: Added `CloseIdleConnections()` in `Shutdown()` to release connection pool resources.

## CI Gate Results

- `go test ./...`: All packages pass (16/16)
- `go vet ./...`: Zero findings
- `gofmt -l .`: Clean (after formatting all files)

## Test Adaptations

13 existing tests were updated to use valid transition paths (e.g., `queued→active→done` instead of `queued→done`) to comply with the new status transition validation hook.

## Files Changed (42 files, +2216/-13)

### New files (10):
- `internal/hooks/hooks.go`, `hooks_test.go`
- `internal/hooks/builtin_pre.go`, `builtin_pre_test.go`
- `internal/hooks/builtin_post.go`, `builtin_post_test.go`
- `internal/hooks/webhook.go`, `webhook_test.go`
- `tests/integration/webhook_test.go`
- `.backlogit/` artifacts (12 files)

### Modified files:
- `internal/core/workspace.go`: HookRunner wiring, webhook notifier, adapter
- `internal/core/artifacts.go`: Pre/post hooks on Create/Update
- `internal/core/archive.go`: WithTopLevel ArchiveOpt, pre/post hooks
- `internal/core/shipment.go`: moveShipmentStatusWithTopLevel, pre/post hooks
- `internal/core/shipment_lifecycle.go`: Pre/post hooks on ShipShipment/AdoptItem
- `internal/config/schema.go`, `defaults.go`, `loader.go`: Config types
- `internal/errors/errors.go`: Error sentinels
- `go.mod`, `go.sum`: x/time dependency

## Next Steps

- Await user merge approval on PR #36
- After merge: post-merge closure, documentation updates, shipment close
