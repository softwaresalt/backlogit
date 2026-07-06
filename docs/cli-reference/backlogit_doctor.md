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
records (rewrites them to their canonical queue restore path). Use
--fix-malformed to clear malformed archived_from records that have no restore
target. Both are destructive, CLI-only migrations: not exposed on the MCP doctor tool.

Target mode (--target {file}) validates ONE .backlogit artifact file against
the header-def schema, with a 5s deadline, and returns a versioned, gate-stable
exit code:

  0  pass
  1  validation fail (required field errors)
  2  timeout (validation exceeded the 5s deadline)
  3  scope or IO error (path outside .backlogit storage root, unreadable/undecodable)
  4  busy (the task's advisory lock is held by a concurrent operation)

Target mode confines the path to the .backlogit storage root and never reads
outside it. With --format json it emits a versioned target-mode schema
(mode:"target"). autoharness's subprocess timeout_seconds: 5 is the
authoritative outer bound. On repeated gate failure, transition the task with
the existing 'backlogit move {id} --status blocked' (and '--status queued' to
resume) — no new command; retry policy is owned by the caller.

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
  backlogit doctor --target .backlogit/queue/001.001-T.md
  backlogit doctor --target .backlogit/queue/001.001-T.md --format json
```

### Options

```text
      --check-archived-from   check archive records for self-referential/malformed archived_from fields (default true)
      --check-duplicates      check for duplicate IDs across directories (default true)
      --check-gate-evidence   advisory: warn when a terminal task/subtask lacks pre-task-completion gate evidence (exit code unaffected)
      --check-orphans         check for orphaned child artifacts (default true)
      --fix-archived-from     repair legacy self-referential archived_from records (destructive, CLI-only)
      --fix-malformed         clear malformed archived_from records with no restore target (destructive, CLI-only; requires --check-archived-from)
      --fix-orphans           archive orphaned artifacts instead of just reporting them
      --format string         output format: text or json (default "text")
  -h, --help                  help for doctor
      --target string         validate a single .backlogit artifact file against header-def (versioned exit-code gate)
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit](backlogit.md)	 - Backlogit — AI-native agile workspace

