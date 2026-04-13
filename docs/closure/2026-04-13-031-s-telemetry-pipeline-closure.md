---
title: "031-S Telemetry Pipeline Enhancements: Post-Merge Closure"
description: "Operational closure record for shipment 031-S, PR #32, merge commit 8e736c7"
ms.date: 2026-04-13
ms.topic: reference
---

## Closure Summary

| Field | Value |
|---|---|
| Feature | 031-F: Telemetry Pipeline Enhancements |
| Shipment | 031-S |
| PR | [#32](https://github.com/softwaresalt/backlogit/pull/32) |
| Merge commit | `8e736c7` |
| Fix commits | `f0b5def` (Copilot review round 1, 11 comments) |
| Mode | post-merge |
| Readiness | **READY** |
| Owner | dewilliams |
| Validation window | 48 hours post-merge |

## Change Summary

Four tasks shipped across five modified packages. All implemented test-first
against failing harnesses verified via full CI on both Go 1.23 and 1.24.

| Task | Title | Package(s) |
|---|---|---|
| 031.001-T | Incremental Harvest with Byte-Offset Checkpoints | `internal/telemetry`, `internal/mcp` |
| 031.002-T | Date-Filtered Harvest (`--since` flag) | `internal/telemetry`, `internal/cli` |
| 031.003-T | Context Window Consumption Tracking | `internal/telemetry`, `internal/db` |
| 031.004-T | CLI Telemetry Report Subcommand | `internal/cli`, `internal/mcp` |

**Net change:** 38 files, +2368 / −57 lines.

## CI Status

| Check | Go 1.23 | Go 1.24 |
|---|---|---|
| `go test ./...` | ✅ pass | ✅ pass |
| `golangci-lint run` | ✅ pass | ✅ pass |
| `gofmt -l .` | ✅ pass | ✅ pass |
| `go vet ./...` | ✅ pass | ✅ pass |

Both CI matrix runs completed green on the final commit `f0b5def` before merge.

## Copilot Review Resolution

| Round | Comments | Resolved |
|---|---|---|
| Round 1 (`f0b5def`) | 11 | 11 fixed |
| **Total** | **11** | **11** |

Notable fixes with architectural significance:

**Windows atomic rename:** `SaveCheckpoint` now calls `os.Remove(path)` before
`os.Rename` on Windows, where rename-over-existing fails unlike POSIX systems.
Pattern consistent with the atomic write approach in `internal/core/artifacts.go`.

**GroupBy validation:** `GenerateReport` rejects unsupported `groupBy` values
(`"model"`, `"tool"`) early with a descriptive error rather than returning empty
output. Only `"session"` and `"server"` are valid.

**Limit after aggregation:** `Limit` in `GenerateReport` now applies to the
aggregated output slice (after summing across all sessions), not to the input
session list. This ensures top-N server rows are computed over all data before
truncation.

**MCP schema enriched:** `backlogit_telemetry_harvest` tool schema now includes
`since` (string, RFC3339) and `force` (bool) optional parameters, matching the
CLI flags.

**HarvestOptions placement:** `HarvestOptions{Force bool, Since *time.Time}`
lives in `checkpoint.go` (colocated with incremental semantics); callers use
`&t` for pointer assignment after parsing.

## New Telemetry Features

### Incremental harvest

```bash
# Only harvest sessions newer than a given timestamp
backlogit telemetry harvest --since 2026-04-01T00:00:00Z

# Force full re-harvest ignoring previous checkpoint
backlogit telemetry harvest --force
```

MCP equivalent:

```json
{"name": "backlogit_telemetry_harvest", "arguments": {"since": "2026-04-01T00:00:00Z"}}
```

### Context-window utilization

Four new columns in `telemetry_sessions`:

| Column | Type | Description |
|---|---|---|
| `peak_utilization` | REAL | Highest context fill fraction observed (0.0–1.0) |
| `remaining_capacity` | REAL | Context remaining at session end (fraction) |
| `depletion_rate` | REAL | Average fill increase per tool call |
| `max_context_tokens` | INTEGER | Model's total context window size |

The schema migration uses `ALTER TABLE ADD COLUMN` with graceful handling of
`duplicate column name` errors for existing workspaces.

### Telemetry report commands

```bash
# Per-session summary table (default)
backlogit telemetry list

# Per-server call volume, top 10
backlogit telemetry top --limit 10

# Full report grouped by session, JSON output
backlogit telemetry report --by session --format json --limit 20

# Report grouped by server, table output
backlogit telemetry report --by server --format table
```

## Architectural Decisions

**`HarvestOptions` in `checkpoint.go`:** The incremental harvest logic is
tightly coupled to the `LastHarvestedAt` checkpoint tracking. Colocating
`HarvestOptions` there avoids a cross-package import from `harvest.go` to
`checkpoint.go` and keeps the bounded context cohesive.

**`Since` as `*time.Time`:** Using a pointer allows distinguishing "not
provided" from "zero time" without a sentinel constant. Callers parse a string
then pass `&t`; `nil` means no filter.

**`formatServerTable` applies `Limit` post-aggregation:** Server-grouped output
aggregates call counts across all sessions before sorting and slicing. Applying
`Limit` to input sessions (pre-aggregation) would give incorrect per-server
totals when a server appears in more than `Limit` sessions.

## Monitoring Plan

### Healthy signals

* `backlogit telemetry harvest --since <date>` exits 0 with
  `"sessions_harvested": N` when `.copilot/logs/` contains sessions newer than
  the since threshold.
* `backlogit telemetry list` renders a table with at least one row after harvest.
* `SELECT peak_utilization FROM telemetry_sessions WHERE peak_utilization IS NOT NULL LIMIT 1`
  returns a row on workspaces with Copilot CLI logs containing context events.
* `backlogit telemetry report --by server` shows MCP server attribution rows.

### Failure signals

* `backlogit telemetry harvest --since` returns `sessions_harvested: 0` when
  all existing sessions predate the threshold (expected, not an error).
* `peak_utilization` and `remaining_capacity` are NULL: the `.copilot/logs/`
  files contain no context-window events (older Copilot CLI versions).
* `backlogit telemetry report --by model` returns an error: `"model"` is not a
  supported groupBy value; use `"session"` or `"server"`.
* `ALTER TABLE` migration log shows duplicate column errors in `slog` output:
  benign — existing workspaces already have the columns; migration swallows them.

## Rollback Plan

The enhancements are purely additive: new CLI flags, new SQLite columns, and
new report commands. No existing artifact, queue state, or core CQRS path is
modified.

**Rollback trigger:** If `backlogit telemetry harvest --since` produces
incorrect session filtering, or if `GenerateReport` panics on unexpected
`groupBy` input in production.

**Rollback steps:**

1. `git revert -m 1 8e736c7` to revert the PR merge commit against its mainline parent and remove the telemetry enhancement code
2. Run `backlogit sync` to rebuild `backlogit.db` (context-window columns will
   not exist after revert; existing sessions are unaffected)
3. The `peak_utilization`, `remaining_capacity`, `depletion_rate`, and
   `max_context_tokens` columns are additive; their absence does not break any
   other backlogit operation

**Risk level:** Low. All changes are backwards-compatible additions.

## Follow-up Tasks

| Item | Description | Priority |
|---|---|---|
| Context-window event parsing | `ContextWindowParser` skeletonized; full parsing from Copilot log event types needs validation on real log samples | medium |
| `backlogit telemetry report` docs | Add sample `backlogit_query_sql` patterns for context-window analysis to docs | low |
| `--session` filter | `report --session <id>` flag accepted but not yet wired to a session-scoped SQL filter | low |

## Validation Window

48 hours post-merge. Validation complete when:

* At least one successful `backlogit telemetry harvest --since <date>` run on
  real `.copilot/logs/` data confirms incremental filtering works correctly.
* `backlogit telemetry list` renders a plausible session table.
* `backlogit telemetry top --limit 5` shows MCP server attribution rows.
* `SELECT peak_utilization FROM telemetry_sessions LIMIT 1` returns a row (if
  context-window events are present in logs) or NULL (acceptable for older logs).
