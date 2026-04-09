---
title: "021-F Token Telemetry Harvest: Post-Merge Closure"
description: "Operational closure record for shipment 009-S, PR #15, merge commit 2c5f4df"
ms.date: 2026-04-09
ms.topic: reference
---

## Closure Summary

| Field | Value |
|---|---|
| Feature | 021-F: Token Telemetry Harvest |
| Shipment | 009-S |
| PR | [#15](https://github.com/softwaresalt/backlogit/pull/15) |
| Merge commit | `2c5f4dfc923c8cf8d8b03d78b5408ee7702e36e1` |
| Fix commits | `1d88a91` (missing files), `88f10aa` (round-1 review), `de87ac2` (round-2 review) |
| Mode | post-merge |
| Readiness | **READY** |
| Owner | dewilliams |
| Validation window | 48 hours post-merge |

## Change Summary

Eight tasks shipped across four new packages and three extended packages.
All implemented test-first against failing harnesses verified via full CI
on both Go 1.23 and 1.24.

| Task | Title | Package(s) |
|---|---|---|
| 021.001-T | CopilotCLIParser: streaming log line parser | `internal/telemetry` |
| 021.002-T | SessionMeta loader: session-state/ and session-store.db | `internal/telemetry` |
| 021.003-T | Correlate: join events into per-session SessionSummary | `internal/telemetry` |
| 021.004-T | AttributeTool: prefix-based MCP server attribution | `internal/telemetry` |
| 021.005-T | HarvestTelemetry: top-level orchestrator | `internal/telemetry` |
| 021.006-T | EnsureTelemetrySchema + RehydrateTelemetry: SQLite tables | `internal/db` |
| 021.007-T | CLI: `backlogit telemetry harvest` | `internal/cli` |
| 021.008-T | MCP: `backlogit_telemetry_harvest` tool | `internal/mcp` |

**Net change:** 37 files, +2179 / −30 lines.

## CI Status

| Check | Go 1.23 | Go 1.24 |
|---|---|---|
| `go test ./...` | ✅ pass | ✅ pass |
| `golangci-lint run` | ✅ pass | ✅ pass |
| `gofmt -l .` | ✅ pass | ✅ pass |
| `go vet ./...` | ✅ pass | ✅ pass |

Both CI matrix runs completed green on the final commit `de87ac2` before merge.

## Copilot Review Resolution

| Round | Comments | Resolved |
|---|---|---|
| Round 1 (`88f10aa`) | 11 | 10 fixed, 1 declined (workspace-scoped metric per plan D5) |
| Round 2 (`de87ac2`) | 5 | 5 fixed |
| **Total** | **16** | **16** |

Notable second-pass fixes with architectural significance:

**Events path correction:** `loadCompletedTasks` was reading `.backlogit/events.jsonl`
which does not exist. Events are written per-item to `.backlogit/logs/<item_id>.jsonl`.
Fixed to scan the `logs/` directory with `doneTasksFromLog` helper and ID deduplication.

**Atomic JSONL write:** `writeTelemetryJSONL` now uses temp-file-then-rename
(`jsonlPath + ".tmp"` → `os.Rename`) to prevent a corrupt `telemetry-sessions.jsonl`
on crash or mid-write process kill. Matches the pattern in `internal/core/artifacts.go`.

**O(S×E) → O(T) server call counting:** `serverCallsPerSession` is now derived from
the already-built `toolStats` map in a single O(T) pass rather than a nested O(S×E)
loop over all events per session.

**Field rename:** `TokensByServer` → `ToolCallsByServer` / `tool_calls_by_server`
throughout `SessionSummaryRecord` and `rawTelemetryRecord`: the field stores call
counts, not token sums.

## New Telemetry Architecture

```text
    ↓ CopilotCLIParser (streaming, 1MB buffer)
[]TelemetryEvent
    ↓ Correlate() + loadCompletedTasks (.backlogit/logs/*.jsonl)
[]SessionSummary
    ↓ AttributeTool() (prefix registry)
    ↓ toolStats + serverCallsPerSession
    ↓ writeTelemetryJSONL (atomic temp-rename)
.backlogit/telemetry-sessions.jsonl
    ↓ RehydrateTelemetry (1MB scanner, transactional)
SQLite: telemetry_sessions + telemetry_tool_usage
    ↓ backlogit_query_sql
Agent queries
```

## Monitoring Plan

### Healthy signals

* `backlogit telemetry harvest` exits 0 with `"sessions_harvested": N > 0` when
  `.copilot/logs/` contains parseable log files.
* `SELECT COUNT(*) FROM telemetry_sessions` returns rows after harvest.
* `SELECT COUNT(*) FROM telemetry_tool_usage` returns rows per session.
* `telemetry-sessions.jsonl` file exists and is non-empty after harvest.
* `tokens_per_task` is non-null in sessions where items were completed to `done`.

### Failure signals

* `backlogit telemetry harvest` exits non-zero or returns `ErrTelemetrySourceMissing`:
  `.copilot/logs/` does not exist or is empty.
* `telemetry_sessions` has zero rows after a harvest that reported `sessions_harvested > 0`:
  RehydrateTelemetry transaction failed silently (check `slog.Warn` output).
* `tokens_per_task` is always null: `loadCompletedTasks` found no `done` events;
  verify `.backlogit/logs/*.jsonl` contains `status_changed` events.
* `.backlogit/telemetry-sessions.jsonl.tmp` exists, indicating a prior harvest crashed mid-write;
  safe to delete, the `tmp` file is stale.
* Scanner errors in `RehydrateTelemetry` logs: a JSONL line exceeds 1MB; investigate
  anomalous `completed_tasks` array sizes.

## Rollback Plan

The telemetry feature adds new files and tables; it does not modify any existing
artifacts, queue state, or core CQRS paths.

**Rollback trigger:** If `backlogit telemetry harvest` causes data corruption in
`.backlogit/` outside the telemetry namespace, or if `RehydrateTelemetry` corrupts
`index.db`, revert by running `backlogit sync` to rebuild the index from Markdown.

**Rollback steps:**

1. `git revert 2c5f4dfc923c8cf8d8b03d78b5408ee7702e36e1` to remove telemetry code
2. Delete `.backlogit/telemetry-sessions.jsonl` if present
3. Run `backlogit sync` to rebuild `index.db` (telemetry tables will not exist)
4. The `telemetry_sessions` and `telemetry_tool_usage` tables are created lazily
   on first harvest; their absence does not affect any other backlogit operation

**Risk level:** Low. Telemetry is purely additive. No existing command, MCP tool,
or data file is modified by the harvest pipeline.

## Follow-up Tasks

These items are stashed or identified as natural next steps:

| Item | Description | Priority |
|---|---|---|
| Incremental harvest | Currently full re-harvest on every run (Plan Review F2 deferral) | medium |
| Session-scoped task completion | `completedTasks` is workspace-scoped, not session-scoped (intentional per plan D5; revisit if per-session attribution is needed) | low |
| Telemetry query examples | Add sample `backlogit_query_sql` patterns for token spend analysis to docs | low |

## Validation Window

48 hours post-merge. Validation is complete when:

* At least one successful `backlogit telemetry harvest` run confirms the parse,
  correlate, and rehydration pipeline on real `.copilot/logs/` data.
* `SELECT * FROM telemetry_sessions LIMIT 5` returns plausible session rows.
* No `telemetry-sessions.jsonl.tmp` residue observed after harvest.
