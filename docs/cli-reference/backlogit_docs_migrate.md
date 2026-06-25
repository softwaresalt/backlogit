---
title: "backlogit docs migrate"
description: "Plan (default) or apply an idempotent frontmatter migration"
---

## backlogit docs migrate

Plan (default) or apply an idempotent frontmatter migration

```text
backlogit docs migrate [flags]
```

### Options

```text
      --apply           write changes; requires --yes and an explicit --path
      --dry-run         compute the plan without writing (default) (default true)
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
```

### SEE ALSO

* [backlogit docs](backlogit_docs.md)	 - Lint and migrate documentation frontmatter (docline base schema)

