---
chunk_strategy: h1-h2-h3
description: Show per-branch telemetry metrics with type classification and enrichment
doc_type: reference
ingested_at: "2026-06-26T02:27:58Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_telemetry_branch.md
title: backlogit telemetry branch
---

## backlogit telemetry branch

Show per-branch telemetry metrics with type classification and enrichment

### Synopsis

Show per-branch telemetry metrics with type classification and enrichment.

Aggregates all sessions by branch name and enriches each branch profile with:
  - Branch type classification (feature, chore, stage, ship, post-merge, main, other)
  - Git PR number (from merge commit history)
  - Backlogit artifact IDs (shipment/feature extracted from branch naming conventions)
  - Lifespan (first to last session)

Use --type to filter by branch type (e.g. --type feature).
Use --format json for machine-readable output.

```text
backlogit telemetry branch [flags]
```

### Options

```text
      --format string   Output format: table, json, markdown (default "table")
  -h, --help            help for branch
      --limit int       Restrict the number of branches returned (0 = no limit)
      --type string     Filter by branch type: feature, chore, stage, ship, post-merge, main, other
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit telemetry](backlogit_telemetry.md)	 - Inspect Copilot CLI token usage and tool telemetry

