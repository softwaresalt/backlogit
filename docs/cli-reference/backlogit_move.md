---
chunk_strategy: h1-h2-h3
description: Change artifact status
doc_type: reference
ingested_at: "2026-06-26T02:27:58Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_move.md
title: backlogit move
---

## backlogit move

Change artifact status

### Synopsis

Change an artifact status and relocate its file according to registry.yaml
routing rules.

For task/subtask completions the pre-task-completion gate broker may run
autoharness gate check. On a gate refusal this command exits 6 (blocked),
7 (configuration/setup error), or 8 (retryable: lock contention or timeout).

```text
backlogit move <id> [flags]
```

### Examples

```text
  backlogit move 001.001-T --status review
  backlogit move 001-F --status done
  backlogit move 001.001-T --status done --gate-base origin/release
```

### Options

```text
      --force-gates           operator-only: force completion past the gate (requires --force-reason)
      --force-reason string   justification recorded in the forced-gate audit event
      --gate-base string      operator-only base ref override for the completion gate (audited)
  -h, --help                  help for move
      --json                  emit the machine-readable gate outcome contract
      --status string         new status (required)
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
      --no-update-check    skip the remote latest-release check
```

### SEE ALSO

* [backlogit](backlogit.md)	 - Backlogit — AI-native agile workspace

