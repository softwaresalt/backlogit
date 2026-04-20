---
title: "backlogit stash harvest"
description: "Harvest a stash item into a planned work item"
ms.date: 2026-04-19
---

## backlogit stash harvest

Harvest a stash item into a planned work item

```text
backlogit stash harvest [stash-id] [flags]
```

### Examples

```text
  backlogit stash harvest ABCD1234 --type feature
  backlogit stash harvest ABCD1234 --type task --parent-id 001-F --status active
  backlogit stash harvest --priority critical --type task
```

### Options

```text
      --description string   description for the harvested work item
  -h, --help                 help for harvest
      --parent-id string     optional parent work item ID
      --priority string      harvest all stash entries at the given priority
      --status string        status for the harvested work item (default "queued")
      --title string         override title for the harvested work item
      --type string          target artifact type (feature, task, subtask) (default "task")
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit stash](backlogit_stash.md)	 - Manage the deferred work stash

