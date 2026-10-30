---
chunk_strategy: h1-h2-h3
description: Reorder a shipment manifest
doc_type: reference
ingested_at: "2026-10-30T16:21:42Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_shipment_reorder.md
title: backlogit shipment reorder
---

## backlogit shipment reorder

Reorder a shipment manifest

### Synopsis

Reorder a queued shipment manifest.

This mirrors the backlogit_reorder_shipment MCP tool. Supply exactly one of
--items (an explicit comma-separated order that must be a permutation of the
current members and respect dependencies) or --auto (dependency-respecting
order). Use --dry-run to preview the resulting order.

```text
backlogit shipment reorder <shipment-id> [flags]
```

### Examples

```text
  backlogit shipment reorder 001-S --auto --dry-run
  backlogit shipment reorder 001-S --items 001-F,001.001-T,001.002-T
```

### Options

```text
      --auto            compute a dependency-respecting order
      --dry-run         preview the change without writing
      --force           override the queued-only guard (requires --reason)
  -h, --help            help for reorder
      --items string    explicit comma-separated manifest order
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

