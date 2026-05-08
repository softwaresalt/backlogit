---
title: "backlogit shipment return-blocked"
description: "Return a blocked item from a shipment"
---

## backlogit shipment return-blocked

Return a blocked item from a shipment

```text
backlogit shipment return-blocked [flags]
```

### Examples

```text
  backlogit shipment return-blocked --shipment 001-S --item 001.001-T --reason "blocked"
```

### Options

```text
  -h, --help              help for return-blocked
      --item string       item ID
      --reason string     blocked reason
      --shipment string   shipment ID
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit shipment](backlogit_shipment.md)	 - Manage shipment work groups

