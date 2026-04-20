---
title: "backlogit shipment ship"
description: "Close a released shipment and archive the released scope"
---

## backlogit shipment ship

Close a released shipment and archive the released scope

```text
backlogit shipment ship <id> [flags]
```

### Examples

```text
  backlogit shipment ship 001-S --sha deadbeef --message "merge: release" --author "dev@example.com"
```

### Options

```text
      --author string    merge commit author to record on released artifacts
  -h, --help             help for ship
      --message string   merge commit message to record on released artifacts
      --sha string       merge commit SHA to record on released artifacts
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit shipment](backlogit_shipment.md)	 - Manage shipment work groups

