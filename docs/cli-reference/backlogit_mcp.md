---
chunk_strategy: h1-h2-h3
description: Start the backlogit MCP stdio server
doc_type: reference
ingested_at: "2026-06-26T02:27:58Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_mcp.md
title: backlogit mcp
---

## backlogit mcp

Start the backlogit MCP stdio server

### Synopsis

Start the backlogit Model Context Protocol server over stdio.

Use this command from MCP-capable clients such as GitHub Copilot CLI, Claude
Code, or Cursor to expose backlogit workspace tools to agents.

```text
backlogit mcp [flags]
```

### Examples

```text
  backlogit mcp
  backlogit --cwd D:\Source\MyProject mcp
```

### Options

```text
  -h, --help   help for mcp
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit](backlogit.md)	 - Backlogit — AI-native agile workspace

