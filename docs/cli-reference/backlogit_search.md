---
chunk_strategy: h1-h2-h3
description: Full-text search across artifacts
doc_type: reference
ingested_at: "2026-06-26T02:27:58Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_search.md
title: backlogit search
---

## backlogit search

Full-text search across artifacts

### Synopsis

Search the full-text index for matching artifacts.

Use this when you want quick keyword lookup without writing SQL.

```text
backlogit search <query> [flags]
```

### Examples

```text
  backlogit search authentication
  backlogit search "token rotation" --limit 10
```

### Options

```text
  -h, --help        help for search
      --limit int   maximum number of results (default 20)
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

