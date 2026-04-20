---
title: "backlogit dep remove"
description: "Remove a dependency edge"
---

## backlogit dep remove

Remove a dependency edge

```text
backlogit dep remove <item-id> <depends-on> [flags]
```

### Examples

```text
  backlogit dep remove 001.002-T 001.001-T
```

### Options

```text
  -h, --help   help for remove
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit dep](backlogit_dep.md)	 - Manage artifact dependencies

