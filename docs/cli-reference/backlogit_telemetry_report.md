---
title: "backlogit telemetry report"
description: "Generate a formatted telemetry report from harvested data"
---

## backlogit telemetry report

Generate a formatted telemetry report from harvested data

```text
backlogit telemetry report [flags]
```

### Options

```text
      --by string        Group output by: session, server, model, class, tool, context (default "session")
      --format string    Output format: table, json, markdown (default "table")
  -h, --help             help for report
      --limit int        Restrict the number of rows returned (0 = no limit)
      --session string   Filter report to a single session ID
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit telemetry](backlogit_telemetry.md)	 - Inspect Copilot CLI token usage and tool telemetry

