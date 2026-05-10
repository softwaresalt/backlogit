---
title: "backlogit telemetry trend"
description: "Show token usage trends grouped by date, branch, or model class"
---

## backlogit telemetry trend

Show token usage trends grouped by date, branch, or model class

### Synopsis

Show token usage trends grouped by date, branch, or model class.

Each output row contains:
  - Group (date YYYY-MM-DD, branch name, or model class)
  - Session count
  - Total tokens
  - Avg tokens per session
  - Avg tokens per task (when available)
  - Avg peak context utilisation (when available)

Use --by branch to switch from date grouping to branch grouping.
Use --by class to group by model class (sonnet, haiku, gpt, o-series, etc.).

```text
backlogit telemetry trend [flags]
```

### Options

```text
      --by string       Group output by: date, branch, class, branch-type (default "date")
      --format string   Output format: table, json, markdown (default "table")
  -h, --help            help for trend
      --limit int       Restrict the number of groups returned (0 = no limit)
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit telemetry](backlogit_telemetry.md)	 - Inspect Copilot CLI token usage and tool telemetry

