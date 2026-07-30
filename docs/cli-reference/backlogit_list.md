---
chunk_strategy: h1-h2-h3
description: List artifacts in the workspace
doc_type: reference
ingested_at: "2026-06-26T02:27:58Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_list.md
title: backlogit list
---

## backlogit list

List artifacts in the workspace

### Synopsis

List artifacts from the backlogit index with optional filters.

Use this command for quick operator views, grouped summaries, or JSON output
that can be piped into other tooling.

Complexity is task-only planning metadata:
size = implementation volume; complexity = implementation difficulty and uncertainty;
priority = urgency. Default queue ordering does not change
when filtering by complexity.

```text
backlogit list [flags]
```

### Examples

```text
  backlogit list
  backlogit list --status active --type task
  backlogit list --group-by status
  backlogit list --json
```

### Options

```text
      --assigned-to string   filter by assignee
      --complexity string    filter by implementation complexity (trivial, low, medium, high)
      --format string        output format: table, json, tile (default "table")
      --group-by string      group output by field (status, type, priority)
  -h, --help                 help for list
      --json                 output as JSON array
      --owner string         filter by owner
      --priority string      filter by priority
      --sprint string        filter by sprint ID
      --status string        filter by status
      --type string          filter by artifact type
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

