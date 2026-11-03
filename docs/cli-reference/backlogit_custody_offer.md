---
chunk_strategy: h1-h2-h3
description: Offer a custody handoff for a shipment
doc_type: reference
ingested_at: "2026-11-03T00:48:33Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_custody_offer.md
title: backlogit custody offer
---

## backlogit custody offer

Offer a custody handoff for a shipment

### Synopsis

Record a custody handoff offer for a shipment. --spec is a JSON custody receipt
with its entries, candidate_set_digest, authority_profile, and accepting role.
--dry-run validates the offer without writing any state.

This mirrors the backlogit_custody_offer MCP tool.

```text
backlogit custody offer <shipment-id> [flags]
```

### Examples

```text
  backlogit custody offer 001-S --spec '<receipt-json>' --dry-run
```

### Options

```text
      --dry-run       validate the offer without writing
  -h, --help          help for offer
      --json          emit JSON output (default)
      --spec string   JSON custody receipt with entries, candidate_set_digest, authority_profile, and accepting_role
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
      --no-update-check    skip the remote latest-release check
```

### SEE ALSO

* [backlogit custody](backlogit_custody.md)	 - Preview, offer, accept, and verify workflow custody handoffs

