---
chunk_strategy: h1-h2-h3
description: Remove a directed semantic link
doc_type: reference
ingested_at: "2026-07-04T05:20:25Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_link_remove.md
title: backlogit link remove
---

## backlogit link remove

Remove a directed semantic link

```text
backlogit link remove <source> <target> <type> [flags]
```

### Examples

```text
  backlogit link remove 001-F 002-F related_to
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
```

### SEE ALSO

* [backlogit link](backlogit_link.md)	 - Manage directed semantic links between artifacts

