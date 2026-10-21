---
chunk_strategy: h1-h2-h3
description: Dispose a queued shipment (Stage-only; needs operator deliberation and approval)
doc_type: reference
ingested_at: "2026-10-21T03:53:30Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_shipment_dispose.md
title: backlogit shipment dispose
---

## backlogit shipment dispose

Dispose a queued shipment (Stage-only; needs operator deliberation and approval)

### Synopsis

Move a queued shipment to abandoned under a decided operator deliberation.

This is a Stage-only operation (MCP tool backlogit_dispose_shipment; CLI
backlogit shipment dispose). It needs a decided deliberation that names this
shipment in its authorizes_disposition field, and in-session operator approval
before the call. Ship and the Orchestrator never run it, against any
workspace. Only queued shipments can be disposed.

```text
backlogit shipment dispose <id> [flags]
```

### Examples

```text
  backlogit shipment dispose 001-S --reason "superseded" --by stage --authorization-ref 001-DL --confirm
```

### Options

```text
      --authorization-ref string   decided deliberation ID that authorizes this disposition (required)
      --by string                  actor disposing the shipment (required)
      --confirm                    confirm the disposition (required)
  -h, --help                       help for dispose
      --reason string              reason for disposing the shipment (required)
      --superseded-by string       shipment ID that supersedes the disposed shipment
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

