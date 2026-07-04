---
chunk_strategy: h1-h2-h3
description: List outgoing semantic links for an artifact
doc_type: reference
ingested_at: "2026-07-04T05:20:25Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_link_list.md
title: backlogit link list
---

## backlogit link list

List outgoing semantic links for an artifact

```text
backlogit link list <id> [flags]
```

### Examples

```text
  backlogit link list 001-F
  backlogit link list 001-F --type related_to
```

### Options

```text
  -h, --help          help for list
      --type string   filter to a single link_type (optional)
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit link](backlogit_link.md)	 - Manage directed semantic links between artifacts

