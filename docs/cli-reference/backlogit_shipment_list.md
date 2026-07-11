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

### Synopsis

List shipments in table, tile, or JSON format.

Table and tile output include a COVERING FEATURE column, and JSON output
includes a top-level "covering_feature" object ({id, title}) per shipment. The
covering feature is a read-only, render-time derivation from each shipment
manifest (never stored) and is blank/omitted when a shipment has no covering
feature.

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
      --no-update-check    skip the remote latest-release check
```

### SEE ALSO

* [backlogit shipment](backlogit_shipment.md)	 - Manage shipment work groups

