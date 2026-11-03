---
chunk_strategy: h1-h2-h3
description: Preview, offer, accept, and verify workflow custody handoffs
doc_type: reference
ingested_at: "2026-11-03T00:48:33Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_custody.md
title: backlogit custody
---

## backlogit custody

Preview, offer, accept, and verify workflow custody handoffs

### Synopsis

Manage workflow custody handoffs for a shipment.

Custody records which candidate changes one role hands to another. preview and
verify are read-only. offer and accept write the handoff; --dry-run validates
without writing. A live offer or accept runs without the SQLite cache only when
the cache cannot be opened, and then reports db: "unavailable".

### Options

```text
  -h, --help   help for custody
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
* [backlogit custody accept](backlogit_custody_accept.md)	 - Accept an offered custody handoff
* [backlogit custody offer](backlogit_custody_offer.md)	 - Offer a custody handoff for a shipment
* [backlogit custody preview](backlogit_custody_preview.md)	 - Classify custody candidates without writing state
* [backlogit custody verify](backlogit_custody_verify.md)	 - Verify recorded custody receipts against Git state

