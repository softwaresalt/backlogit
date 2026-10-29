---
chunk_strategy: h1-h2-h3
description: Inspect and manage verification-key trust anchors
doc_type: reference
ingested_at: "2026-10-29T03:03:19Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_trust-anchor.md
title: backlogit trust-anchor
---

## backlogit trust-anchor

Inspect and manage verification-key trust anchors

### Options

```text
  -h, --help   help for trust-anchor
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
* [backlogit trust-anchor add](backlogit_trust-anchor_add.md)	 - Add a trust anchor (fingerprint must be externally pinned)
* [backlogit trust-anchor list](backlogit_trust-anchor_list.md)	 - List configured trust anchors with their resolved state
* [backlogit trust-anchor revoke](backlogit_trust-anchor_revoke.md)	 - Revoke a trust anchor (requires operator approval and an external revocation authority; fails closed otherwise)
* [backlogit trust-anchor rotate](backlogit_trust-anchor_rotate.md)	 - Rotate a trust anchor to a new key (requires operator approval)
* [backlogit trust-anchor show](backlogit_trust-anchor_show.md)	 - Show one trust anchor with its resolved state
* [backlogit trust-anchor verify-key](backlogit_trust-anchor_verify-key.md)	 - Exit 0 if the key resolves as trusted, non-zero otherwise (read-only)

