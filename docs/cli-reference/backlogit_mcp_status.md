---
chunk_strategy: h1-h2-h3
description: Report whether a backlogit MCP daemon is running for the workspace
doc_type: reference
ingested_at: "2026-10-30T02:16:41Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_mcp_status.md
title: backlogit mcp status
---

## backlogit mcp status

Report whether a backlogit MCP daemon is running for the workspace

```text
backlogit mcp status [flags]
```

### Examples

```text
  backlogit mcp status --format json
```

### Options

```text
      --format string   output format: text or json (default "text")
  -h, --help            help for status
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

