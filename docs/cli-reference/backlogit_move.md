---
title: "backlogit move"
description: "Change artifact status"
---

## backlogit move

Change artifact status

### Synopsis

Change an artifact status and relocate its file according to registry.yaml
routing rules.

```text
backlogit move <id> [flags]
```

### Examples

```text
  backlogit move 001.001-T --status review
  backlogit move 001-F --status done
```

### Options

```text
  -h, --help            help for move
      --status string   new status (required)
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit](backlogit.md)	 - Backlogit — AI-native agile workspace

