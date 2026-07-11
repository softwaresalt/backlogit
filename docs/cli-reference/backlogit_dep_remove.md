---
chunk_strategy: h1-h2-h3
description: Remove a dependency edge
doc_type: reference
ingested_at: "2026-06-26T02:27:58Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_dep_remove.md
title: backlogit dep remove
---

## backlogit dep remove

Remove a dependency edge

```text
backlogit dep remove <item-id> <depends-on> [flags]
```

### Examples

```text
  backlogit dep remove 001.002-T 001.001-T
```

### Options

```text
  -h, --help   help for remove
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
      --no-update-check    skip the remote latest-release check
```

### SEE ALSO

* [backlogit dep](backlogit_dep.md)	 - Manage artifact dependencies

