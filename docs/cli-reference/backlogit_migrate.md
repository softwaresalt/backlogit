---
title: "backlogit migrate"
description: "Migrate backlog data between supported formats and layouts"
---

## backlogit migrate

Migrate backlog data between supported formats and layouts

### Synopsis

Migrate backlog data either from supported source adapters such as backlog-md
or from older internal workspace layouts.

Use --source with --adapter backlog-md for source imports. Use --dry-run and
--validate before writing imported artifacts. Use --rollback only for internal
layout migrations, not source imports.

```text
backlogit migrate [flags]
```

### Examples

```text
  backlogit migrate --source .\.backlog --adapter backlog-md --dry-run
  backlogit migrate --source .\.backlog --adapter backlog-md --validate
  backlogit migrate --source .\.backlog --adapter backlog-md
  backlogit migrate --dry-run
  backlogit migrate --rollback
```

### Options

```text
      --adapter string   migration adapter to use (for example: backlog-md)
      --detect           detect the adapter for the source and print it
      --dry-run          preview changes without moving files
      --format string    report format: text or json (default "text")
  -h, --help             help for migrate
      --rollback         reverse a previous migration
      --source string    path to source workspace or file to import
      --validate         validate the source import without writing artifacts
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit](backlogit.md)	 - Backlogit — AI-native agile workspace

