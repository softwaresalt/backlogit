---
chunk_strategy: h1-h2-h3
description: Write an agent-readable command map into the workspace
doc_type: reference
ingested_at: "2026-06-26T02:27:58Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_metadata_export-command-map.md
title: backlogit metadata export-command-map
---

## backlogit metadata export-command-map

Write an agent-readable command map into the workspace

### Synopsis

Export a cacheable command map file into a workspace-relative path such as
.github\instructions\backlogit-command-map.instructions.md so agents can reason
over backlogit commands and metadata without re-discovering them each run.

```text
backlogit metadata export-command-map <workspace-relative-path> [flags]
```

### Examples

```text
  backlogit metadata export-command-map .github\instructions\backlogit-command-map.md
  backlogit metadata export-command-map .github\instructions\backlogit-command-map.json --format json
```

### Options

```text
      --format string   output format: markdown or json (default "markdown")
  -h, --help            help for export-command-map
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
      --no-update-check    skip the remote latest-release check
```

### SEE ALSO

* [backlogit metadata](backlogit_metadata.md)	 - Discover backlogit metadata for agents and tooling

