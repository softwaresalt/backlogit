---
chunk_strategy: h1-h2-h3
description: Add an item to a shipment
doc_type: reference
ingested_at: "2026-07-04T00:42:54Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_shipment_add.md
title: backlogit shipment add
---

## backlogit shipment add

Add an item to a shipment

### Synopsis

Add a backlog item to a shipment manifest.

This mirrors the backlogit_add_to_shipment MCP tool: it takes positional
<shipment-id> and <item-id> arguments and associates the item via the shared
core mutation. It is idempotent when the item already belongs to this shipment;
adding an item already assigned to another shipment is refused.

```text
backlogit shipment add <shipment-id> <item-id> [flags]
```

### Examples

```text
  backlogit shipment add 001-S 001.001-T
```

### Options

```text
  -h, --help   help for add
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
      --no-update-check    skip the remote latest-release check
```

### SEE ALSO

* [backlogit shipment](backlogit_shipment.md)	 - Manage shipment work groups

