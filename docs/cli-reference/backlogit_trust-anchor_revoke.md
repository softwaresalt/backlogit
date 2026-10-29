---
chunk_strategy: h1-h2-h3
description: Revoke a trust anchor (requires operator approval and an external revocation authority; fails closed otherwise)
doc_type: reference
ingested_at: "2026-10-29T03:03:19Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_trust-anchor_revoke.md
title: backlogit trust-anchor revoke
---

## backlogit trust-anchor revoke

Revoke a trust anchor (requires operator approval and an external revocation authority; fails closed otherwise)

```text
backlogit trust-anchor revoke <id> [flags]
```

### Options

```text
      --approval string   operator approval token file (signed JSON evidence); required for rotate and revoke
  -h, --help              help for revoke
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

