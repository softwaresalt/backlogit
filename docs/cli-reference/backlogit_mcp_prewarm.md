---
chunk_strategy: h1-h2-h3
description: Start or attach to the workspace background MCP daemon
doc_type: reference
ingested_at: "2026-11-09T08:21:42Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_mcp_prewarm.md
title: backlogit mcp prewarm
---

## backlogit mcp prewarm

Start or attach to the workspace background MCP daemon

```text
backlogit mcp prewarm [flags]
```

### Options

```text
      --format string           output format: text or json (default "text")
  -h, --help                    help for prewarm
      --idle-timeout duration   stop after this duration with no active sessions (default 15m0s)
      --replace                 stop any running workspace MCP daemon before starting
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

