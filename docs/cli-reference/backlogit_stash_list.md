---
title: "backlogit stash list"
description: "List the current active stash entries"
ms.date: 2026-04-19
---

## backlogit stash list

List the current active stash entries

```text
backlogit stash list [flags]
```

### Examples

```text
  backlogit stash list
  backlogit stash list --priority high
  backlogit stash list --kind feature
  backlogit stash list --group-by-priority
```

### Options

```text
      --format string       output format: table, json, tile (default "json")
      --group-by-priority   group stash entries by priority
  -h, --help                help for list
      --kind string         filter stash entries by kind (feature, task, bug, epic, unknown)
      --priority string     filter stash entries by priority
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit stash](backlogit_stash.md)	 - Manage the deferred work stash

