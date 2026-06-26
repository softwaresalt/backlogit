---
chunk_strategy: h1-h2-h3
description: List dependencies for an artifact
doc_type: reference
ingested_at: "2026-06-26T02:27:58Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_dep_list.md
title: backlogit dep list
---

## backlogit dep list

List dependencies for an artifact

```text
backlogit dep list <item-id> [flags]
```

### Examples

```text
  backlogit dep list 001.002-T
  backlogit dep list 001.001-T --reverse
```

### Options

```text
  -h, --help      help for list
      --reverse   show items that depend on this item
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit dep](backlogit_dep.md)	 - Manage artifact dependencies

