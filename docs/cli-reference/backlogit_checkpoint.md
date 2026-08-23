---
chunk_strategy: h1-h2-h3
description: Manage session state checkpoints
doc_type: reference
ingested_at: "2026-06-26T02:27:58Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_checkpoint.md
title: backlogit checkpoint
---

## backlogit checkpoint

Manage session state checkpoints

### Synopsis

Manage agent session state checkpoints for disaster recovery.

Checkpoints are written by agent sessions to enable recovery from unexpected
termination. Use these commands to list, inspect, resolve, and clean up
checkpoint files.

### Options

```text
  -h, --help   help for checkpoint
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
* [backlogit checkpoint abandon](backlogit_checkpoint_abandon.md)	 - Administratively abandon a valid checkpoint
* [backlogit checkpoint cleanup](backlogit_checkpoint_cleanup.md)	 - Archive resolved and stale checkpoints
* [backlogit checkpoint create](backlogit_checkpoint_create.md)	 - Create a session state checkpoint (open context, closed schema)
* [backlogit checkpoint get](backlogit_checkpoint_get.md)	 - Get and validate a specific checkpoint
* [backlogit checkpoint list](backlogit_checkpoint_list.md)	 - List session state checkpoints
* [backlogit checkpoint quarantine](backlogit_checkpoint_quarantine.md)	 - Quarantine a malformed checkpoint
* [backlogit checkpoint resolve](backlogit_checkpoint_resolve.md)	 - Mark a checkpoint as resolved

