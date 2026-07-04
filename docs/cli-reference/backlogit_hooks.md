---
chunk_strategy: h1-h2-h3
description: Poll and acknowledge agent hook events
doc_type: reference
ingested_at: "2026-07-04T05:20:25Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_hooks.md
title: backlogit hooks
---

## backlogit hooks

Poll and acknowledge agent hook events

### Synopsis

Poll for unacknowledged hook events and acknowledge processed events.

These commands are the CLI fallback for the MCP poll_hook_events/ack_hook_events
tools. They reuse the same durable queue at .backlogit/hooks_queue.jsonl and the
per-consumer checkpoints, so consumer progress is shared across surfaces.

### Options

```text
  -h, --help   help for hooks
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit](backlogit.md)	 - Backlogit — AI-native agile workspace
* [backlogit hooks ack](backlogit_hooks_ack.md)	 - Acknowledge processed hook events up to and including --seq
* [backlogit hooks poll](backlogit_hooks_poll.md)	 - Poll for unacknowledged hook events since the consumer checkpoint

