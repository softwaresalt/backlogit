---
chunk_strategy: h1-h2-h3
description: Delete an artifact
doc_type: reference
ingested_at: "2026-06-26T02:27:58Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_delete.md
title: backlogit delete
---

## backlogit delete

Delete an artifact

### Synopsis

Delete an artifact from the workspace and remove it from the index.

This is a destructive operation and requires --force.

```text
backlogit delete <id> [flags]
```

### Examples

```text
  backlogit delete 001.001-T --force
```

### Options

```text
      --force   skip confirmation and delete immediately
  -h, --help    help for delete
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

