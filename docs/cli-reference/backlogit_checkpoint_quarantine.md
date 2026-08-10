---
chunk_strategy: h1-h2-h3
description: Quarantine a malformed checkpoint
doc_type: reference
ingested_at: "2026-08-10T08:26:43Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_checkpoint_quarantine.md
title: backlogit checkpoint quarantine
---

## backlogit checkpoint quarantine

Quarantine a malformed checkpoint

### Synopsis

Quarantine a malformed session state checkpoint.

Quarantine operates ONLY on a malformed (unparseable or schema-invalid)
checkpoint. If the target file parses and validates cleanly, this command
refuses and directs you to "checkpoint abandon" instead — abandon and
quarantine are disjoint verbs by design. The checkpoint's bytes are moved
verbatim (byte-identical) into the workspace archive/checkpoints directory.

```text
backlogit checkpoint quarantine <filename> [flags]
```

### Examples

```text
  backlogit checkpoint quarantine checkpoint-20260423-100000.json --reason "corrupt JSON"
```

### Options

```text
  -h, --help              help for quarantine
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

