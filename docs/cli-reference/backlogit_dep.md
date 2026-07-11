---
chunk_strategy: h1-h2-h3
description: Manage artifact dependencies
doc_type: reference
ingested_at: "2026-06-26T02:27:58Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_dep.md
title: backlogit dep
---

## backlogit dep

Manage artifact dependencies

### Synopsis

Manage explicit dependency edges between work items.

Dependencies are stored in the backlogit index and can be queried from both the
CLI and MCP tools.

### Options

```text
  -h, --help   help for dep
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
* [backlogit dep add](backlogit_dep_add.md)	 - Add a dependency edge
* [backlogit dep list](backlogit_dep_list.md)	 - List dependencies for an artifact
* [backlogit dep remove](backlogit_dep_remove.md)	 - Remove a dependency edge

