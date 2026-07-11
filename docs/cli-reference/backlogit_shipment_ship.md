---
chunk_strategy: h1-h2-h3
description: Close a released shipment and archive the released scope
doc_type: reference
ingested_at: "2026-06-26T02:27:58Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_shipment_ship.md
title: backlogit shipment ship
---

## backlogit shipment ship

Close a released shipment and archive the released scope

```text
backlogit shipment ship <id> [flags]
```

### Examples

```text
  backlogit shipment ship 001-S --sha deadbeef --message "merge: release" --author "dev@example.com"
```

### Options

```text
      --author string    merge commit author to record on released artifacts
  -h, --help             help for ship
      --message string   merge commit message to record on released artifacts
      --sha string       merge commit SHA to record on released artifacts
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

