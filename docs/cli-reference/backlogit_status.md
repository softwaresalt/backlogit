---
chunk_strategy: h1-h2-h3
description: Show workspace artifact summary
doc_type: reference
ingested_at: "2026-06-26T02:27:58Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_status.md
title: backlogit status
---

## backlogit status

Show workspace artifact summary

### Synopsis

Show a workspace summary grouped by artifact type and status.

This is a quick health check for the current indexed backlog state.

```text
backlogit status [flags]
```

### Examples

```text
  backlogit status
  backlogit --cwd D:\Source\MyProject status
```

### Options

```text
  -h, --help   help for status
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit](backlogit.md)	 - Backlogit — AI-native agile workspace

