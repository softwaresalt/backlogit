---
title: "backlogit queue move"
description: "Reorder an item in the queue"
---

## backlogit queue move

Reorder an item in the queue

### Synopsis

Reorder an item within the default active queue view.

Positions are 1-based and use the same default scope as queue view: queued,
active, blocked, and review items.

```text
backlogit queue move <item-id> [flags]
```

### Examples

```text
  backlogit queue move 001.001-T --position 1
```

### Options

```text
  -h, --help           help for move
      --position int   target position in the queue
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit queue](backlogit_queue.md)	 - Manage the work queue

