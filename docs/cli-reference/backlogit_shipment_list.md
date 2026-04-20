---
title: "backlogit shipment list"
description: "List shipments"
---

## backlogit shipment list

List shipments

```text
backlogit shipment list [flags]
```

### Examples

```text
  backlogit shipment list --status active
```

### Options

```text
      --format string   output format: table, json, tile (default "json")
  -h, --help            help for list
      --status string   filter shipments by status
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit shipment](backlogit_shipment.md)	 - Manage shipment work groups

