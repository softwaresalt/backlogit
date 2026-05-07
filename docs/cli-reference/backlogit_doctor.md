---
title: "backlogit doctor"
description: "Check workspace integrity"
---

## backlogit doctor

Check workspace integrity

### Synopsis

Scan the .backlogit workspace for structural issues such as
orphaned artifacts (child types with no parent) and duplicate IDs
across queue and archive directories.

```text
backlogit doctor [flags]
```

### Examples

```text
  backlogit doctor
  backlogit doctor --check-orphans=false
  backlogit doctor --format json
```

### Options

```text
      --check-duplicates   check for duplicate IDs across directories (default true)
      --check-orphans      check for orphaned child artifacts (default true)
      --format string      output format: text or json (default "text")
  -h, --help               help for doctor
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit](backlogit.md)	 - Backlogit — AI-native agile workspace

