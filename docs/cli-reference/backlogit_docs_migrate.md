---
chunk_strategy: h1-h2-h3
description: Plan (default) or apply an idempotent frontmatter migration
doc_type: reference
ingested_at: "2026-06-26T02:27:58Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_docs_migrate.md
title: backlogit docs migrate
---

## backlogit docs migrate

Plan (default) or apply an idempotent frontmatter migration

```text
backlogit docs migrate [flags]
```

### Options

```text
      --apply           write changes; without it migrate only plans (dry-run). Requires --yes and an explicit --path
      --format string   output format: text, json (default: text on TTY, json otherwise)
  -h, --help            help for migrate
      --path string     limit to a repo-relative sub-path (required for --apply)
      --yes             confirm a write (required with --apply)
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
      --no-update-check    skip the remote latest-release check
```

### SEE ALSO

* [backlogit docs](backlogit_docs.md)	 - Lint and migrate documentation frontmatter (docline base schema)

