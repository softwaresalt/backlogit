---
chunk_strategy: h1-h2-h3
description: Create a shipment
doc_type: reference
ingested_at: "2026-08-02T05:40:00Z"
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
  backlogit shipment create --title "High priority sprint" --items 010-F --priority high
```

### Options

```text
  -h, --help               help for create
      --items string       comma-separated item IDs
      --priority string    shipment priority: critical, high, medium, low (optional; empty sorts last)
      --title string       shipment title
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
      --no-update-check    skip the remote latest-release check
```

### Priority and queue ordering

When `--priority` is supplied, the shipment enters the queue with that priority and the
`queue view --type shipment --sort priority` command will use it for ordering.
Shipments without a priority sort last with a deterministic `id ASC` tie-break.
Priority is lenient: an unrecognized value sorts last rather than being rejected.

To order shipments by priority:

```text
backlogit queue view --type shipment --status queued --sort priority
```

### SEE ALSO

* [backlogit shipment](backlogit_shipment.md)	 - Manage shipment work groups

