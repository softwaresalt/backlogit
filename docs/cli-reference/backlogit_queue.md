---
chunk_strategy: h1-h2-h3
description: Manage the work queue
doc_type: reference
ingested_at: "2026-06-26T02:27:58Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_queue.md
title: backlogit queue
---

## backlogit queue

Manage the work queue

### Synopsis

View and manipulate the indexed work queue.

Use queue view for grouped queue output, queue move to reorder items, and
queue bulk-status to update multiple items in one command.

### Options

```text
  -h, --help   help for queue
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
      --no-update-check    skip the remote latest-release check
```

### SEE ALSO

* [backlogit](backlogit.md)	 - Backlogit — AI-native agile workspace
* [backlogit queue bulk-status](backlogit_queue_bulk-status.md)	 - Update status for multiple items
* [backlogit queue move](backlogit_queue_move.md)	 - Reorder an item in the queue
* [backlogit queue view](backlogit_queue_view.md)	 - View queue items

