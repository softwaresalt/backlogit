---
chunk_strategy: h1-h2-h3
description: Classify custody candidates without writing state
doc_type: reference
ingested_at: "2026-11-03T00:48:33Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_custody_preview.md
title: backlogit custody preview
---

## backlogit custody preview

Classify custody candidates without writing state

### Synopsis

Classify candidate paths for a shipment against the base commit, the Git index,
and the worktree. The result reports the candidate-set digest and the authority
profile that the candidate set requires. This command is read-only and works
without the SQLite cache.

This mirrors the backlogit_custody_preview MCP tool.

```text
backlogit custody preview <shipment-id> [flags]
```

### Examples

```text
  backlogit custody preview 001-S --candidates '[{"path":"docs/plan.md","change_kind":"modified","staged_state":"unstaged_only","contributor":"stage","source_kind":"stage_artifact"}]'
```

### Options

```text
      --branch string       work branch to evaluate against (defaults to the checked-out branch)
      --candidates string   JSON array of candidate specs: path, change_kind, staged_state, contributor, source_kind
  -h, --help                help for preview
      --json                emit JSON output (default)
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
      --no-update-check    skip the remote latest-release check
```

### SEE ALSO

* [backlogit custody](backlogit_custody.md)	 - Preview, offer, accept, and verify workflow custody handoffs

