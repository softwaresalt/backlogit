---
title: "backlogit archive"
description: "Archive a completed artifact"
---

## backlogit archive

Archive a completed artifact

### Synopsis

Archive one completed artifact or bulk-archive all terminal artifacts.

Archived items are moved into .backlogit\archive and tracked in the index.

```text
backlogit archive <id> [flags]
```

### Examples

```text
  backlogit archive 001.001-T
  backlogit archive --all-done
```

### Options

```text
      --all-done   archive all items with terminal status
  -h, --help       help for archive
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit](backlogit.md)	 - Backlogit — AI-native agile workspace

