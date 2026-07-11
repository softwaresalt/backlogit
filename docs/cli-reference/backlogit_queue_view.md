---
chunk_strategy: h1-h2-h3
description: View queue items
doc_type: reference
ingested_at: "2026-06-26T02:27:58Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_queue_view.md
title: backlogit queue view
---

## backlogit queue view

View queue items

### Synopsis

View active work queue items.

By default, queue view shows queued, active, blocked, and review items using
priority as the secondary sort after any manually assigned queue positions.

```text
backlogit queue view [flags]
```

### Examples

```text
  backlogit queue view
  backlogit queue view --status active --group-by type
  backlogit queue view --sort priority
```

### Options

```text
      --format string     output format: table, json, tile (default "table")
      --group-by string   group output by field
  -h, --help              help for view
      --sort string       sort by field (default "priority")
      --status string     filter by status
      --type string       filter by artifact type
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
      --no-update-check    skip the remote latest-release check
```

### SEE ALSO

* [backlogit queue](backlogit_queue.md)	 - Manage the work queue

