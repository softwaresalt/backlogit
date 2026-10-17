---
chunk_strategy: h1-h2-h3
description: Enforce workflow-policy gates (harness, red/green, review, CI, merge-ready)
doc_type: reference
ingested_at: "2026-10-17T21:55:52Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_policy.md
title: backlogit policy
---

## backlogit policy

Enforce workflow-policy gates (harness, red/green, review, CI, merge-ready)

### Options

```text
  -h, --help   help for policy
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
* [backlogit policy check](backlogit_policy_check.md)	 - Evaluate one task's commit-bound evidence against the workflow-policy graph
* [backlogit policy verify-overrides](backlogit_policy_verify-overrides.md)	 - Verify the tamper-evident override ledger against its external anchor

