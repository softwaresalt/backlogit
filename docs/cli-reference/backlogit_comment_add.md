---
chunk_strategy: h1-h2-h3
description: Append a comment to an artifact
doc_type: reference
ingested_at: "2026-07-04T05:20:25Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_comment_add.md
title: backlogit comment add
---

## backlogit comment add

Append a comment to an artifact

```text
backlogit comment add <item-id> [flags]
```

### Examples

```text
  backlogit comment add 001-F --actor ship --comment "built U4"
```

### Options

```text
      --actor string        actor recording the comment
      --comment string      comment text
      --commit-sha string   associated commit SHA (optional)
  -h, --help                help for add
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
      --no-update-check    skip the remote latest-release check
```

### SEE ALSO

* [backlogit comment](backlogit_comment.md)	 - Append comments to an artifact's history

