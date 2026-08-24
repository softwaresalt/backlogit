---
chunk_strategy: h1-h2-h3
description: Manage shipment work groups
doc_type: reference
ingested_at: "2026-06-26T02:27:58Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_shipment.md
title: backlogit shipment
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
      --no-update-check    skip the remote latest-release check
```

### SEE ALSO

* [backlogit](backlogit.md)	 - Backlogit — AI-native agile workspace
* [backlogit shipment add](backlogit_shipment_add.md)	 - Add an item to a shipment
* [backlogit shipment claim](backlogit_shipment_claim.md)	 - Claim a queued shipment
* [backlogit shipment create](backlogit_shipment_create.md)	 - Create a shipment
* [backlogit shipment get](backlogit_shipment_get.md)	 - Get a shipment by ID
* [backlogit shipment list](backlogit_shipment_list.md)	 - List shipments
* [backlogit shipment repair-evidence](backlogit_shipment_repair-evidence.md)	 - Repair stale gate evidence for a shipment member
* [backlogit shipment return-blocked](backlogit_shipment_return-blocked.md)	 - Return a blocked item from a shipment
* [backlogit shipment ship](backlogit_shipment_ship.md)	 - Close a released shipment and archive the released scope

