---
chunk_strategy: h1-h2-h3
description: Accept an offered custody handoff
doc_type: reference
ingested_at: "2026-11-03T00:48:33Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_custody_accept.md
title: backlogit custody accept
---

## backlogit custody accept

Accept an offered custody handoff

### Synopsis

Accept a previously offered custody receipt. operator_verified receipts require
--auth-token, a signed operator approval, and a configured nonce provider; the
operator approval must be supplied in the signed token rather than inferred from
a confirmation. --dry-run verifies the acceptance without consuming a nonce or
writing state.

This mirrors the backlogit_custody_accept MCP tool.

```text
backlogit custody accept <shipment-id> [flags]
```

### Examples

```text
  backlogit custody accept 001-S --receipt-id rcpt-stage-ship-1 --by ship --dry-run
```

### Options

```text
      --auth-token string        encoded operator approval token (required for operator_verified receipts)
      --by string                accepting role or principal
      --dry-run                  verify the acceptance without consuming a nonce or writing
  -h, --help                     help for accept
      --idempotency-key string   key that deduplicates retried accepts
      --json                     emit JSON output (default)
      --receipt-id string        offered receipt ID to accept
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

