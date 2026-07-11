---
chunk_strategy: h1-h2-h3
description: Show telemetry JSONL and SQL table schemas
doc_type: reference
ingested_at: "2026-06-26T02:27:58Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_telemetry_schema.md
title: backlogit telemetry schema
---

## backlogit telemetry schema

Show telemetry JSONL and SQL table schemas

### Synopsis

Show telemetry JSONL fact table and SQL table schemas.

Lists every field in the telemetry fact tables (session_summary, tool_usage,
tool_call_fact, session_fact) and the SQLite cache tables (telemetry_sessions,
telemetry_tool_usage). Useful for agents and operators building queries
against harvested telemetry data.

```text
backlogit telemetry schema [flags]
```

### Options

```text
      --format string   Output format: text, json, markdown (default "text")
  -h, --help            help for schema
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
      --no-update-check    skip the remote latest-release check
```

### SEE ALSO

* [backlogit telemetry](backlogit_telemetry.md)	 - Inspect Copilot CLI token usage and tool telemetry

