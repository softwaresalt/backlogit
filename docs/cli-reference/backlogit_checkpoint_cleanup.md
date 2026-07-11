---
chunk_strategy: h1-h2-h3
description: Archive resolved and stale checkpoints
doc_type: reference
ingested_at: "2026-06-26T02:27:58Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_checkpoint_cleanup.md
title: backlogit checkpoint cleanup
---

## backlogit checkpoint cleanup

Archive resolved and stale checkpoints

```text
backlogit checkpoint cleanup [flags]
```

### Examples

```text
  backlogit checkpoint cleanup --retention-days 7
```

### Options

```text
  -h, --help                 help for cleanup
      --retention-days int   override retention days (defaults to config)
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

