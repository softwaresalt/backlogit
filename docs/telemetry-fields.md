---
title: "Telemetry Field Reference"
description: "Field-level reference for harvested telemetry records, SessionSummaryRecord, ToolUsageRecord, and SQLite column mappings"
ms.topic: reference
---

## Telemetry Field Reference

This reference documents every field produced by `backlogit telemetry harvest`.
Fields appear in `.backlogit/telemetry-sessions.jsonl` and are mirrored to the
`telemetry_sessions` and `telemetry_tool_usage` SQLite tables in
`.backlogit/backlogit.db`.

### Session summary fields

Each `session_summary` JSONL record (Go type: `SessionSummaryRecord`) describes
one agent session.

| Field | Type | JSON key | SQLite column | Description |
|---|---|---|---|---|
| RecordType | string | `record_type` | — | Always `"session_summary"`. |
| HarvestedAt | time | `harvested_at` | `harvested_at` (TEXT, RFC3339) | Timestamp when this record was written. |
| SessionID | string | `session_id` | `session_id` (PK) | Unique session identifier from the Copilot CLI log. |
| Branch | string | `branch` | `branch` | Git branch active during the session. |
| Repository | string | `repository` | `repository` | Repository name from session metadata. |
| TotalTokens | int | `total_tokens` | `total_tokens` | Sum of prompt + completion tokens across all model calls. |
| PromptTokens | int | `prompt_tokens` | `prompt_tokens` | Total prompt (input) tokens consumed. |
| CompletionTokens | int | `completion_tokens` | `completion_tokens` | Total completion (output) tokens generated. |
| CachedTokens | int | `cached_tokens` | `cached_tokens` | Tokens served from the model's prompt cache. |
| ModelCalls | int | `model_calls` | `model_calls` | Number of model inference calls in this session. |
| ToolCalls | int | `tool_calls` | `tool_calls` | Number of tool invocations in this session. |
| TokensByModel | map[string]int | `tokens_by_model` | — (JSON only) | Total tokens per model name. Not stored in SQLite. |
| ToolCallsByServer | map[string]int | `tool_calls_by_server` | — (JSON only) | Tool call count per MCP server. Used for server-level ranking. Not stored in SQLite. |
| CompletedTasks | []string | `completed_tasks` | — (JSON only) | Backlogit task IDs completed in this session. |
| TokensPerTask | *float64 | `tokens_per_task` | `tokens_per_task` (REAL) | Average tokens per completed task. `null` when no tasks were completed. |
| CompactionCount | int | `compaction_count` | `compaction_count` | Number of context compaction events in this session. |
| PeakUtilization | *float64 | `peak_utilization` | `peak_utilization` (REAL) | Peak context window utilisation as a fraction (0.0–1.0). `null` when no model calls are recorded. |
| RemainingCapacity | *int | `remaining_capacity` | `remaining_capacity` (INTEGER) | Remaining context tokens at peak utilisation. `null` when unavailable. |
| DepletionRate | *float64 | `depletion_rate` | `depletion_rate` (REAL) | Estimated token consumption rate (tokens per tool call). `null` when unavailable. |
| MaxContextTokens | *int | `max_context_tokens` | `max_context_tokens` (INTEGER) | Maximum context window size for the primary model used. `null` when model is unknown. |

### Tool usage fields

Each `tool_usage` JSONL record (Go type: `ToolUsageRecord`) describes per-tool
call counts for one session. The composite key `(session_id, server_name, tool_name)`
is unique per harvest run.

| Field | Type | JSON key | SQLite column | Description |
|---|---|---|---|---|
| RecordType | string | `record_type` | — | Always `"tool_usage"`. |
| HarvestedAt | time | `harvested_at` | `harvested_at` (TEXT, RFC3339) | Timestamp when this record was written. |
| SessionID | string | `session_id` | `session_id` (FK) | Session this record belongs to. |
| ServerName | string | `server_name` | `server_name` | MCP server that owns the tool (e.g., `"backlogit"`, `"copilot"`). |
| ToolName | string | `tool_name` | `tool_name` | Name of the MCP tool called (e.g., `"backlogit_create_item"`). |
| CallCount | int | `call_count` | `call_count` | Number of times this tool was called in the session. |
| TotalDurMs | int | `total_duration_ms` | `total_dur_ms` | Cumulative call duration in milliseconds. |

### SQLite table schemas

#### `telemetry_sessions`

```sql
CREATE TABLE IF NOT EXISTS telemetry_sessions (
    session_id         TEXT    PRIMARY KEY,
    branch             TEXT    NOT NULL DEFAULT '',
    repository         TEXT    NOT NULL DEFAULT '',
    total_tokens       INTEGER NOT NULL DEFAULT 0,
    prompt_tokens      INTEGER NOT NULL DEFAULT 0,
    completion_tokens  INTEGER NOT NULL DEFAULT 0,
    cached_tokens      INTEGER NOT NULL DEFAULT 0,
    model_calls        INTEGER NOT NULL DEFAULT 0,
    tool_calls         INTEGER NOT NULL DEFAULT 0,
    tokens_per_task    REAL,
    compaction_count   INTEGER NOT NULL DEFAULT 0,
    harvested_at       TEXT    NOT NULL DEFAULT '',
    peak_utilization   REAL,
    remaining_capacity INTEGER,
    depletion_rate     REAL,
    max_context_tokens INTEGER
);
```

#### `telemetry_tool_usage`

```sql
CREATE TABLE IF NOT EXISTS telemetry_tool_usage (
    session_id   TEXT    NOT NULL,
    server_name  TEXT    NOT NULL,
    tool_name    TEXT    NOT NULL,
    call_count   INTEGER NOT NULL DEFAULT 0,
    total_dur_ms INTEGER NOT NULL DEFAULT 0,
    harvested_at TEXT    NOT NULL DEFAULT '',
    PRIMARY KEY (session_id, server_name, tool_name)
);
```

### Example SQL queries

List top sessions by total tokens:

```sql
SELECT session_id, total_tokens, model_calls, tool_calls
FROM telemetry_sessions
ORDER BY total_tokens DESC
LIMIT 10;
```

List top MCP servers by total call count:

```sql
SELECT server_name, SUM(call_count) AS total_calls
FROM telemetry_tool_usage
GROUP BY server_name
ORDER BY total_calls DESC;
```

Find sessions with high context window utilisation:

```sql
SELECT session_id, peak_utilization, remaining_capacity, max_context_tokens
FROM telemetry_sessions
WHERE peak_utilization IS NOT NULL
ORDER BY peak_utilization DESC
LIMIT 20;
```

### Notes

* `tokens_by_model` and `tool_calls_by_server` are stored in JSONL only. Query them
  via `backlogit telemetry report --group-by server` or by reading the JSONL directly.
* The SQLite tables are rebuilt from JSONL on every `telemetry harvest` run; they are
  an ephemeral query cache, not the source of truth.
* Use `backlogit_query_sql` (MCP) or `backlogit query` (CLI) to run ad-hoc queries
  against the SQLite tables.
