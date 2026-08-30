---
chunk_strategy: h1-h2-h3
description: Record a stash provenance correction
doc_type: reference
ingested_at: "2026-08-30T00:21:35Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_stash_correct.md
title: backlogit stash correct
---

## backlogit stash correct

Record a stash provenance correction

### Synopsis

Record that a stash entry's canonical actual delivery artifact differs from the
historically auto-harvested artifact. Preserves the original harvested_artifact_id and
appends an append-only correction record to provenance_corrections.jsonl.

```text
backlogit stash correct [flags]
```

### Examples

```text
  backlogit stash correct --stash-id 11FFF601 --canonical-delivery 150-F \
    --reason "Actual delivery was 150-F/133-S, not auto-harvested 151-F" --actor "ship-agent"
```

### Options

```text
      --actor string                Actor performing the correction (required)
      --canonical-delivery string   Canonical delivery artifact ID (required)
  -h, --help                        help for correct
      --reason string               Reason for the correction (required)
      --stash-id string             Stash entry ID to correct (required)
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
      --no-update-check    skip the remote latest-release check
```

### SEE ALSO

* [backlogit stash](backlogit_stash.md)	 - Manage the deferred work stash

