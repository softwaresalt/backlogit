---
chunk_strategy: h1-h2-h3
description: 'Retroactive exec-plan for PR #98: adds granular tool-call and session-fact JSONL tables with --by tool and --by context reporting dimensions'
doc_type: plan
docline:
    branch: feat/054-events-fact-harvest
    commit: 68fa483f081e5536b8a66b4ffe999922b6a6e977
    date: 2026-05-09T00:00:00Z
    deliberation_id: 045-DL
    feature_id: 055-F
    origin: .backlogit/queue/045-DL.md
    pr: '#98'
    status: approved
ingested_at: "2026-06-26T02:33:21Z"
schema_version: "1.0"
source: docs/exec-plans/2026-05-09-events-fact-harvest-plan.md
title: events.jsonl Fact Table Harvest — Implementation Plan
---

## Overview

This plan captures the implementation of granular fact-table telemetry for the backlogit
telemetry pipeline. The work adds a second harvest layer on top of the existing
`backlogit telemetry harvest` command, parsing `.copilot/session-state/*/events.jsonl`
files to produce two new JSONL fact tables and two new reporting dimensions.

This is a retroactive plan: all implementation tasks are complete and quality-gated.
The plan is created to satisfy the Stage pipeline gate and provide a durable audit
trail in `docs/exec-plans/`.

## Constitution Check

| Principle | Status | Notes |
|---|---|---|
| I. Type-Safe Go | Pass | Structs with json/yaml tags; `omitempty` for additive schema evolution |
| II. MCP Protocol Fidelity | Pass | No MCP schema changes; existing tools unchanged |
| III. Test-First Development | Pass | 7 new tests in events_harvest_test.go written alongside implementation |
| IV. Workspace Containment | Pass | Output paths stay within `.backlogit/telemetry/` |
| V. Structured Observability | Pass | slog used throughout; fact counts emitted in harvest output |
| VI. Single-Binary Simplicity | Pass | No new dependencies added |
| VII. CQRS Data Architecture | Pass | Fact tables are append-only JSONL; no Markdown source-of-truth changes |
| VIII. Git-Friendly Persistence | Pass | JSONL rows are one-JSON-object-per-line, deterministic |
| IX. Agent Context Efficiency | Pass | Wide flat rows — no joins needed for reporting queries |

## Deliverables

### New Files

| File | Purpose |
|---|---|
| `internal/telemetry/events_harvest.go` | `ParseEventFacts` (parser) + `HarvestEventsFacts` (orchestrator) |
| `internal/telemetry/events_harvest_test.go` | 7 unit tests covering dual-schema events, MCP name resolution, checkpoint skipping, force mode |

### Modified Files

| File | Change |
|---|---|
| `internal/telemetry/records.go` | Added `ToolCallFact`, `SessionFact`, `ModelUsageMetrics` structs |
| `internal/telemetry/checkpoint.go` | Added `ProcessedEventSessions map[string]bool` field + nil-guard on read |
| `internal/telemetry/harvest.go` | Wired `HarvestEventsFacts`; extended `HarvestResult` with fact counts |
| `internal/telemetry/reporter.go` | Added `GenerateFactsReport` supporting `--by tool` and `--by context` in text/csv/markdown formats |
| `internal/cli/telemetry.go` | Updated `--by` flag enum; harvest output now shows tool-call and session-fact counts |
| `docs/cli-reference/backlogit_telemetry_report.md` | Regenerated to document new `--by` values |

## Key Design Decisions

### Dual-Schema events.jsonl Parsing

The events.jsonl format has two coexisting schemas:

- Old-format (compaction events): uses `event_type` field at top level.
- New-format (all other events, including tool calls): uses `type` + nested `data` object.

The parser uses `type` exclusively and ignores `event_type`. This avoids confusion between
the two schemas and correctly handles the full event stream.

### MCP Tool Name Resolution

For MCP tool calls, the `mcpToolName` field (e.g., `backlogit_move_item`) is used when
present. When absent, the server prefix is stripped from `toolName` to produce a short name.
Built-in tool calls retain their full `toolName`. This gives consistent short-form names
across both built-in and MCP tools in the fact table.

### Incremental Checkpoint

Sessions are identified by their directory name under `.copilot/session-state/`. Sessions
with a `session.shutdown` event are added to `HarvestCheckpoint.ProcessedEventSessions`
after harvest. Subsequent runs skip those sessions. Force mode (`--force`) clears the map
and re-processes all sessions.

### Token Data Granularity

Token data (input/output/cache/reasoning tokens, context window breakdown) is only available
in the `session.shutdown` event — not per-turn. `session-facts.jsonl` rows are therefore
keyed per-session, not per-turn. This is a deliberate design choice aligned with the data
that the Copilot CLI actually emits.

### Schema Evolution Strategy

All new struct fields use `omitempty`. Existing `tool-calls.jsonl` and `session-facts.jsonl`
rows written by earlier versions remain valid because the reader does not require new fields.
This provides a forward-compatible additive schema.

## Output Fact Tables

### `.backlogit/telemetry/tool-calls.jsonl`

One row per matched tool call (built-in + MCP):

```json
{
  "tool": "backlogit_move_item",
  "server": "backlogit",
  "session_id": "abc123",
  "branch": "feat/054-events-fact-harvest",
  "repo": "softwaresalt/backlogit",
  "model": "claude-sonnet-4",
  "duration_ms": 142,
  "success": true,
  "timestamp": "2026-05-08T14:23:01Z"
}
```

### `.backlogit/telemetry/session-facts.jsonl`

One row per session with a `session.shutdown` event:

```json
{
  "session_id": "abc123",
  "branch": "feat/054-events-fact-harvest",
  "model_usage": {
    "claude-sonnet-4": { "input_tokens": 42000, "output_tokens": 8000, "cache_read": 100000 }
  },
  "context_system_tokens": 8000,
  "context_conversation_tokens": 32000,
  "context_tool_def_tokens": 12000,
  "total_api_duration_ms": 142000,
  "timestamp": "2026-05-08T14:55:00Z"
}
```

## Reporting Dimensions

### `--by tool`

Aggregates `tool-calls.jsonl` by tool name:

| Tool | Calls | Success% | Avg Duration (ms) | Total Duration (ms) | Sessions |
|---|---|---|---|---|---|
| backlogit_move_item | 1240 | 99.8% | 87 | 107,880 | 60 |

### `--by context`

Per-session context window breakdown from `session-facts.jsonl`:

| Session | System% | Conversation% | Tool Defs% | Cache% |
|---|---|---|---|---|
| abc123 | 15.3% | 61.2% | 23.1% | 0.4% |

## Quality Gate Evidence

| Gate | Result | Details |
|---|---|---|
| `go test ./...` | Pass | 7 new tests; all 212 tests pass |
| `go vet ./...` | Pass | Zero findings |
| `golangci-lint run` | Pass | Zero findings |
| `gofmt -l .` | Pass | No unformatted files |
| Live harvest | Verified | 38,910 tool call facts, 44 session facts across 60 sessions |
| CI (PR #98) | Green | All checks passing |

## Risk Assessment

| Risk | Blast Radius | Mitigation |
|---|---|---|
| events.jsonl schema drift | Low | Parser uses `type` field; old `event_type` rows silently skipped |
| Large session directories | Low | Incremental checkpoint skips already-processed sessions |
| Reporter nil-pointer on empty fact tables | Low | GenerateFactsReport guards on empty slice before aggregation |
| Checkpoint map nil on first run | Resolved | Nil-guard added to checkpoint.go reader |

Blast radius is **low**: additive-only changes, no existing struct fields removed, no
existing CLI flags changed, no Markdown artifacts modified. `plan-harden` is not required.

## Tasks Decomposition

These tasks represent the logical work units actually implemented:

1. Data modelling — `ToolCallFact`, `SessionFact`, `ModelUsageMetrics` structs in records.go
2. Checkpoint extension — `ProcessedEventSessions` field + nil-guard in checkpoint.go
3. events.jsonl parser — `ParseEventFacts` in events_harvest.go (dual-schema, MCP name resolution)
4. Harvest orchestrator — `HarvestEventsFacts` in events_harvest.go + harvest.go wiring
5. Reporting — `GenerateFactsReport` in reporter.go (--by tool, --by context, text/csv/markdown)
6. CLI integration — `--by` flag update + harvest output in telemetry.go
7. Tests — 7 unit tests in events_harvest_test.go
