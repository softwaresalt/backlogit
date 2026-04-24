---
title: "backlogit checkpoint"
description: "Manage session state checkpoints"
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
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit](backlogit.md)	 - Backlogit — AI-native agile workspace
* [backlogit checkpoint cleanup](backlogit_checkpoint_cleanup.md)	 - Archive resolved and stale checkpoints
* [backlogit checkpoint get](backlogit_checkpoint_get.md)	 - Get and validate a specific checkpoint
* [backlogit checkpoint list](backlogit_checkpoint_list.md)	 - List session state checkpoints
* [backlogit checkpoint resolve](backlogit_checkpoint_resolve.md)	 - Mark a checkpoint as resolved

