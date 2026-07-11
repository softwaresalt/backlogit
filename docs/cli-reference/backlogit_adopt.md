---
chunk_strategy: h1-h2-h3
description: Adopt an orphaned item under a new parent feature
doc_type: reference
ingested_at: "2026-06-26T02:27:58Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_adopt.md
title: backlogit adopt
---

## backlogit adopt

Adopt an orphaned item under a new parent feature

```text
backlogit adopt <item-id> [flags]
```

### Examples

```text
  backlogit adopt 015.009-T --parent 016-F
```

### Options

```text
  -h, --help            help for adopt
      --parent string   New parent feature ID (required)
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

