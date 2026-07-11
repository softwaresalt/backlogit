---
chunk_strategy: h1-h2-h3
description: Print version, latest release, commit, build date, and Go runtime information
doc_type: reference
ingested_at: "2026-06-26T02:27:58Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_version.md
title: backlogit version
---

## backlogit version

Print version, latest release, commit, build date, and Go runtime information

### Synopsis

Print build metadata and latest-release status.

By default, backlogit performs a bounded latest-release check against GitHub.
Use --no-update-check or set BACKLOGIT_NO_UPDATE_CHECK to one of
1, true, t, yes, y, or on to skip the remote call for CI and scripts.

```text
backlogit version [flags]
```

### Examples

```text
  backlogit version
  backlogit version --no-update-check
  backlogit version --format json
```

### Options

```text
      --format string     output format: json (default: human)
  -h, --help              help for version
      --no-update-check   skip the remote latest-release check
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit](backlogit.md)	 - Backlogit — AI-native agile workspace

