---
title: Harness scaffold complete for 021-F token telemetry
description: Session checkpoint after completing harness-architect for 021-F on branch feat/021-token-telemetry
ms.date: 2026-04-09
---

## Summary

Completed the harness-architect phase for feature 021-F (Token and context
window efficiency telemetry) on branch `feat/021-token-telemetry`. All
production stubs compile, all harness tests are in the red phase, and
`go build ./...` passes cleanly.

## Shipment state

Shipment 009-S is **active**. Feature 021-F has 8 tasks (021.001-T through
021.008-T), all `active`, all with harness commands recorded.

## Production stubs created

All files live under `internal/telemetry/` or adjacent packages. All non-trivial
stubs use `panic("not implemented: ...")` so `go build` passes but tests fail.

| File | Purpose | Status |
|---|---|---|
| `internal/telemetry/types.go` | Core types: TelemetryEvent, ModelCall, ToolCall, EventKind, interfaces | Complete (no panics) |
| `internal/telemetry/parser.go` | CopilotCLIParser + LogParser interface | Stub (panic) |
| `internal/telemetry/session_events.go` | ParseSessionEvents + LoadSessionEvents | Stub (panic) |
| `internal/telemetry/session_store.go` | ReadSessionStore for session-store.db | Stub (panic) |
| `internal/telemetry/attribution.go` | AttributeTool with defaultPrefixes map | Stub (panic) |
| `internal/telemetry/correlator.go` | Correlate: join events+meta into summaries | Stub (panic) |
| `internal/telemetry/records.go` | SessionSummaryRecord + ToolUsageRecord typed structs | Complete (no panics) |
| `internal/telemetry/harvest.go` | HarvestTelemetry orchestrator + HarvestResult | Stub (panic) |
| `internal/db/telemetry_schema.go` | EnsureTelemetrySchema + RehydrateTelemetry | Stub (panic) |
| `internal/cli/telemetry.go` | NewTelemetryCmd + 3 subcommands (harvest, list, top) | Structural complete; RunE stubs panic |
| `internal/mcp/tools.go` | backlogit_telemetry_harvest tool registered + handleTelemetryHarvest | Stub (panic) |
| `internal/errors/errors.go` | ErrTelemetrySourceMissing + ErrTelemetryParseFailed added | Complete |

## Harness test files created

| File | Tests | Phase |
|---|---|---|
| `internal/telemetry/parser_test.go` | CopilotCLIParser: sample log, empty, malformed, field validation | Red (panic) |
| `internal/telemetry/session_events_test.go` | ParseSessionEvents: compactions, empty, no compactions, malformed | Red (panic) |
| `internal/telemetry/session_store_test.go` | ReadSessionStore: missing DB, reads session metadata | Red (panic) |
| `internal/telemetry/attribution_test.go` | AttributeTool: known prefixes, unknown, longest-prefix-wins | Red (panic) |
| `internal/telemetry/correlator_test.go` | Correlate: groups by session, no task completions | Red (panic) |
| `internal/telemetry/harvest_test.go` | HarvestTelemetry: happy path, idempotent, missing .copilot | Red (panic) |
| `internal/telemetry/helpers_test.go` | Shared openTestDB helper | Support file |
| `internal/db/telemetry_schema_test.go` | Schema: idempotent create, rehydrate, composite PK, SQL gate | Red (panic) |
| `internal/cli/telemetry_test.go` | Command registration: telemetry, harvest, list, top subcommands | Green (structural) |
| `tests/contract/telemetry_tool_test.go` | Tool registered, missing path error, sample logs, empty logs | Red (panic) |

## Build and test state

```
go build ./...             → EXIT 0 ✓
go test ./internal/telemetry/...  → FAIL (red - panics) ✓
go test ./internal/db/...         → FAIL (red - panics) ✓
go test ./internal/cli/...        → PASS (structural tests pass) ✓
go test ./tests/contract/...      → FAIL (red - panics) ✓
golangci-lint run (new files)     → EXIT 0 ✓
```

## Harness commands by task

| Task | Harness Command |
|---|---|
| 021.001-T | `go test ./internal/telemetry/ -run TestCopilotCLIParser` |
| 021.002-T | `go test ./internal/telemetry/ -run TestParseSessionEvents\|TestReadSessionStore` |
| 021.003-T | `go test ./internal/telemetry/ -run TestAttributeTool` |
| 021.004-T | `go test ./internal/telemetry/ -run TestCorrelate` |
| 021.005-T | `go test ./internal/db/ -run TestEnsureTelemetrySchema\|TestRehydrateTelemetry\|TestTelemetryTablesQueryableViaGate` |
| 021.006-T | `go test ./internal/telemetry/ -run TestHarvestTelemetry` |
| 021.007-T | `go test ./internal/cli/ -run TestTelemetry && go test ./internal/telemetry/ -run TestHarvestTelemetry` |
| 021.008-T | `go test ./tests/contract/ -run TestTelemetryHarvestTool` |

## Design decisions locked in harness

Per plan review revisions, the harness encodes these decisions:

* **Streaming parser**: `Parse(r io.Reader, emit func(TelemetryEvent) error) error` (not accumulating slice)
* **Dedicated JSONL**: `telemetry-sessions.jsonl` (NOT the existing `telemetry.jsonl`)
* **Single write path**: JSONL → rehydrate → SQLite (no direct upserts)
* **Composite PK**: `(session_id, server_name, tool_name)` for `telemetry_tool_usage`
* **Graceful fallback**: missing `session-store.db` returns empty map, not error
* **Task completions**: detected from `.backlogit/events.jsonl`, NOT internal Copilot logs
* **v1 re-harvest**: full re-harvest every run (no checkpoint)

## Next step

Invoke `build-feature` skill starting with 021.001-T (no dependencies). Then proceed
in dependency order:
1. 021.001-T (parser) and 021.003-T (attribution) — can run in parallel
2. 021.002-T (session metadata) — depends on 021.001-T
3. 021.004-T (correlator) — depends on 021.001-T, 021.002-T, 021.003-T
4. 021.005-T (DB schema) — depends on 021.004-T
5. 021.006-T (harvest pipeline) — depends on 021.004-T, 021.005-T
6. 021.007-T (CLI) and 021.008-T (MCP) — both depend on 021.006-T
