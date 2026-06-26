---
chunk_strategy: h1-h2-h3
description: List shipments
doc_type: reference
ingested_at: "2026-06-26T02:27:58Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_shipment_list.md
title: backlogit shipment list
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
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit shipment](backlogit_shipment.md)	 - Manage shipment work groups

