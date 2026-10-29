---
chunk_strategy: h1-h2-h3
description: Add a trust anchor (fingerprint must be externally pinned)
doc_type: reference
ingested_at: "2026-10-29T03:03:19Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_trust-anchor_add.md
title: backlogit trust-anchor add
---

## backlogit trust-anchor add

Add a trust anchor (fingerprint must be externally pinned)

```text
backlogit trust-anchor add [flags]
```

### Options

```text
      --algo string             signature algorithm (ed25519 or ecdsa-p256)
      --fingerprint string      externally pinned fingerprint of the key
  -h, --help                    help for add
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

