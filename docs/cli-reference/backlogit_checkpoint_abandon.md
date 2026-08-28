---
chunk_strategy: h1-h2-h3
description: Administratively abandon a valid checkpoint
doc_type: reference
ingested_at: "2026-08-10T08:26:43Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_checkpoint_abandon.md
title: backlogit checkpoint abandon
---

## backlogit checkpoint abandon

Administratively abandon a valid checkpoint

### Synopsis

Administratively abandon a session state checkpoint.

Abandon operates on a parseable, schema-valid, AND conforming checkpoint —
one carrying no unmodeled or duplicate top-level keys. If the target does
not parse, fails schema validation, or carries unmodeled/duplicate keys,
this command refuses and directs you to "checkpoint quarantine" instead,
which is the sole accepting verb for that class.

```text
backlogit checkpoint abandon <filename> [flags]
```

### Examples

```text
  backlogit checkpoint abandon checkpoint-20260423-100000.json --reason "superseded by newer session"
```

### Options

```text
  -h, --help              help for abandon
      --operator string   operator identity (defaults to BACKLOGIT_OPERATOR env var, then the OS user)
      --reason string     reason for the disposition (required)
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
      --no-update-check    skip the remote latest-release check
```

### SEE ALSO

* [backlogit checkpoint](backlogit_checkpoint.md)	 - Manage session state checkpoints

