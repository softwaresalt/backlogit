---
chunk_strategy: h1-h2-h3
description: Save a keyed memory summary to .backlogit/memories.json
doc_type: reference
ingested_at: "2026-07-04T05:20:25Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_memory_save.md
title: backlogit memory save
---

## backlogit memory save

Save a keyed memory summary to .backlogit/memories.json

```text
backlogit memory save [flags]
```

### Examples

```text
  backlogit memory save --key session-079 --summary "shipped U1-U8"
```

### Options

```text
  -h, --help             help for save
      --key string       memory key
      --summary string   memory summary
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit memory](backlogit_memory.md)	 - Persist keyed agent session memory

