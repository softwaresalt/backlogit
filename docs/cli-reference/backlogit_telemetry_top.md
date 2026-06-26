---
chunk_strategy: h1-h2-h3
description: Show top N servers by token usage
doc_type: reference
ingested_at: "2026-06-26T02:27:58Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_telemetry_top.md
title: backlogit telemetry top
---

## backlogit telemetry top

Show top N servers by token usage

```text
backlogit telemetry top [flags]
```

### Options

```text
  -h, --help    help for top
      --n int   Number of top entries to display (default 10)
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit telemetry](backlogit_telemetry.md)	 - Inspect Copilot CLI token usage and tool telemetry

