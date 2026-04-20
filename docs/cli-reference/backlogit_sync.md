---
title: "backlogit sync"
description: "Rehydrate the SQLite index from Markdown source files"
---

## backlogit sync

Rehydrate the SQLite index from Markdown source files

### Synopsis

Rebuild the backlogit SQLite cache from the Markdown and JSONL files in
the workspace.

Use this after making manual changes to .backlogit files or when you want to
force the disposable cache to match the file-backed source of truth.

```text
backlogit sync [flags]
```

### Examples

```text
  backlogit sync
  backlogit --cwd D:\Source\MyProject sync
```

### Options

```text
  -h, --help   help for sync
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit](backlogit.md)	 - Backlogit — AI-native agile workspace

