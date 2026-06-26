---
chunk_strategy: h1-h2-h3
description: Execute a read-only SQL query against the index
doc_type: reference
ingested_at: "2026-06-26T02:27:58Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_query.md
title: backlogit query
---

## backlogit query

Execute a read-only SQL query against the index

### Synopsis

Execute a gated read-only SQL query against the backlogit SQLite cache.

Only SELECT statements are allowed. Use this for token-efficient inspection of
items, dependencies, logs, and indexed stash data.

```text
backlogit query "<sql>" [flags]
```

### Examples

```text
  backlogit query "SELECT id, title, status FROM items ORDER BY updated_at DESC LIMIT 20"
  backlogit query "SELECT stash_id, kind, state FROM stash_entries ORDER BY updated_at DESC"
```

### Options

```text
  -h, --help   help for query
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit](backlogit.md)	 - Backlogit — AI-native agile workspace

