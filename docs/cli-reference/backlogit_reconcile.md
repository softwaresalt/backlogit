---
chunk_strategy: h1-h2-h3
description: Reconcile archived items by correcting their lifecycle status
doc_type: reference
ingested_at: "2026-08-30T00:21:35Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_reconcile.md
title: backlogit reconcile
---

## backlogit reconcile

Reconcile archived items by correcting their lifecycle status

### Synopsis

Reconcile archived items whose archived_status does not reflect the correct
terminal status.  Each item is unarchived, updated to the target status, and
re-archived with a durable lifecycle_reconciliation event that preserves the
original archive history.

The operation is idempotent: items already at the target status are returned
as no_op without modification.

```text
backlogit reconcile <item-id...> [flags]
```

### Examples

```text
  backlogit reconcile 001-T 002-T --reason "P-001 lifecycle fix" --actor "ship-agent"
  backlogit reconcile 001-T --reason "closed without resolution" --actor "ops" --target-status rejected
```

### Options

```text
      --actor string             Actor performing the reconciliation (required)
  -h, --help                     help for reconcile
      --idempotency-key string   Optional idempotency key for deduplication
      --reason string            Reason for reconciliation (required)
      --target-status string     Target terminal status (done, accepted, rejected, abandoned) (default "done")
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
      --no-update-check    skip the remote latest-release check
```

### SEE ALSO

* [backlogit](backlogit.md)	 - Backlogit — AI-native agile workspace

