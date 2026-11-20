---
chunk_strategy: h1-h2-h3
description: Initialize a new backlogit workspace
doc_type: reference
ingested_at: "2026-06-26T02:27:58Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_init.md
title: backlogit init
---

## backlogit init

Initialize a new backlogit workspace

### Synopsis

Initialize a backlogit workspace in the target directory.

This creates the .backlog storage root, logs directory, canonical stash JSONL
file, default YAML configuration files, default artifact templates, installs the
backlogit binary into the chosen target tool directory, and configures .mcp.json.

```text
backlogit init [path] [flags]
```

### Examples

```text
  backlogit init
  backlogit init --tools-dir .bin
  backlogit init D:\Source\MyProject
```

### Options

```text
  -h, --help               help for init
      --tools-dir string   workspace-relative target tool directory for backlogit binary and .mcp.json (default .bin or bin)
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

