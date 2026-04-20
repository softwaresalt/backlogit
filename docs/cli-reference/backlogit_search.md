---
title: "backlogit search"
description: "Full-text search across artifacts"
---

## backlogit search

Full-text search across artifacts

### Synopsis

Search the full-text index for matching artifacts.

Use this when you want quick keyword lookup without writing SQL.

```text
backlogit search <query> [flags]
```

### Examples

```text
  backlogit search authentication
  backlogit search "token rotation" --limit 10
```

### Options

```text
  -h, --help        help for search
      --limit int   maximum number of results (default 20)
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit](backlogit.md)	 - Backlogit — AI-native agile workspace

