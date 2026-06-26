---
chunk_strategy: h1-h2-h3
description: Update status for multiple items
doc_type: reference
ingested_at: "2026-06-26T02:27:58Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_queue_bulk-status.md
title: backlogit queue bulk-status
---

## backlogit queue bulk-status

Update status for multiple items

```text
backlogit queue bulk-status [flags]
```

### Examples

```text
  backlogit queue bulk-status --ids 001.001-T,001.002-T,001.003-T --status active
```

### Options

```text
  -h, --help            help for bulk-status
      --ids string      comma-separated list of item IDs
      --status string   target status
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit queue](backlogit_queue.md)	 - Manage the work queue

