---
chunk_strategy: h1-h2-h3
description: Claim a queued shipment
doc_type: reference
ingested_at: "2026-06-26T02:27:58Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_shipment_claim.md
title: backlogit shipment claim
---

## backlogit shipment claim

Claim a queued shipment

```text
backlogit shipment claim <id> [flags]
```

### Examples

```text
  backlogit shipment claim 001-S
```

### Options

```text
  -h, --help   help for claim
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

