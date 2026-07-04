---
chunk_strategy: h1-h2-h3
description: Manage directed semantic links between artifacts
doc_type: reference
ingested_at: "2026-07-04T05:20:25Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_link.md
title: backlogit link
---

## backlogit link

Manage directed semantic links between artifacts

### Synopsis

Create, remove, and list directed semantic links between artifacts.

These commands are the CLI fallback for the MCP add_link/remove_link/get_links
tools. They reuse the same shared core path so behavior is identical across
surfaces; success output is JSON isomorphic to the MCP tool results.

### Options

```text
  -h, --help   help for link
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit](backlogit.md)	 - Backlogit — AI-native agile workspace
* [backlogit link add](backlogit_link_add.md)	 - Create a directed semantic link from source to target
* [backlogit link list](backlogit_link_list.md)	 - List outgoing semantic links for an artifact
* [backlogit link remove](backlogit_link_remove.md)	 - Remove a directed semantic link

