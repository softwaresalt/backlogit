---
chunk_strategy: h1-h2-h3
description: Get a shipment by ID
doc_type: reference
ingested_at: "2026-06-26T02:27:58Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_shipment_get.md
title: backlogit shipment get
---

## backlogit shipment get

Get a shipment by ID

### Synopsis

Get a shipment by ID and print it as JSON.

The response includes a top-level "covering_feature" object ({id, title}) when
the shipment manifest contains a root covering feature. This field is a
read-only, render-time derivation from the manifest — it is never stored on the
shipment and is omitted entirely when the shipment has no covering feature.

```text
backlogit shipment get <id> [flags]
```

### Examples

```text
  backlogit shipment get 001-S
```

### Options

```text
  -h, --help   help for get
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

