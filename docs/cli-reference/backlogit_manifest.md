---
chunk_strategy: h1-h2-h3
description: Print a JSON-RPC manifest of all backlogit MCP tool definitions
doc_type: reference
ingested_at: "2026-06-26T02:27:58Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_manifest.md
title: backlogit manifest
---

## backlogit manifest

Print a JSON-RPC manifest of all backlogit MCP tool definitions

### Synopsis

Print the manifest of all backlogit MCP tools as a JSON object.

The output format is compatible with the MCP tools/list response:

  {"tools": [{"name": "...", "description": "...", "inputSchema": {...}}, ...]}

Tools are sorted alphabetically by name to match MCP tools/list ordering.
This allows agents to discover the full backlogit tool surface through the CLI
in the same format they receive during MCP server initialization. Combine with
--jsonrpc to receive a JSON-RPC 2.0 response envelope.

```text
backlogit manifest [flags]
```

### Examples

```text
  backlogit manifest
  backlogit manifest | jq '.tools[].name'
  backlogit --jsonrpc manifest
```

### Options

```text
  -h, --help   help for manifest
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
      --no-update-check    skip the remote latest-release check
```

### SEE ALSO

* [backlogit](backlogit.md)	 - Backlogit — AI-native agile workspace

