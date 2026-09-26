---
chunk_strategy: h1-h2-h3
description: Unblock a shipment
doc_type: reference
ingested_at: "2026-09-22T02:24:04Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_shipment_unblock.md
title: backlogit shipment unblock
---

## backlogit shipment unblock

Unblock a shipment

```text
backlogit shipment unblock <id> [flags]
```

### Examples

```text
  backlogit shipment unblock 001-S --to active --confirm --by agent
```

### Options

```text
      --by string   actor unblocking the shipment
      --confirm     confirm the unblock transition
  -h, --help        help for unblock
      --to string   target shipment status (queued or active)
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

