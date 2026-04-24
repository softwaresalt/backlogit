---
title: "backlogit checkpoint list"
description: "List session state checkpoints"
---

## backlogit checkpoint list

List session state checkpoints

```text
backlogit checkpoint list [flags]
```

### Examples

```text
  backlogit checkpoint list --agent ship --status active
```

### Options

```text
      --agent string          filter by agent (ship, stage)
      --feature-id string     filter by feature ID
  -h, --help                  help for list
      --max-age-hours float   maximum age in hours
      --shipment-id string    filter by shipment ID
      --status string         filter by status (active, resolved)
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit checkpoint](backlogit_checkpoint.md)	 - Manage session state checkpoints

