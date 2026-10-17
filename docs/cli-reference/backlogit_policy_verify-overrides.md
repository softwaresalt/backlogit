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
An edited, reordered, extended, truncated, emptied or deleted ledger fails,
and so does a missing anchor unless --genesis declares that no override has
ever been recorded (the ledger must then be empty).
Both paths must be absolute, outside the repository and in different
directories. Verification takes the ledger's lock file, so it needs
write access to the ledger directory.

```text
backlogit policy verify-overrides [flags]
```

### Examples

```text
  backlogit policy verify-overrides --ledger /secure/ledger/overrides.jsonl --anchor /secure/anchor/anchor.json
  backlogit policy verify-overrides --genesis --ledger /secure/ledger/overrides.jsonl --anchor /secure/anchor/anchor.json
```

### Options

```text
      --anchor string   absolute path of the ledger anchor (outside the repository, not in the ledger's directory)
      --format string   output format: text, json (default: text on TTY, json otherwise)
      --genesis         accept a missing anchor as the genesis state (the ledger must then be empty)
  -h, --help            help for verify-overrides
      --ledger string   absolute path of the override ledger (outside the repository)
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

