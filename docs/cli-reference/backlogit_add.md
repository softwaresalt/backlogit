---
chunk_strategy: h1-h2-h3
description: Create a new artifact
doc_type: reference
ingested_at: "2026-06-26T02:27:58Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_add.md
title: backlogit add
---

## backlogit add

Create a new artifact

### Synopsis

Create a new backlogit artifact in the current workspace.

Artifacts are written as Markdown files under .backlogit\queue or the target
directory selected by registry routing. Typed hierarchical IDs are assigned
automatically when the configured queue layout supports the requested type.

```text
backlogit add [flags]
```

### Examples

```text
  backlogit add --type feature --title "Authentication hardening"
  backlogit add --type task --title "Add token rotation" --parent 001-F
  backlogit add --type subtask --title "Write expiry tests" --parent 001.001-T --section description="Cover refresh and expiry flows"
```

### Options

```text
      --assigned-to string    assignee
      --commit string         commit SHA
      --dependencies string   comma-separated dependency IDs
      --description string    artifact description
  -h, --help                  help for add
      --labels string         comma-separated labels
      --owner string          owner
      --parent string         parent artifact ID (required for level-2+ types such as task, review)
      --priority string       priority (low, medium, high, critical)
      --references string     comma-separated reference paths
      --section stringArray   section content as name=value (repeatable)
      --sprint string         sprint ID
      --status string         initial status (queued, active, …)
      --title string          artifact title
      --type string           artifact type (feature, task, subtask, …)
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit](backlogit.md)	 - Backlogit — AI-native agile workspace

