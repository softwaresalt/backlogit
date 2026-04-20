---
title: "backlogit status"
description: "Show workspace artifact summary"
ms.date: 2026-04-19
---

## backlogit status

Show workspace artifact summary

### Synopsis

Show a workspace summary grouped by artifact type and status.

This is a quick health check for the current indexed backlog state.

```text
backlogit status [flags]
```

### Examples

```text
  backlogit status
  backlogit --cwd D:\Source\MyProject status
```

### Options

```text
  -h, --help   help for status
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit](backlogit.md)	 - Backlogit — AI-native agile workspace

