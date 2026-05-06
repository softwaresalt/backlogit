---
title: Telemetry fields reference
description: Reference for harvested telemetry JSONL fields, derived metrics, and SQLite column mappings
ms.date: 2026-05-05
ms.topic: reference
---

## Overview

backlogit harvests Copilot CLI session telemetry into
`.backlogit/telemetry-sessions.jsonl` and then rehydrates two SQLite tables in
`.backlogit/backlogit.db`:

* `telemetry_sessions`
* `telemetry_tool_usage`

The JSONL file is the telemetry source of truth. SQLite is the query cache.

## JSONL record types

The harvested JSONL stream contains two record types:

* `session_summary`
* `tool_usage`

## Session summary fields

The `session_summary` record is written from
`internal/telemetry/records.go:SessionSummaryRecord`.

| Field | Type | Description |
|---|---|---|
| `record_type` | string | Always `session_summary` |
| `harvested_at` | timestamp | UTC time when the harvest wrote the record |
| `session_id` | string | Copilot session identifier |
| `branch` | string | Git branch recorded for the session |
| `repository` | string | Repository identifier for the session |
| `total_tokens` | integer | Total prompt plus completion tokens for the session |
| `prompt_tokens` | integer | Prompt-token total for the session |
| `completion_tokens` | integer | Completion-token total for the session |
| `cached_tokens` | integer | Cached prompt tokens reported by the model |
| `model_calls` | integer | Number of model calls correlated to the session |
| `tool_calls` | integer | Number of tool calls correlated to the session |
| `tokens_by_model` | object | Token totals grouped by model identifier |
| `tool_calls_by_server` | object | Tool-call counts grouped by attributed server name |
| `completed_tasks` | array | Backlog item IDs correlated as completed during the session |
| `tokens_per_task` | number or null | Derived `total_tokens / len(completed_tasks)` when at least one task completed |
| `compaction_count` | integer | Number of compaction events observed for the session |
| `peak_utilization` | number or null | Highest prompt-token to max-context ratio observed in the session |
| `remaining_capacity` | integer or null | Tokens remaining at the point of peak utilization |
| `depletion_rate` | number or null | Average total tokens consumed per model call |
| `max_context_tokens` | integer or null | Model context-window size used for the utilization calculation |

## Tool usage fields

The `tool_usage` record is written from
`internal/telemetry/records.go:ToolUsageRecord`.

| Field | Type | Description |
|---|---|---|
| `record_type` | string | Always `tool_usage` |
| `harvested_at` | timestamp | UTC time when the harvest wrote the record |
| `session_id` | string | Copilot session identifier |
| `server_name` | string | Attributed server or tool family |
| `tool_name` | string | Concrete tool name from the Copilot log |
| `call_count` | integer | Number of calls for the `(session_id, server_name, tool_name)` tuple |
| `total_duration_ms` | integer | Total runtime for the tuple in milliseconds |

## Derived context metrics

backlogit computes session-level context metrics from correlated model calls and
compaction events.

| Metric | Meaning |
|---|---|
| `peak_utilization` | Highest prompt pressure relative to the model context limit |
| `remaining_capacity` | Tokens left when the session hit peak utilization |
| `depletion_rate` | Average total tokens consumed per model call |
| `max_context_tokens` | Context-window limit selected for the model at peak utilization |

## SQLite column mappings

The telemetry cache is rehydrated by `internal/db/telemetry_schema.go`.

### `telemetry_sessions`

| SQLite column | JSONL field |
|---|---|
| `session_id` | `session_id` |
| `branch` | `branch` |
| `repository` | `repository` |
| `total_tokens` | `total_tokens` |
| `prompt_tokens` | `prompt_tokens` |
| `completion_tokens` | `completion_tokens` |
| `cached_tokens` | `cached_tokens` |
| `model_calls` | `model_calls` |
| `tool_calls` | `tool_calls` |
| `tokens_per_task` | `tokens_per_task` |
| `compaction_count` | `compaction_count` |
| `harvested_at` | `harvested_at` |
| `peak_utilization` | `peak_utilization` |
| `remaining_capacity` | `remaining_capacity` |
| `depletion_rate` | `depletion_rate` |
| `max_context_tokens` | `max_context_tokens` |

### `telemetry_tool_usage`

| SQLite column | JSONL field |
|---|---|
| `session_id` | `session_id` |
| `server_name` | `server_name` |
| `tool_name` | `tool_name` |
| `call_count` | `call_count` |
| `total_dur_ms` | `total_duration_ms` |
| `harvested_at` | `harvested_at` |

## Query notes

Use `backlogit telemetry report` for formatted CLI output and `backlogit query`
for targeted SQL access to the cache tables.
