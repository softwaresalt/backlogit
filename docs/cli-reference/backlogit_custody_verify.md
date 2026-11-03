---
chunk_strategy: h1-h2-h3
description: Verify recorded custody receipts against Git state
doc_type: reference
ingested_at: "2026-11-03T00:48:33Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_custody_verify.md
title: backlogit custody verify
---

## backlogit custody verify

Verify recorded custody receipts against Git state

### Synopsis

Verify the recorded custody receipts of a shipment against the current Git state.
This command is read-only.

This mirrors the backlogit_custody_verify MCP tool.

```text
backlogit custody verify <shipment-id> [flags]
```

### Examples

```text
  backlogit custody verify 001-S --branch feat/my-branch
```

### Options

```text
      --branch string   work branch to evaluate against (defaults to the checked-out branch)
  -h, --help            help for verify
      --json            emit JSON output (default)
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

