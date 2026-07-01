---
chunk_strategy: h1-h2-h3
description: Update artifact fields or sections
doc_type: reference
ingested_at: "2026-06-26T02:27:58Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_update.md
title: backlogit update
---

## backlogit update

Update artifact fields or sections

### Synopsis

Update frontmatter fields or template-backed body sections on an existing
artifact.

Use repeated --section name=value flags to update named sections without
replacing the rest of the document body.

```text
backlogit update <id> [flags]
```

### Examples

```text
  backlogit update 001.001-T --status review
  backlogit update 001.001-T --priority high
  backlogit update 001-F --section goals="Ship passwordless sign-in"
  backlogit update 001-F --harness-status passing
```

### Options

```text
      --assigned-to string      assignee
      --commit string           commit SHA
      --description string      new description
      --harness-status string   harness status (pending, scaffolded, passing, failing)
  -h, --help                    help for update
      --id string               artifact ID (immutable, always rejected)
      --labels string           comma-separated labels
      --owner string            owner
      --priority string         new priority
      --section stringArray     section update as name=value (repeatable)
      --size string             T-shirt size (XS, S, M, L, XL); body-preserving, mutually exclusive with other field flags
      --sprint string           sprint ID
      --status string           new status
      --title string            new title
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit](backlogit.md)	 - Backlogit — AI-native agile workspace

