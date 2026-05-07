---
title: "backlogit stash add"
description: "Add an item to the stash"
---

## backlogit stash add

Add an item to the stash

```text
backlogit stash add <text> [flags]
```

### Examples

```text
  backlogit stash add "Investigate tenant-specific rate limits" --kind feature --priority high
  backlogit stash add "Document migration edge cases" --kind task
```

### Options

```text
  -h, --help              help for add
      --kind string       stash item kind (feature, task, bug, epic, unknown, spike, subtask, deliberation, review, shipment; workspace-configured WIT types are also accepted) (default "task")
      --priority string   stash priority (low, medium, high, critical) (default "medium")
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit stash](backlogit_stash.md)	 - Manage the deferred work stash

