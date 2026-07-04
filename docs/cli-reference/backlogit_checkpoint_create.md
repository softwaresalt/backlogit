---
chunk_strategy: h1-h2-h3
description: Create a session state checkpoint
doc_type: reference
ingested_at: "2026-07-04T00:42:54Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_checkpoint_create.md
title: backlogit checkpoint create
---

## backlogit checkpoint create

Create a session state checkpoint

### Synopsis

Create a session state checkpoint from a JSON state dump.

The state dump is written to the workspace checkpoints directory. When the dump
declares schema_version=1, it is validated as a V1 checkpoint and missing
created_at, updated_at, and status fields are auto-populated. The written path
is returned as JSON.

```text
backlogit checkpoint create [flags]
```

### Examples

```text
  backlogit checkpoint create --state-dump '{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build"}'
```

### Options

```text
  -h, --help                help for create
      --state-dump string   JSON checkpoint state dump
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit checkpoint](backlogit_checkpoint.md)	 - Manage session state checkpoints

