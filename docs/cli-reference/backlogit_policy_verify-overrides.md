---
chunk_strategy: h1-h2-h3
description: Verify the tamper-evident override ledger against its external anchor
doc_type: reference
ingested_at: "2026-10-17T21:55:52Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_policy_verify-overrides.md
title: backlogit policy verify-overrides
---

## backlogit policy verify-overrides

Verify the tamper-evident override ledger against its external anchor

### Synopsis

Verify the append-only, hash-chained override ledger against its external
anchor and list its entries (authenticated principal, justification, gate).
An edited, reordered, extended, truncated, emptied or deleted ledger fails.
Both paths must be absolute and outside the workspace.

```text
backlogit policy verify-overrides [flags]
```

### Examples

```text
  backlogit policy verify-overrides --ledger /secure/overrides.jsonl --anchor /secure/anchor.json
```

### Options

```text
      --anchor string   absolute path of the ledger anchor (outside the workspace)
      --format string   output format: text, json (default: text on TTY, json otherwise)
  -h, --help            help for verify-overrides
      --ledger string   absolute path of the override ledger (outside the workspace)
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
      --no-update-check    skip the remote latest-release check
```

### SEE ALSO

* [backlogit policy](backlogit_policy.md)	 - Enforce workflow-policy gates (harness, red/green, review, CI, merge-ready)

