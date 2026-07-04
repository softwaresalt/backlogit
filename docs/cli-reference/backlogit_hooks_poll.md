---
chunk_strategy: h1-h2-h3
description: Poll for unacknowledged hook events since the consumer checkpoint
doc_type: reference
ingested_at: "2026-07-04T05:20:25Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_hooks_poll.md
title: backlogit hooks poll
---

## backlogit hooks poll

Poll for unacknowledged hook events since the consumer checkpoint

```text
backlogit hooks poll [flags]
```

### Examples

```text
  backlogit hooks poll --consumer-id ship
```

### Options

```text
      --consumer-id string   agent consumer ID (e.g. ship, stage)
  -h, --help                 help for poll
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit hooks](backlogit_hooks.md)	 - Poll and acknowledge agent hook events

