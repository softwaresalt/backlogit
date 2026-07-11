---
chunk_strategy: h1-h2-h3
description: Print the unified metadata catalog
doc_type: reference
ingested_at: "2026-06-26T02:27:58Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_metadata_catalog.md
title: backlogit metadata catalog
---

## backlogit metadata catalog

Print the unified metadata catalog

```text
backlogit metadata catalog [flags]
```

### Examples

```text
  backlogit metadata catalog
  backlogit metadata catalog --json
```

### Options

```text
  -h, --help   help for catalog
      --json   output as JSON (default true)
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

