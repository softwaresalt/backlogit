---
chunk_strategy: h1-h2-h3
description: List session state checkpoints
doc_type: reference
ingested_at: "2026-06-26T02:27:58Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_checkpoint_list.md
title: backlogit checkpoint list
---

## backlogit checkpoint list

List session state checkpoints

```text
backlogit checkpoint list [flags]
```

### Examples

```text
  backlogit checkpoint list --agent ship --status active
```

### Options

```text
      --agent string          filter by agent (ship, stage)
      --feature-id string     filter by feature ID
  -h, --help                  help for list
      --max-age-hours float   maximum age in hours
      --shipment-id string    filter by shipment ID
      --status string         filter by status (active, resolved)
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
      --no-update-check    skip the remote latest-release check
```

### SEE ALSO

* [backlogit checkpoint](backlogit_checkpoint.md)	 - Manage session state checkpoints

