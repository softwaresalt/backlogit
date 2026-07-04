---
chunk_strategy: h1-h2-h3
description: Append comments to an artifact's history
doc_type: reference
ingested_at: "2026-07-04T05:20:25Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_comment.md
title: backlogit comment
---

## backlogit comment

Append comments to an artifact's history

### Synopsis

Append a comment event to an artifact's JSONL log and index.

This is the CLI fallback for the MCP append_comment tool. It reuses the same
shared core path (core.AppendComment) so the persisted and indexed comment event
is identical across surfaces; success output is JSON isomorphic to the MCP tool
result.

### Options

```text
  -h, --help   help for comment
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit](backlogit.md)	 - Backlogit — AI-native agile workspace
* [backlogit comment add](backlogit_comment_add.md)	 - Append a comment to an artifact

