---
title: "backlogit dep list"
description: "List dependencies for an artifact"
---

## backlogit dep list

List dependencies for an artifact

```text
backlogit dep list <item-id> [flags]
```

### Examples

```text
  backlogit dep list 001.002-T
  backlogit dep list 001.001-T --reverse
```

### Options

```text
  -h, --help      help for list
      --reverse   show items that depend on this item
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit dep](backlogit_dep.md)	 - Manage artifact dependencies

