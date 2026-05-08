---
title: "backlogit delete"
description: "Delete an artifact"
---

## backlogit delete

Delete an artifact

### Synopsis

Delete an artifact from the workspace and remove it from the index.

This is a destructive operation and requires --force.

```text
backlogit delete <id> [flags]
```

### Examples

```text
  backlogit delete 001.001-T --force
```

### Options

```text
      --force   skip confirmation and delete immediately
  -h, --help    help for delete
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit](backlogit.md)	 - Backlogit — AI-native agile workspace

