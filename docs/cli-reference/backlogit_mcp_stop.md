---
chunk_strategy: h1-h2-h3
description: Stop the backlogit MCP daemon running for the workspace
doc_type: reference
ingested_at: "2026-10-30T02:16:41Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_mcp_stop.md
title: backlogit mcp stop
---

## backlogit mcp stop

Stop the backlogit MCP daemon running for the workspace

```text
backlogit mcp stop [flags]
```

### Examples

```text
  backlogit mcp stop --format json
```

### Options

```text
      --format string   output format: text or json (default "text")
  -h, --help            help for stop
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
      --no-update-check    skip the remote latest-release check
```

### SEE ALSO

* [backlogit mcp](backlogit_mcp.md)	 - Start the backlogit MCP stdio server

