---
title: "backlogit dep add"
description: "Add a dependency edge"
---

## backlogit dep add

Add a dependency edge

```text
backlogit dep add <item-id> <depends-on> [flags]
```

### Examples

```text
  backlogit dep add 001.002-T 001.001-T
  backlogit dep add 010-T 002-F --type blocks
```

### Options

```text
  -h, --help          help for add
      --type string   dependency relationship type (default "blocks")
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit dep](backlogit_dep.md)	 - Manage artifact dependencies

