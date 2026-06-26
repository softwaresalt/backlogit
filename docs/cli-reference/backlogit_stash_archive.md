---
chunk_strategy: h1-h2-h3
description: Archive an active stash entry
doc_type: reference
ingested_at: "2026-06-26T02:27:58Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_stash_archive.md
title: backlogit stash archive
---

## backlogit stash archive

Archive an active stash entry

```text
backlogit stash archive <stash-id> [flags]
```

### Examples

```text
  backlogit stash archive ABCD1234
  backlogit stash remove ABCD1234
```

### Options

```text
  -h, --help   help for archive
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit stash](backlogit_stash.md)	 - Manage the deferred work stash

