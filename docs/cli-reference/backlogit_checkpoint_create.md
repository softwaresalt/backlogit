---
chunk_strategy: h1-h2-h3
description: Create a session state checkpoint (open context, closed schema)
doc_type: reference
ingested_at: "2026-07-04T00:42:54Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_checkpoint_create.md
title: backlogit checkpoint create
---

## backlogit checkpoint create

Create a session state checkpoint (open context, closed schema)

### Synopsis

Create a session state checkpoint from a JSON state dump.

The state dump is written to the workspace checkpoints directory. When the dump
declares schema_version=1, it is validated as a V1 checkpoint and missing
created_at, updated_at, and status fields are auto-populated.

The top level and the nested progress object are a CLOSED schema namespace:
any key outside the modeled set (schema_version, agent, session_id, phase,
status, created_at, updated_at, context, progress, resume_hint, disposition,
disposition_reason, disposition_operator, and disposition_at at the top
level; tasks_completed, tasks_remaining, files_modified, and decisions
inside progress) is an unknown field and the create is rejected, naming
every offending key path.

The context object is the OPEN counterpart: shipment_id, feature_id,
task_ids, and branch are modeled, but any other key you supply there
survives the create round-trip unchanged. The written path and context_keys
(the exact list of context key names persisted to disk) are returned as
JSON.

```text
backlogit checkpoint create [flags]
```

### Examples

```text
  backlogit checkpoint create --state-dump '{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","context":{"shipment_id":"129-S","pr_number":372}}'
```

### Options

```text
  -h, --help                help for create
      --state-dump string   JSON checkpoint state dump
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

