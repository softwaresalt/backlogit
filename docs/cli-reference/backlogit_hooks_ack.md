---
chunk_strategy: h1-h2-h3
description: Acknowledge processed hook events up to and including --seq
doc_type: reference
ingested_at: "2026-07-04T05:20:25Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_hooks_ack.md
title: backlogit hooks ack
---

## backlogit hooks ack

Acknowledge processed hook events up to and including --seq

```text
backlogit hooks ack [flags]
```

### Examples

```text
  backlogit hooks ack --consumer-id ship --seq 12
```

### Options

```text
      --consumer-id string   agent consumer ID
  -h, --help                 help for ack
      --seq int              highest sequence number processed
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit hooks](backlogit_hooks.md)	 - Poll and acknowledge agent hook events

