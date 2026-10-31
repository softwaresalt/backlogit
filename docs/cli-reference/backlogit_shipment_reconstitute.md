---
chunk_strategy: h1-h2-h3
description: Reconstitute shipments from a declarative plan
doc_type: reference
ingested_at: "2026-10-31T10:22:49Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_shipment_reconstitute.md
title: backlogit shipment reconstitute
---

## backlogit shipment reconstitute

Reconstitute shipments from a declarative plan

### Synopsis

Declaratively split or regroup queued shipments from a YAML plan.

This mirrors the backlogit_reconstitute_shipment MCP tool. The plan is validated
and applied atomically; only queued shipments may be changed unless --force is
supplied together with --reason. Use --dry-run to preview the result without
writing. Output is always JSON; --json is accepted for symmetry with other commands.

```text
backlogit shipment reconstitute --plan <plan-file> [flags]
```

### Examples

```text
  backlogit shipment reconstitute --plan plans/split.yaml --dry-run
  backlogit shipment reconstitute --plan plans/split.yaml --json
```

### Options

```text
      --dry-run         preview the change without writing
      --force           override the queued-only guard (requires --reason)
  -h, --help            help for reconstitute
      --json            emit JSON output (default)
      --plan string     path to the reconstitution plan YAML file
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

