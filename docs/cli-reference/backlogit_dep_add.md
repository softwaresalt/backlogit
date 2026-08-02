---
chunk_strategy: h1-h2-h3
description: Add a dependency edge
doc_type: reference
ingested_at: "2026-08-02T05:40:00Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_dep_add.md
title: backlogit dep add
---

## backlogit dep add

Add a dependency edge

```text
backlogit dep add <item-id> <depends-on> [flags]
```

### Examples

```text
  backlogit dep add 001.002-T 001.001-T
  backlogit dep add 010-T 002-F --type blocks

  # Shipment-to-shipment sequencing: "010-S must ship before 020-S"
  backlogit dep add 020-S 010-S --type blocks
```

### Options

```text
  -h, --help          help for add
      --type string   dependency relationship type (default "blocks")
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
      --no-update-check    skip the remote latest-release check
```

### Shipment-to-shipment blocking order

When the source (`<item-id>`) is a shipment and `--type blocks` (the default),
the command is routed through `AddShipmentBlock` which validates that the
destination is also a shipment.

The direction is **dependent depends_on prerequisite**: to express "shipment A
must ship before shipment B", the edge is:

```text
backlogit dep add <B> <A> --type blocks
```

B depends on A; B is suppressed in the queued-shipment view until A reaches a
terminal status (shipped, abandoned, or similar). The edge persists through
`sync_index` (Rehydrate rebuilds it as `blocks`).

For non-`blocks` dependency types (`relates_to`, `parent_of`), the generic
`AddDependency` path is used regardless of endpoint types — no endpoint
validation is applied and both endpoints may be of any artifact type.

### Deferred: MCP `get_queue` priority-sort read parity

The MCP `backlogit_get_queue` handler currently sorts by `created_at` (ignores
`sort_by`). Priority-sort read parity for the MCP queue surface is explicitly
deferred as a follow-up. The CLI `queue view --sort priority` is the current
priority-ordered read surface.

### SEE ALSO

* [backlogit dep](backlogit_dep.md)	 - Manage artifact dependencies

