---
chunk_strategy: h1-h2-h3
description: Get and validate a specific checkpoint
doc_type: reference
ingested_at: "2026-06-26T02:27:58Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_checkpoint_get.md
title: backlogit checkpoint get
---

## backlogit checkpoint get

Get and validate a specific checkpoint

```text
backlogit checkpoint get <filename> [flags]
```

### Examples

```text
  backlogit checkpoint get checkpoint-20260423-100000.json
```

### Options

```text
  -h, --help   help for get
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit checkpoint](backlogit_checkpoint.md)	 - Manage session state checkpoints

