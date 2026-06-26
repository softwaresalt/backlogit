---
chunk_strategy: h1-h2-h3
description: Add a dependency edge
doc_type: reference
ingested_at: "2026-06-26T02:27:58Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_dep_add.md
title: backlogit dep add
---

## backlogit dep add

Add a dependency edge

```text
backlogit dep add <item-id> <depends-on> [flags]
```

### Examples

```text
  backlogit dep add 001.002-T 001.001-T
  backlogit dep add 010-T 002-F --type blocks
```

### Options

```text
  -h, --help          help for add
      --type string   dependency relationship type (default "blocks")
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit dep](backlogit_dep.md)	 - Manage artifact dependencies

