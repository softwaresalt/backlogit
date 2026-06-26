---
chunk_strategy: h1-h2-h3
description: Lint and migrate documentation frontmatter (docline base schema)
doc_type: reference
ingested_at: "2026-06-26T02:27:58Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_docs.md
title: backlogit docs
---

## backlogit docs

Lint and migrate documentation frontmatter (docline base schema)

### Synopsis

Operate on the repository documentation surface using the docline base
frontmatter contract: lint in-scope docs, plan and apply idempotent migrations,
inspect the active scope, and classify a path's doc_type.

### Examples

```text
  backlogit docs lint
  backlogit docs lint --profile ingestion --format json
  backlogit docs migrate
  backlogit docs migrate --apply --yes --path docs/decisions
  backlogit docs scope
  backlogit docs classify docs/decisions/x.md
```

### Options

```text
  -h, --help   help for docs
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit](backlogit.md)	 - Backlogit — AI-native agile workspace
* [backlogit docs classify](backlogit_docs_classify.md)	 - Print the derived doc_type for a repo-relative path
* [backlogit docs lint](backlogit_docs_lint.md)	 - Validate in-scope documentation frontmatter
* [backlogit docs migrate](backlogit_docs_migrate.md)	 - Plan (default) or apply an idempotent frontmatter migration
* [backlogit docs scope](backlogit_docs_scope.md)	 - Print the active docline scope, profiles, and taxonomy

