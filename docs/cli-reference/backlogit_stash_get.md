---
chunk_strategy: h1-h2-h3
description: Get a stash entry by ID
doc_type: reference
ingested_at: "2026-06-26T02:27:58Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_stash_get.md
title: backlogit stash get
---

## backlogit stash get

Get a stash entry by ID

```text
backlogit stash get <stash-id> [flags]
```

### Examples

```text
  backlogit stash get ABCD1234
```

### Options

```text
  -h, --help   help for get
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
      --no-update-check    skip the remote latest-release check
```

### SEE ALSO

* [backlogit stash](backlogit_stash.md)	 - Manage the deferred work stash

