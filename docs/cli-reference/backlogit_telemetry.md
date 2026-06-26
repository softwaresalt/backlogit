---
chunk_strategy: h1-h2-h3
description: Inspect Copilot CLI token usage and tool telemetry
doc_type: reference
ingested_at: "2026-06-26T02:27:58Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_telemetry.md
title: backlogit telemetry
---

## backlogit telemetry

Inspect Copilot CLI token usage and tool telemetry

### Synopsis

Inspect Copilot CLI token usage and tool telemetry

Use telemetry harvest to parse logs, telemetry report for machine-readable
session and server summaries, and telemetry top to rank servers by
proportional token attribution.

See https://github.com/softwaresalt/backlogit/blob/main/docs/telemetry-fields.md
for harvested field definitions and SQLite column mappings.

### Options

```text
  -h, --help   help for telemetry
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit](backlogit.md)	 - Backlogit — AI-native agile workspace
* [backlogit telemetry branch](backlogit_telemetry_branch.md)	 - Show per-branch telemetry metrics with type classification and enrichment
* [backlogit telemetry harvest](backlogit_telemetry_harvest.md)	 - Parse Copilot CLI logs and write telemetry-sessions.jsonl
* [backlogit telemetry list](backlogit_telemetry_list.md)	 - List harvested session summaries
* [backlogit telemetry report](backlogit_telemetry_report.md)	 - Generate a formatted telemetry report from harvested data
* [backlogit telemetry schema](backlogit_telemetry_schema.md)	 - Show telemetry JSONL and SQL table schemas
* [backlogit telemetry top](backlogit_telemetry_top.md)	 - Show top N servers by token usage
* [backlogit telemetry trend](backlogit_telemetry_trend.md)	 - Show token usage trends grouped by date, branch, or model class

