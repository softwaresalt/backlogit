---
title: "backlogit get"
description: "Retrieve an artifact by ID"
---

## backlogit get

Retrieve an artifact by ID

### Synopsis

Retrieve a single artifact and print either a human-readable detail view,
JSON output, or a specific named body section.

```text
backlogit get <id> [flags]
```

### Examples

```text
  backlogit get 001-F
  backlogit get 001-F --format json
  backlogit get 001-F --section description
```

### Options

```text
      --format string    output format: table, json (default "table")
  -h, --help             help for get
      --json             output frontmatter as JSON
      --section string   extract a named section from the body
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit](backlogit.md)	 - Backlogit — AI-native agile workspace

