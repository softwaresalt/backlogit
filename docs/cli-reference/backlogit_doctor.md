---
chunk_strategy: h1-h2-h3
description: Check workspace integrity
doc_type: reference
ingested_at: "2026-06-26T02:27:58Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_doctor.md
title: backlogit doctor
---

## backlogit doctor

Check workspace integrity

### Synopsis

Scan the .backlogit workspace for structural issues such as
orphaned artifacts (child types with no parent) and duplicate IDs
across queue and archive directories.

Use --fix-orphans to archive orphaned artifacts automatically.
Use --fix-archived-from to repair legacy self-referential archived_from
records (rewrites them to their canonical queue restore path). This is a
destructive, CLI-only migration: it is not exposed on the MCP doctor tool.

```text
backlogit doctor [flags]
```

### Examples

```text
  backlogit doctor
  backlogit doctor --check-orphans=false
  backlogit doctor --fix-orphans
  backlogit doctor --fix-archived-from
  backlogit doctor --format json
```

### Options

```text
      --check-archived-from   check archive records for self-referential/malformed archived_from fields (default true)
      --check-duplicates      check for duplicate IDs across directories (default true)
      --check-orphans         check for orphaned child artifacts (default true)
      --fix-archived-from     repair legacy self-referential archived_from records (destructive, CLI-only)
      --fix-orphans           archive orphaned artifacts instead of just reporting them
      --format string         output format: text or json (default "text")
  -h, --help                  help for doctor
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit](backlogit.md)	 - Backlogit — AI-native agile workspace

