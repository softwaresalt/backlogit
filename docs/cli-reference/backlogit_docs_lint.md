---
chunk_strategy: h1-h2-h3
description: Validate in-scope documentation frontmatter (retains non-zero exit on violations)
doc_type: reference
ingested_at: "2026-06-26T02:27:58Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_docs_lint.md
title: backlogit docs lint
---

## backlogit docs lint

Validate in-scope documentation frontmatter (retains non-zero exit on violations)

### Synopsis

Validate in-scope documentation frontmatter against the docline base schema.

Prints the findings and exits non-zero when any violation exists (CI-friendly).
A per-file frontmatter decode failure (malformed YAML) is reported as a
finding with rule decode_error rather than aborting the scan: the rest of the
corpus is still linted, and the process still exits non-zero because a
corpus containing a decode_error is not a clean tree — the non-zero exit is
retained for this case exactly as for any other violation.

```text
backlogit docs lint [flags]
```

### Examples

```text
  backlogit docs lint
  backlogit docs lint --profile ingestion --format json
  backlogit docs lint --path docs/decisions
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
      --no-update-check    skip the remote latest-release check
```

### SEE ALSO

* [backlogit docs](backlogit_docs.md)	 - Lint and migrate documentation frontmatter (docline base schema)

