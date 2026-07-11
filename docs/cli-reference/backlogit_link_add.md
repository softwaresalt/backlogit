---
chunk_strategy: h1-h2-h3
description: Create a directed semantic link from source to target
doc_type: reference
ingested_at: "2026-07-04T05:20:25Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_link_add.md
title: backlogit link add
---

## backlogit link add

Create a directed semantic link from source to target

```text
backlogit link add <source> <target> <type> [flags]
```

### Examples

```text
  backlogit link add 001-F 002-F related_to
```

### Options

```text
  -h, --help   help for add
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
      --no-update-check    skip the remote latest-release check
```

### SEE ALSO

* [backlogit link](backlogit_link.md)	 - Manage directed semantic links between artifacts

