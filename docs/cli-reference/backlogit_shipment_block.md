---
chunk_strategy: h1-h2-h3
description: Block an active shipment
doc_type: reference
ingested_at: "2026-09-22T02:24:04Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_shipment_block.md
title: backlogit shipment block
---

## backlogit shipment block

Block an active shipment

```text
backlogit shipment block <id> [flags]
```

### Examples

```text
  backlogit shipment block 001-S --reason "waiting for prerequisite" --by agent --resume-checkpoint checkpoint.json
```

### Options

```text
      --by string                  actor blocking the shipment
  -h, --help                       help for block
      --reason string              reason the shipment is blocked (required)
      --resume-checkpoint string   checkpoint reference for resuming the shipment
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

