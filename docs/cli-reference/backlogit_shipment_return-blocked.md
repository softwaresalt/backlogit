---
chunk_strategy: h1-h2-h3
description: Return a blocked item from a shipment
doc_type: reference
ingested_at: "2026-06-26T02:27:58Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_shipment_return-blocked.md
title: backlogit shipment return-blocked
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
      --no-update-check    skip the remote latest-release check
```

### SEE ALSO

* [backlogit shipment](backlogit_shipment.md)	 - Manage shipment work groups

