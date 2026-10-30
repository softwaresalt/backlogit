---
chunk_strategy: h1-h2-h3
description: Remove an item from a shipment manifest
doc_type: reference
ingested_at: "2026-10-30T16:21:42Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_shipment_remove.md
title: backlogit shipment remove
---

## backlogit shipment remove

Remove an item from a shipment manifest

### Synopsis

Remove a backlog item from a queued shipment manifest.

This mirrors the backlogit_remove_from_shipment MCP tool. Only queued shipments
may be changed unless --force is supplied together with --reason; a blocked
shipment is never modifiable. Use --dry-run to preview the resulting manifest.

```text
backlogit shipment remove <shipment-id> <item-id> [flags]
```

### Examples

```text
  backlogit shipment remove 001-S 001.001-T --reason "descope"
  backlogit shipment remove 001-S 001.001-T --dry-run
```

### Options

```text
      --dry-run         preview the change without writing
      --force           override the queued-only guard (requires --reason)
  -h, --help            help for remove
      --reason string   operator justification (required with --force)
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

