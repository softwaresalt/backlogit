---
chunk_strategy: h1-h2-h3
description: List configured trust anchors with their resolved state
doc_type: reference
ingested_at: "2026-10-29T03:03:19Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_trust-anchor_list.md
title: backlogit trust-anchor list
---

## backlogit trust-anchor list

List configured trust anchors with their resolved state

```text
backlogit trust-anchor list [flags]
```

### Options

```text
  -h, --help   help for list
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
      --no-update-check    skip the remote latest-release check
```

### SEE ALSO

* [backlogit trust-anchor](backlogit_trust-anchor.md)	 - Inspect and manage verification-key trust anchors

