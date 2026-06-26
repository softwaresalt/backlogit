---
chunk_strategy: h1-h2-h3
description: Rehydrate the SQLite index from Markdown source files
doc_type: reference
ingested_at: "2026-06-26T02:27:58Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_sync.md
title: backlogit sync
---

## backlogit sync

Rehydrate the SQLite index from Markdown source files

### Synopsis

Rebuild the backlogit SQLite cache from the Markdown and JSONL files in
the workspace.

Use this after making manual changes to .backlogit files or when you want to
force the disposable cache to match the file-backed source of truth.

```text
backlogit sync [flags]
```

### Examples

```text
  backlogit sync
  backlogit --cwd D:\Source\MyProject sync
```

### Options

```text
  -h, --help   help for sync
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit](backlogit.md)	 - Backlogit — AI-native agile workspace

