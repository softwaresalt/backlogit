---
chunk_strategy: h1-h2-h3
description: Rotate a trust anchor to a new key (requires operator approval)
doc_type: reference
ingested_at: "2026-10-29T03:03:19Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_trust-anchor_rotate.md
title: backlogit trust-anchor rotate
---

## backlogit trust-anchor rotate

Rotate a trust anchor to a new key (requires operator approval)

```text
backlogit trust-anchor rotate <id> [flags]
```

### Options

```text
      --algo string             signature algorithm (ed25519 or ecdsa-p256)
      --approval string         operator approval token file (signed JSON evidence); required for rotate and revoke
      --fingerprint string      externally pinned fingerprint of the key
  -h, --help                    help for rotate
      --id string               id of the new trust anchor
      --not-after string        optional RFC3339 end of validity
      --not-before string       optional RFC3339 start of validity
      --public-key-ref string   inline PEM or workspace-relative path to the public key
      --role string             role of the new trust anchor
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

