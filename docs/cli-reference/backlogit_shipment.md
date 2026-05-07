---
title: "backlogit shipment"
description: "Manage shipment work groups"
---

## backlogit shipment

Manage shipment work groups

### Synopsis

Manage shipment artifacts that group related backlog items for delivery.

Use shipment commands to create a shipment, inspect its current state, list
shipments in the workspace, claim queued shipments, and return blocked items.

### Options

```text
  -h, --help   help for shipment
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit](backlogit.md)	 - Backlogit — AI-native agile workspace
* [backlogit shipment claim](backlogit_shipment_claim.md)	 - Claim a queued shipment
* [backlogit shipment create](backlogit_shipment_create.md)	 - Create a shipment
* [backlogit shipment get](backlogit_shipment_get.md)	 - Get a shipment by ID
* [backlogit shipment list](backlogit_shipment_list.md)	 - List shipments
* [backlogit shipment return-blocked](backlogit_shipment_return-blocked.md)	 - Return a blocked item from a shipment
* [backlogit shipment ship](backlogit_shipment_ship.md)	 - Close a released shipment and archive the released scope

