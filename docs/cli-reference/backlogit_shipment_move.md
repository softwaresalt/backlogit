---
chunk_strategy: h1-h2-h3
description: Move an item between shipment manifests
doc_type: reference
ingested_at: "2026-10-30T16:21:42Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_shipment_move.md
title: backlogit shipment move
---

## backlogit shipment move

Move an item between shipment manifests

### Synopsis

Atomically move a backlog item from one shipment manifest to another.

This mirrors the backlogit_move_shipment_item MCP tool. Both shipments must be
queued unless --force is supplied together with --reason; a blocked shipment is
never modifiable. Use --dry-run to preview both resulting manifests.

```text
backlogit shipment move <item-id> --from <shipment-id> --to <shipment-id> [flags]
```

### Examples

```text
  backlogit shipment move 001.001-T --from 001-S --to 002-S --reason "split scope"
```

### Options

```text
      --dry-run         preview the change without writing
      --force           override the queued-only guard (requires --reason)
      --from string     source shipment ID
  -h, --help            help for move
      --reason string   operator justification (required with --force)
      --to string       destination shipment ID
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

