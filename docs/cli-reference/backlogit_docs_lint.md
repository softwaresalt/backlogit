---
chunk_strategy: h1-h2-h3
description: Validate in-scope documentation frontmatter
doc_type: reference
ingested_at: "2026-06-26T02:27:58Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_docs_lint.md
title: backlogit docs lint
---

## backlogit docs lint

Validate in-scope documentation frontmatter

```text
backlogit docs lint [flags]
```

### Options

```text
      --format string    output format: text, json (default: text on TTY, json otherwise)
  -h, --help             help for lint
      --path string      limit to a repo-relative sub-path
      --profile string   validation profile: authoring, ingestion (default "authoring")
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit docs](backlogit_docs.md)	 - Lint and migrate documentation frontmatter (docline base schema)

