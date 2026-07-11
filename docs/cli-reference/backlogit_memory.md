---
chunk_strategy: h1-h2-h3
description: Persist keyed agent session memory
doc_type: reference
ingested_at: "2026-07-04T05:20:25Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_memory.md
title: backlogit memory
---

## backlogit memory

Persist keyed agent session memory

### Synopsis

Save keyed session memory summaries.

This is the CLI fallback for the MCP save_memory tool. It writes to
.backlogit/memories.json at the resolved workspace root, matching the MCP
handler's path resolution so a save invoked from a subdirectory targets the
correct workspace.

### Options

```text
  -h, --help   help for memory
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
* [backlogit memory save](backlogit_memory_save.md)	 - Save a keyed memory summary to .backlogit/memories.json

