---
chunk_strategy: h1-h2-h3
description: Print version, commit, build date, and Go runtime information
doc_type: reference
ingested_at: "2026-06-26T02:27:58Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_version.md
title: backlogit version
---

## backlogit version

Print version, commit, build date, and Go runtime information

```text
backlogit version [flags]
```

### Examples

```text
  backlogit version
  backlogit version --format json
```

### Options

```text
      --format string   output format: json (default: human)
  -h, --help            help for version
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit](backlogit.md)	 - Backlogit — AI-native agile workspace

