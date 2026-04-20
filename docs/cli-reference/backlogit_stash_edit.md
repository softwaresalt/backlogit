---
title: "backlogit stash edit"
description: "Edit a stash entry's text, kind, or priority"
ms.date: 2026-04-19
---

## backlogit stash edit

Edit a stash entry's text, kind, or priority

```text
backlogit stash edit <stash-id> [flags]
```

### Examples

```text
  backlogit stash edit ABCD1234 --kind feature
  backlogit stash edit ABCD1234 --priority high
  backlogit stash edit ABCD1234 --text "Updated description"
```

### Options

```text
  -h, --help              help for edit
      --kind string       new stash item kind (feature, task, bug, epic, unknown)
      --priority string   new stash priority (low, medium, high, critical)
      --text string       new stash item text
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit stash](backlogit_stash.md)	 - Manage the deferred work stash

