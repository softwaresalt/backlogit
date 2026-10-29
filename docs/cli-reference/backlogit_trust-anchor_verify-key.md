---
chunk_strategy: h1-h2-h3
description: Exit 0 if the key resolves as trusted, non-zero otherwise (read-only)
doc_type: reference
ingested_at: "2026-10-29T03:03:19Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_trust-anchor_verify-key.md
title: backlogit trust-anchor verify-key
---

## backlogit trust-anchor verify-key

Exit 0 if the key resolves as trusted, non-zero otherwise (read-only)

```text
backlogit trust-anchor verify-key <id> [flags]
```

### Options

```text
  -h, --help   help for verify-key
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

