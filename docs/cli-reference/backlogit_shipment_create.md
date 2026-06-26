---
chunk_strategy: h1-h2-h3
description: Create a shipment
doc_type: reference
ingested_at: "2026-06-26T02:27:58Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_shipment_create.md
title: backlogit shipment create
---

## backlogit shipment create

Create a shipment

```text
backlogit shipment create [flags]
```

### Examples

```text
  backlogit shipment create --title "Sprint 1" --items 001-F,001.001-T
```

### Options

```text
  -h, --help           help for create
      --items string   comma-separated item IDs
      --title string   shipment title
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit shipment](backlogit_shipment.md)	 - Manage shipment work groups

